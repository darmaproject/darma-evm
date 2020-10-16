// Copyright 2015 The go-ethereum Authors
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

package vm

import (
	"github.com/darmaproject/darmasuite/dvm/common"
)

/*
// ContractRef is a reference to the contract's backing object
type ContractRef interface {
	Base58Address() string
	Address() common.Address
}

// AccountRef implements ContractRef.
//
// Account references are used during VM initialisation and
// it's primary use is to fetch addresses. Removing this object
// proves difficult because of the cached jump destinations which
// are fetched from the parent contract (i.e. the caller), which
// is a ContractRef.
type AccountRef struct {
	base58Address string
	address       common.Address
}

func NewAddressFromBase58(addr string) common.Address {
	hash := crypto.Keccak256([]byte(addr))

	return common.BytesToAddress(hash[:])
}

func NewAccountRef(base58Addr string) AccountRef {
	return AccountRef{
		base58Address: base58Addr,
		address:       NewAddressFromBase58(base58Addr),
	}
}

func NewAccountRefFromContractRef(addr ContractRef) AccountRef {
	return AccountRef{
		base58Address: addr.Base58Address(),
		address:       addr.Address(),
	}
}

func NewAccountRefForContract(address common.Address) AccountRef {
	return AccountRef{
		base58Address: "",
		address:       address,
	}
}*/

// ContractRef is a reference to the contract's backing object
type ContractRef interface {
	Address() common.Address
}

// AccountRef implements ContractRef.
//
// Account references are used during VM initialisation and
// it's primary use is to fetch addresses. Removing this object
// proves difficult because of the cached jump destinations which
// are fetched from the parent contract (i.e. the caller), which
// is a ContractRef.
type AccountRef common.Address

/*func NewAddressFromBase58(addr string) AccountRef {
	hash := crypto.Keccak256([]byte(addr))

	return AccountRef(common.BytesToAddress(hash[:]))
}*/

// Address casts AccountRef to a Address
func (ar AccountRef) Address() common.Address { return (common.Address)(ar) }
