// Copyright 2019 The darmasuite Authors
// This file is part of the darmasuite library.
//
// The darmasuite library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The darmasuite library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the darmasuite library. If not, see <http://www.gnu.org/licenses/>.

package wavm

import (
	"bytes"
	"encoding/json"
	"github.com/darmaproject/darmasuite/config"
	"github.com/romana/rlog"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/darmaproject/darma-wasm/darma"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/state"
	"github.com/darmaproject/darmasuite/dvm/core/vm"
	errorsmsg "github.com/darmaproject/darmasuite/dvm/core/vm"
	"github.com/darmaproject/darmasuite/dvm/core/vm/interface"
	wasmcontract "github.com/darmaproject/darmasuite/dvm/core/wavm/contract"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/gas"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/storage"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/utils"
	"github.com/darmaproject/darmasuite/dvm/crypto"
	"github.com/darmaproject/darmasuite/dvm/params"
)

var emptyCodeHash = crypto.Keccak256Hash(nil)
var electionAddress = common.BytesToAddress([]byte{9})

type WAVM struct {
	// Context provides auxiliary blockchain related information
	vm.Context
	// StateDB gives access to the underlying state
	StateDB inter.StateDB
	// Depth is the current call stack
	depth int
	// Mutable is the current call mutable state
	// -1:init state,0:unmutable,1:mutable
	mutable int

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// chain rules contains the chain rules for the current epoch
	chainRules params.Rules
	// virtual machine configuration options used to initialise the
	// wavm.
	wavmConfig Config
	// global (to this context) darma virtual machine
	// used throughout the execution of the tx.
	abort int32
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64

	Wavm *Wavm
}

func (wavm *WAVM) GetCallGasTemp() uint64 {
	return wavm.callGasTemp
}

func (wavm *WAVM) SetCallGasTemp(gas uint64) {
	wavm.callGasTemp = gas
}

func (wavm *WAVM) GetChainConfig() *params.ChainConfig {
	return wavm.chainConfig
}

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func runWavm(wavm *WAVM, contract *wasmcontract.WASMContract, input []byte, isCreate bool) ([]byte, error) {
	rlog.Debugf("---runWavm---, input %x", input)
	if contract.CodeAddr != nil {
		precompiles := vm.PrecompiledContractsHubble
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return vm.RunPrecompiledContract(wavm, p, input, contract)
		}
	}
	if len(contract.Code) == 0 {
		return nil, nil
	}
	var code wasmcontract.WasmCode
	decode, vmInput, err := utils.DecodeContractCode(contract.Code)
	if err != nil {
		return nil, err
	}
	code = decode
	if isCreate == true {
		input = vmInput
	}

	abi, err := GetAbi(code.Abi)
	if err != nil {
		return nil, err
	}
	gasRule := gas.NewGas(wavm.wavmConfig.DisableFloatingPoint)
	gasTable := wavm.ChainConfig().GasTable(wavm.Context.BlockNumber)
	gasCounter := gas.NewGasCounter(contract, gasTable)
	crx := ChainContext{
		CanTransfer: wavm.Context.CanTransfer,
		Transfer:    wavm.Context.Transfer,
		//TransferEx:  wavm.Context.TransferEx,
		GetHash:         wavm.Context.GetHash,
		StringToAddress: wavm.Context.StringToAddress,
		AddressToString: wavm.Context.AddressToString,
		// Message information
		Origin:   wavm.Context.Origin,
		GasPrice: wavm.Context.GasPrice,

		// Block information
		Coinbase:       wavm.Context.Coinbase,
		GasLimit:       wavm.Context.GasLimit,
		CoinUnit:       config.COIN_UNIT,
		BlockNumber:    wavm.Context.BlockNumber,
		Time:           wavm.Context.Time,
		Difficulty:     wavm.Context.Difficulty,
		Contract:       contract,
		StateDB:        wavm.StateDB.(*state.StateDB),
		Code:           code.Code,
		Abi:            abi,
		Wavm:           wavm,
		IsCreated:      isCreate,
		GasRule:        gasRule,
		GasCounter:     gasCounter,
		GasTable:       gasTable,
		StorageMapping: make(map[uint64]storage.StorageMapping),
	}
	newwawm := NewWavm(crx, wavm.wavmConfig, isCreate)
	wavm.Wavm = newwawm
	err = newwawm.InstantiateModule(code.Code, []uint8{})
	if err != nil {
		return nil, err
	}
	mutable := MutableFunction(abi, newwawm.Module)
	var res []byte
	if isCreate == true {
		// compile the wasm code: add gas counter, add statedb r/w
		compiled, err := CompileModule(newwawm.Module, crx, mutable)
		if err != nil {
			return nil, err
		}
		res, err = newwawm.Apply(input, compiled, mutable)
		if err != nil {
			return nil, err
		}
		compileres, err := json.Marshal(compiled)
		if err != nil {
			return nil, err
		}
		code.Compiled = compileres
		res = utils.CompressWasmAndAbi(code.Abi, code.Code, code.Compiled)
	} else {
		var compiled []darma.Compiled
		err = json.Unmarshal(code.Compiled, &compiled)
		if err != nil {
			return nil, err
		}
		res, err = newwawm.Apply(input, compiled, mutable)
		if err != nil {
			return nil, err
		}
	}
	return res, err
}

