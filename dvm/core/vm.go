// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package dvm

import (
	"github.com/darmaproject/darmasuite/dvm/core/evm"
	"github.com/darmaproject/darmasuite/dvm/core/types"
	"github.com/darmaproject/darmasuite/dvm/core/wavm"
	"github.com/darmaproject/darmasuite/dvm/params"
	"math/big"

	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/vm"
	"github.com/darmaproject/darmasuite/dvm/core/vm/interface"
)

// NewVMContext creates a new context for use in the VM.
func NewVMContext(msg Message, header *types.Header, origin common.Address, hashfunc vm.GetHashFunc, strToAddrFunc vm.StringToAddress, addrToStrFunc vm.AddressToString) vm.Context {
	// Can't get miner's address
	beneficiary := msg.From()

	return vm.Context{
		CanTransferFunc: CanTransfer,
		TransferFunc:    Transfer,
		GetHash:         hashfunc,
		StringToAddress: strToAddrFunc,
		AddressToString: addrToStrFunc,
		Origin:          origin,
		Coinbase:        beneficiary,
		BlockNumber:     new(big.Int).Set(header.Number),
		Time:            new(big.Int).SetUint64(header.Time),
		Difficulty:      new(big.Int).Set(header.Difficulty),
		GasLimit:        header.GasLimit,
		GasPrice:        new(big.Int).Set(msg.GasPrice()),
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db inter.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db inter.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

func GetVM(msg Message, ctx vm.Context, statedb inter.StateDB, chainConfig *params.ChainConfig, vmConfig vm.Config) vm.VM {
	if chainConfig.IsEVM(ctx.BlockNumber){
		//Fixme:
		return evm.NewEVM(ctx, statedb, chainConfig, evm.Config{Debug:false})
	}
	return wavm.NewWAVM(ctx, statedb, chainConfig, vmConfig)
}
