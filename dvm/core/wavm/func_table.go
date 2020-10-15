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
	"reflect"

	"github.com/darmaproject/darma-wasm/wasm"
)

const (
	OpNameGetBalanceFromAddress = "GetBalanceFromAddress"
	OpNameGetBlockNumber        = "GetBlockNumber"
	OpNameGetGas                = "GetGas"
	OpNameGetBlockHash          = "GetBlockHash"
	OpNameGetBlockProduser      = "GetBlockProduser"
	OpNameGetTimestamp          = "GetTimestamp"
	OpNameGetOrigin             = "GetOrigin"
	OpNameGetSender             = "GetSender"
	OpNameGetGasLimit           = "GetGasLimit"
	OpNameGetCoinUnit           = "GetCoinUnit"
	OpNameGetValue              = "GetValue"
	OpNameSHA3                  = "SHA3"
	OpNameEcrecover             = "Ecrecover"
	OpNameGetContractAddress    = "GetContractAddress"
	OpNameAssert                = "Assert"

	OpNameEvent = "Event"

	OpNamePrintAddress  = "PrintAddress"
	OpNamePrintStr      = "PrintStr"
	OpNamePrintQStr     = "PrintQStr"
	OpNamePrintUint64T  = "PrintUint64T"
	OpNamePrintUint32T  = "PrintUint32T"
	OpNamePrintInt64T   = "PrintInt64T"
	OpNamePrintInt32T   = "PrintInt32T"
	OpNamePrintUint256T = "PrintUint256T"

	OpNameFromI64 = "FromI64"
	OpNameFromU64 = "FromU64"
	OpNameToI64   = "ToI64"
	OpNameToU64   = "ToU64"
	OpNameConcat  = "Concat"
	OpNameEqual   = "Equal"

	OpNameSendFromContract     = "SendFromContract"
	OpNameTransferFromContract = "TransferFromContract"

	OpNameContractCall = "ContractCall"

	//string to address
	OpNameAddressFrom     = "AddressFrom"
	OpNameAddressToString = "AddressToString"
	OpNameU256From        = "U256From"
	OpNameU256ToString    = "U256ToString"

	OpNameAddKeyInfo          = "AddKeyInfo"
	OpNameWriteWithPointer    = "WriteWithPointer"
	OpNameReadWithPointer     = "ReadWithPointer"
	OpNameInitializeVariables = "InitializeVariables"

	//uint256
	OpNameU256FromU64 = "U256FromU64"
	OpNameU256FromI64 = "U256FromI64"
	OpNameU256Add     = "U256_Add"
	OpNameU256Sub     = "U256_Sub"
	OpNameU256Mul     = "U256_Mul"
	OpNameU256Div     = "U256_Div"
	OpNameU256Mod     = "U256_Mod"
	OpNameU256Pow     = "U256_Pow"
	OpNameU256Cmp     = "U256_Cmp"
	OpNameU256Shl     = "U256_Shl"
	OpNameU256Shr     = "U256_Shr"
	OpNameU256And     = "U256_And"
	OpNameU256Or      = "U256_Or"
	OpNameU256Xor     = "U256_Xor"

	//math
	OpNamePow = "Pow"

	//add gas
	OpNameAddGas = "AddGas"

	OpNameRevert = "Revert"

	//qlang
	OpNameSender = "Sender"
	OpNameLoad   = "Load"
	OpNameStore  = "Store"
)

