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

package vm

import (
	"github.com/romana/rlog"
	"math/big"

	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/vm/interface"
	"github.com/darmaproject/darmasuite/dvm/crypto"
	"github.com/darmaproject/darmasuite/dvm/params"
)

type OPCode interface {
	String() string
	IsPush() bool
	Byte() byte
}

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	CanTransferFunc func(inter.StateDB, common.Address, *big.Int) bool
	TransferFunc    func(inter.StateDB, common.Address, common.Address, *big.Int)
	TransferExFunc  func(string, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	GetHashFunc     func(uint64) common.Hash
	StringToAddress func(string) []byte
	AddressToString func([]byte) string
)

// any external tranfers
type TransferExternal struct {
	Address common.Address `msgpack:"A,omitempty" json:"A,omitempty"` //  transfer to this blob
	Amount  uint64         `msgpack:"V,omitempty" json:"V,omitempty"` // Amount in Atomic units
}

type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransferFunc CanTransferFunc
	// Transfer transfers ether from one account to the other
	TransferFunc TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash         GetHashFunc
	StringToAddress StringToAddress
	AddressToString AddressToString

	// Message information
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        *big.Int       // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY

	//External transfers
	TxStorage []TransferExternal `msgpack:"T,omitempty"` // all external transfers
}

func (ctx *Context) CanTransfer(db inter.StateDB, addr common.Address, amount *big.Int) bool {
	return ctx.CanTransferFunc(db, addr, amount)
}

func (ctx *Context) Transfer(db inter.StateDB, sender, recipient common.Address, amount *big.Int, isCreate bool) bool {
	rlog.Infof("vm.Context.Transfer %d from %x to %x",amount,sender,recipient)

	ctx.TransferFunc(db, sender, recipient, amount)

	//External transfers only support non-contract addresses
	code := db.GetCode(recipient)
	if !isCreate && len(code) == 0 {
		ctx.TransferEx(recipient, amount)
	}

	return true
}

//TransferCallback any external tranfers from contract address
func (ctx *Context) TransferEx(recipient common.Address, amount *big.Int) {
	value := amount.Uint64()
	if ctx.TxStorage == nil {
		ctx.TxStorage = make([]TransferExternal, 0)
	}

	txEntry := TransferExternal{recipient, value}
	ctx.TxStorage = append(ctx.TxStorage, txEntry)
}

type VM interface {
	Cancel()
	Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error)
	Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error)
	CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error)
	DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	GetStateDb() inter.StateDB
	ChainConfig() *params.ChainConfig
	GetContext() Context
}