func NewWAVM(ctx vm.Context, statedb inter.StateDB, chainConfig *params.ChainConfig, vmConfig vm.Config) *WAVM {
	wavmConfig := Config{
		Debug:       vmConfig.Debug,
		Tracer:      vmConfig.Tracer,
		NoRecursion: vmConfig.NoRecursion,
	}
	wavm := &WAVM{
		Context:     ctx,
		StateDB:     statedb,
		wavmConfig:  wavmConfig,
		chainConfig: chainConfig,
		chainRules:  chainConfig.Rules(ctx.BlockNumber),
		mutable:     -1,
	}
	return wavm
}

func (wavm *WAVM) Cancel() {
	atomic.StoreInt32(&wavm.abort, 1)
}

func (wavm *WAVM) Create(caller vm.ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if wavm.depth > int(params.CallCreateDepth) {
		return nil, common.Address{}, gas, errorsmsg.ErrDepth
	}
	if !wavm.CanTransfer(wavm.StateDB, caller.Address(), value) {
		return nil, common.Address{}, gas, errorsmsg.ErrInsufficientBalance
	}
	// Ensure there's no existing contract already at the designated address
	nonce := wavm.StateDB.GetNonce(caller.Address())
	wavm.StateDB.SetNonce(caller.Address(), nonce+1)

	//contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	contractHash := wavm.StateDB.GetCodeHash(contractAddr)
	if wavm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, common.Address{}, 0, errorsmsg.ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := wavm.StateDB.Snapshot()
	wavm.StateDB.CreateAccount(contractAddr)
	wavm.StateDB.SetNonce(contractAddr, 1)
	wavm.Transfer(wavm.StateDB, caller.Address(), contractAddr, value, true)

	// initialise a new contract and set the code that is to be used by the
	// wavm. The contract is a scoped environment for this execution context
	// only.
	contract := wasmcontract.NewWASMContract(caller, vm.AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)

	if wavm.wavmConfig.NoRecursion && wavm.depth > 0 {
		return nil, contractAddr, gas, nil
	}

	if wavm.wavmConfig.Debug && wavm.depth == 0 {
		wavm.wavmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, code, gas, value)
	}
	start := time.Now()
	ret, err = runWavm(wavm, contract, nil, true)
	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > params.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * params.CreateDataGas / 2
		if contract.UseGas(createDataGas) {
			wavm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = errorsmsg.ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the wavm or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// also counts for code storage gas errors.
	if maxCodeSizeExceeded || err != nil {
		wavm.StateDB.RevertToSnapshot(snapshot)
		if err != nil && err.Error() != errorsmsg.ErrExecutionReverted.Error() {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = errorsmsg.ErrMaxCodeSizeExceeded
	}
	if wavm.wavmConfig.Debug && wavm.depth == 0 {
		wavm.wavmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	return ret, contractAddr, contract.Gas, err
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (wavm *WAVM) Call(caller vm.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if wavm.wavmConfig.NoRecursion && wavm.depth > 0 {
		rlog.Info("---Call-Return 1--")
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if wavm.depth > int(params.CallCreateDepth) {
		return nil, gas, errorsmsg.ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !wavm.Context.CanTransfer(wavm.StateDB, caller.Address(), value) {
		return nil, gas, errorsmsg.ErrInsufficientBalance
	}
	var (
		to       = vm.AccountRef(addr)
		snapshot = wavm.StateDB.Snapshot()
	)
	if !wavm.StateDB.Exist(addr) {
		precompiles := vm.PrecompiledContractsHubble
		if precompiles[addr] == nil && value.Sign() == 0 {
			// Calling a non existing account, don't do antything, but ping the tracer
			if wavm.wavmConfig.Debug && wavm.depth == 0 {
				wavm.wavmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				wavm.wavmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			rlog.Info("---Call-Return 2--")
			return nil, gas, nil
		}
		wavm.StateDB.CreateAccount(addr)
	}
	wavm.Transfer(wavm.StateDB, caller.Address(), to.Address(), value, false)

	// Initialise a new contract and set the code that is to be used by the WAVM.
	// The contract is a scoped environment for this execution context only.
	contract := wasmcontract.NewWASMContract(caller, to, value, gas)

	code := wavm.StateDB.GetCode(addr)

	contract.SetCallCode(&addr, wavm.StateDB.GetCodeHash(addr), code)

	start := time.Now()

	// Capture the tracer start/end events in debug mode
	if wavm.wavmConfig.Debug && wavm.depth == 0 {
		wavm.wavmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)

		defer func() { // Lazy evaluation of the parameters
			wavm.wavmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
		}()
	}
	ret, err = runWavm(wavm, contract, input, false)
	// When an error was returned by the WAVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// this also counts for code storage gas errors.
	if err != nil {
		wavm.StateDB.RevertToSnapshot(snapshot)
		if err.Error() != errorsmsg.ErrExecutionReverted.Error() && !bytes.Equal(to.Address().Bytes(), electionAddress.Bytes()) {
			contract.UseGas(contract.Gas)
		}
	}
	rlog.Info("---Call-Return last--")
	return ret, contract.Gas, err
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (wavm *WAVM) CallCode(caller vm.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if wavm.wavmConfig.NoRecursion && wavm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if wavm.depth > int(params.CallCreateDepth) {
		return nil, gas, errorsmsg.ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !wavm.CanTransfer(wavm.StateDB, caller.Address(), value) {
		return nil, gas, errorsmsg.ErrInsufficientBalance
	}

	var (
		snapshot = wavm.StateDB.Snapshot()
		to       = vm.AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// WAVM. The contract is a scoped environment for this execution context
	// only.
	contract := wasmcontract.NewWASMContract(caller, to, value, gas)

	code := wavm.StateDB.GetCode(addr)

	contract.SetCallCode(&addr, wavm.StateDB.GetCodeHash(addr), code)

	ret, err = runWavm(wavm, contract, input, false)
	if err != nil {
		wavm.StateDB.RevertToSnapshot(snapshot)
		// if err != errExecutionReverted {
		// 	contract.UseGas(contract.Gas)
		// }
	}
	return ret, contract.Gas, err
}
func (wavm *WAVM) DelegateCall(caller vm.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if wavm.wavmConfig.NoRecursion && wavm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if wavm.depth > int(params.CallCreateDepth) {
		return nil, gas, errorsmsg.ErrDepth
	}

	var (
		snapshot = wavm.StateDB.Snapshot()
		to       = vm.AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	ctr := wasmcontract.NewWASMContract(caller, to, nil, gas).AsDelegate()
	contract := ctr.(*wasmcontract.WASMContract)

	code := wavm.StateDB.GetCode(addr)

	contract.SetCallCode(&addr, wavm.StateDB.GetCodeHash(addr), code)

	ret, err = runWavm(wavm, contract, input, false)
	if err != nil {
		wavm.StateDB.RevertToSnapshot(snapshot)
		// if err != errExecutionReverted {
		// 	contract.UseGas(contract.Gas)
		// }
	}
	return ret, contract.Gas, err
}
func (wavm *WAVM) StaticCall(caller vm.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	return nil, 1, nil
}
func (wavm *WAVM) GetStateDb() inter.StateDB {
	return wavm.StateDB
}
func (wavm *WAVM) ChainConfig() *params.ChainConfig {
	return wavm.chainConfig
}

func (wavm *WAVM) GetContext() vm.Context {
	return wavm.Context
}

func (wavm *WAVM) GetOrigin() common.Address {
	return wavm.Origin
}

func (wavm *WAVM) GetTime() *big.Int {
	return wavm.Time
}

func (wavm *WAVM) GetBlockNum() *big.Int {
	return wavm.BlockNumber
}
