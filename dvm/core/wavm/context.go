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
	"math/big"

	"github.com/darmaproject/darmasuite/dvm/accounts/abi"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/state"
	inter "github.com/darmaproject/darmasuite/dvm/core/vm/interface"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/contract"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/gas"
	"github.com/darmaproject/darmasuite/dvm/core/wavm/storage"
	"github.com/darmaproject/darmasuite/dvm/params"
)

type ChainContext struct {
	// CanTransfer returns whether the account contains
	// sufficient darma to transfer the value
	CanTransfer func(inter.StateDB, common.Address, *big.Int) bool
	// Transfer transfers darma from one account to the other
	Transfer func(inter.StateDB, common.Address, common.Address, *big.Int, bool) bool
	//TransferEx transfers to any external address from the contract account
	//TransferEx func(string, *big.Int)
	// GetHash returns the hash corresponding to n
	GetHash         func(uint64) common.Hash
	StringToAddress func(string) []byte
	AddressToString func([]byte) string
	// Message information
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase       common.Address // Provides information for COINBASE
	GasLimit       uint64         // Provides information for GASLIMIT
	CoinUnit       uint64         // Provides information for COINUNIT
	BlockNumber    *big.Int       // Provides information for NUMBER
	Time           *big.Int       // Provides information for TIME
	Difficulty     *big.Int       // Provides information for DIFFICULTY
	StateDB        *state.StateDB
	Contract       *contract.WASMContract
	Code           []byte  //Wasm contract code
	Abi            abi.ABI //Wasm contract abi
	Wavm           *WAVM
	IsCreated      bool
	StorageMapping map[uint64]storage.StorageMapping
	GasRule        gas.Gas
	GasCounter     gas.GasCounter
	GasTable       params.GasTable
}