func (ef *EnvFunctions) getFuncTable() map[string]wasm.Function {
	// If params in darmalib.h changed，this func signature needs to change，or wasm validate won't pass.
	var func_table = map[string]wasm.Function{
		OpNameGetBalanceFromAddress: {
			Host: reflect.ValueOf(ef.GetBalanceFromAddress),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetGas: {
			Host: reflect.ValueOf(ef.GetGas),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetBlockHash: {
			Host: reflect.ValueOf(ef.GetBlockHash),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetBlockProduser: {
			Host: reflect.ValueOf(ef.GetBlockProduser),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetTimestamp: {
			Host: reflect.ValueOf(ef.GetTimestamp),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetOrigin: {
			Host: reflect.ValueOf(ef.GetOrigin),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetSender: {
			Host: reflect.ValueOf(ef.GetSender),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetGasLimit: {
			Host: reflect.ValueOf(ef.GetGasLimit),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetCoinUnit: {
			Host: reflect.ValueOf(ef.GetCoinUnit),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetValue: {
			Host: reflect.ValueOf(ef.GetValue),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetBlockNumber: {
			Host: reflect.ValueOf(ef.GetBlockNumber),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameSHA3: {
			Host: reflect.ValueOf(ef.SHA3),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameEcrecover: {
			Host: reflect.ValueOf(ef.Ecrecover),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32, wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameGetContractAddress: {
			Host: reflect.ValueOf(ef.GetContractAddress),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameAssert: {
			Host: reflect.ValueOf(ef.Assert),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameSendFromContract: {
			Host: reflect.ValueOf(ef.SendFromContract),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameTransferFromContract: {
			Host: reflect.ValueOf(ef.TransferFromContract),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameEvent: {
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintAddress: {
			Host: reflect.ValueOf(ef.PrintAddress),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintStr: {
			Host: reflect.ValueOf(ef.PrintStr),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintQStr: {
			Host: reflect.ValueOf(ef.PrintQStr),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintUint64T: {
			Host: reflect.ValueOf(ef.PrintUint64T),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintUint32T: {
			Host: reflect.ValueOf(ef.PrintUint32T),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintInt64T: {
			Host: reflect.ValueOf(ef.PrintInt64T),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintInt32T: {
			Host: reflect.ValueOf(ef.PrintInt32T),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePrintUint256T: {
			Host: reflect.ValueOf(ef.PrintUint256T),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameFromI64: {
			Host: reflect.ValueOf(ef.fromI64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameFromU64: {
			Host: reflect.ValueOf(ef.fromU64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameToI64: {
			Host: reflect.ValueOf(ef.toI64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameToU64: {
			Host: reflect.ValueOf(ef.toU64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameConcat: {
			Host: reflect.ValueOf(ef.Concat),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameEqual: {
			Host: reflect.ValueOf(ef.Equal),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameContractCall: {
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameAddressFrom: {
			Host: reflect.ValueOf(ef.AddressFrom),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameAddressToString: {
			Host: reflect.ValueOf(ef.AddressToString),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256From: {
			Host: reflect.ValueOf(ef.U256From),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256ToString: {
			Host: reflect.ValueOf(ef.U256ToString),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256FromU64: {
			Host: reflect.ValueOf(ef.U256FromU64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256FromI64: {
			Host: reflect.ValueOf(ef.U256FromI64),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameAddKeyInfo: {
			Host: reflect.ValueOf(ef.AddKeyInfo),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64, wasm.ValueTypeI32, wasm.ValueTypeI64, wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameWriteWithPointer: {
			Host: reflect.ValueOf(ef.WriteWithPointer),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64, wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameReadWithPointer: {
			Host: reflect.ValueOf(ef.ReadWithPointer),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64, wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameInitializeVariables: {
			Host: reflect.ValueOf(ef.InitializeVariables),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Add: {
			Host: reflect.ValueOf(ef.U256Add),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Sub: {
			Host: reflect.ValueOf(ef.U256Sub),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Mul: {
			Host: reflect.ValueOf(ef.U256Mul),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Div: {
			Host: reflect.ValueOf(ef.U256Div),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Mod: {
			Host: reflect.ValueOf(ef.U256Mod),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Pow: {
			Host: reflect.ValueOf(ef.U256Pow),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Cmp: {
			Host: reflect.ValueOf(ef.U256Cmp),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Shl: {
			Host: reflect.ValueOf(ef.U256Shl),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Shr: {
			Host: reflect.ValueOf(ef.U256Shr),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256And: {
			Host: reflect.ValueOf(ef.U256And),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Xor: {
			Host: reflect.ValueOf(ef.U256Xor),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameU256Or: {
			Host: reflect.ValueOf(ef.U256Or),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNamePow: {
			Host: reflect.ValueOf(ef.Pow),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64, wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI64},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameAddGas: {
			Host: reflect.ValueOf(ef.AddGas),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameRevert: {
			Host: reflect.ValueOf(ef.Revert),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameSender: {
			Host: reflect.ValueOf(ef.Sender),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameLoad: {
			Host: reflect.ValueOf(ef.Load),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
		OpNameStore: {
			Host: reflect.ValueOf(ef.Store),
			Sig: &wasm.FunctionSig{
				ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
				ReturnTypes: []wasm.ValueType{},
			},
			Body: &wasm.FunctionBody{
				Code: []byte{},
			},
		},
	}
	return func_table
}

//
//func ResolveHostFunc() map[FieldName]*Function {
//	for k, v := range resolveHostFunc {
//		v.IsHost = true
//		v.FieldName = k
//		resolveHostFunc[k] = v
//	}
//	return resolveHostFunc
//}
