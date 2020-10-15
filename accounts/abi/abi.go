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

package abi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// The ABI holds information about a contract's context and available
// invokable methods. It will allow you to type check function calls and
// packs data accordingly.
type ABI struct {
	Constructor Method
	Methods     map[string]Method
	Events      map[string]Event
	Calls       map[string]Method
	// Keys        map[string]Key
}

// JSON returns a parsed ABI interface and error if it failed.
func JSON(reader io.Reader) (ABI, error) {
	dec := json.NewDecoder(reader)
	var abi ABI
	if err := dec.Decode(&abi); err != nil {
		return ABI{}, err
	}

	return abi, nil
}

// Pack the given method name to conform the ABI. Method call's data
// will consist of method_id, args0, arg1, ... argN. Method id consists
// of 4 bytes and arguments are all 32 bytes.
// Method ids are created from the first 4 bytes of the hash of the
// methods string signature. (signature = baz(uint32,string32))
func (abi ABI) Pack(name string, args ...interface{}) ([]byte, error) {
	// Fetch the ABI of the requested method
	if name == "" || name == abi.Constructor.Name {
		// constructor
		arguments, err := abi.Constructor.Inputs.Pack(args...)
		if err != nil {
			return nil, err
		}
		return arguments, nil

	}
	method, exist := abi.Methods[name]
	if !exist {
		method, exist = abi.Calls[name]
		if !exist {
			return nil, fmt.Errorf("method or call '%s' not found", name)
		}
	}
	arguments, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	// Pack up the method ID too if not a constructor and return
	return append(method.Id(), arguments...), nil
}

func (abi ABI) PackStrArgs(name string, args ...string) ([]byte, error) {
	// Fetch the ABI of the requested method
	if name == "" || name == abi.Constructor.Name {
		// constructor
		arguments, err := abi.Constructor.Inputs.PackStrArgs(args...)
		if err != nil {
			return nil, err
		}
		return arguments, nil

	}
	method, exist := abi.Methods[name]
	if !exist {
		method, exist = abi.Calls[name]
		if !exist {
			return nil, fmt.Errorf("method or call '%s' not found", name)
		}
	}
	arguments, err := method.Inputs.PackStrArgs(args...)
	if err != nil {
		return nil, err
	}
	// Pack up the method ID too if not a constructor and return
	return append(method.Id(), arguments...), nil
}

// Unpack output in v according to the abi specification
func (abi ABI) Unpack(v interface{}, name string, output []byte) (err error) {
	if len(output) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(output)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Outputs.Unpack(v, output)
	} else if event, ok := abi.Events[name]; ok {
		return event.Inputs.Unpack(v, output)
	} else if call, ok := abi.Calls[name]; ok {
		return call.Outputs.Unpack(v, output)
	}
	return fmt.Errorf("abi: could not locate named method or event or call")
}

func (abi ABI) UnpackStrArgs(name string, output []byte) (string, error) {
	if len(output) == 0 {
		return "", fmt.Errorf("abi: unmarshalling empty output")
	}
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(output)%32 != 0 {
			return "", fmt.Errorf("abi: improperly formatted output")
		}
		return method.Outputs.UnpackStrArgs(output)
	} else if event, ok := abi.Events[name]; ok {
		return event.Inputs.UnpackStrArgs(output)
	} else if call, ok := abi.Calls[name]; ok {
		return call.Outputs.UnpackStrArgs(output)
	}
	return "", fmt.Errorf("abi: could not locate named method or event or call")
}

// Unpack Input in v according to the abi specification
func (abi ABI) UnpackInput(v interface{}, name string, input []byte) (err error) {
	if len(input) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(input)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Inputs.Unpack(v, input)
	}
	return fmt.Errorf("abi: could not locate named method")
}

// UnmarshalJSON implements json.Unmarshaler interface
func (abi *ABI) UnmarshalJSON(data []byte) error {
	var fields []struct {
		Type      string
		Name      string
		Constant  bool
		Anonymous bool
		Inputs    []Argument
		Outputs   []Argument
		Tables    []Table
	}

	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	abi.Methods = make(map[string]Method)
	abi.Events = make(map[string]Event)
	abi.Calls = make(map[string]Method)
	// abi.Keys = make(map[string]Key)
	for _, field := range fields {
		switch field.Type {
		case "constructor":
			abi.Constructor = Method{
				Name:    field.Name,
				Inputs:  field.Inputs,
				Outputs: field.Outputs,
			}
		// empty defaults to function according to the abi spec
		case "function", "":
			abi.Methods[field.Name] = Method{
				Name:    field.Name,
				Const:   field.Constant,
				Inputs:  field.Inputs,
				Outputs: field.Outputs,
			}
		case "call":
			abi.Calls[field.Name] = Method{
				Name:    field.Name,
				Const:   field.Constant,
				Inputs:  field.Inputs,
				Outputs: field.Outputs,
			}
		case "event":
			abi.Events[field.Name] = Event{
				Name:      field.Name,
				Anonymous: field.Anonymous,
				Inputs:    field.Inputs,
			}
			// case "key":
			// 	key := Key{
			// 		Name:   field.Name,
			// 		Tables: field.Tables,
			// 	}
			// 	key.KeyTraversal()
			// 	abi.Keys[field.Name] = key
		}
	}

	return nil
}

// MethodById looks up a method by the 4-byte id
// returns nil if none found
func (abi *ABI) MethodById(sigdata []byte) (*Method, error) {
	for _, method := range abi.Methods {
		if bytes.Equal(method.Id(), sigdata[:4]) {
			return &method, nil
		}
	}
	return nil, fmt.Errorf("no method with id: %#x", sigdata[:4])
}

func (abi *ABI) IsERC20() error {
	methods := []string{"totalSupply", "balanceOf", "allowance", "transfer", "approve", "transferFrom"}
	events := []string{"Transfer", "Approval"}

	for _, name := range methods {
		v, ok := abi.Methods[name]
		if !ok {
			return fmt.Errorf("no %s", name)
		}

		switch name {
		case "totalSupply":
			if !v.Const {
				return fmt.Errorf("%s is not const", name)
			}
			if len(v.Inputs) > 0 {
				return fmt.Errorf("%s can't have %d inputs", name, len(v.Inputs))
			}
			if len(v.Outputs) != 1 {
				return fmt.Errorf("%s can't have %d outputs", name, len(v.Outputs))
			}
			if v.Outputs[0].Type.T != UintTy {
				return fmt.Errorf("%s have invalid output type", name)
			}
		case "balanceOf":
			fallthrough
		case "allowance":
			if !v.Const {
				return fmt.Errorf("%s is not const", name)
			}
			if len(v.Inputs) != 1 {
				return fmt.Errorf("%s can't have %d inputs", name, len(v.Inputs))
			}
			if v.Inputs[0].Type.T != AddressTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if len(v.Outputs) != 1 {
				return fmt.Errorf("%s can't have %d outputs", name, len(v.Outputs))
			}
			if v.Outputs[0].Type.T != UintTy {
				return fmt.Errorf("%s have invalid output type", name)
			}
		case "transfer":
			fallthrough
		case "approve":
			if v.Const {
				return fmt.Errorf("%s can't be const", name)
			}
			if len(v.Inputs) != 2 {
				return fmt.Errorf("%s can't have %d inputs", name, len(v.Inputs))
			}
			if v.Inputs[0].Type.T != AddressTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if v.Inputs[1].Type.T != UintTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if len(v.Outputs) != 1 {
				return fmt.Errorf("%s can't have %d outputs", name, len(v.Outputs))
			}
			if v.Outputs[0].Type.T != BoolTy {
				return fmt.Errorf("%s have invalid output type", name)
			}
		case "transferFrom":
			if v.Const {
				return fmt.Errorf("%s can't be const", name)
			}
			if len(v.Inputs) != 3 {
				return fmt.Errorf("%s can't have %d inputs", name, len(v.Inputs))
			}
			if v.Inputs[0].Type.T != AddressTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if v.Inputs[1].Type.T != AddressTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if v.Inputs[2].Type.T != UintTy {
				return fmt.Errorf("%s have invalid input type", name)
			}
			if len(v.Outputs) != 1 {
				return fmt.Errorf("%s can't have %d outputs", name, len(v.Outputs))
			}
			if v.Outputs[0].Type.T != BoolTy {
				return fmt.Errorf("%s have invalid output type", name)
			}
		}
	}

	for _, name := range events {
		v, ok := abi.Events[name]
		if !ok {
			return fmt.Errorf("no %s", name)
		}

		switch name {
		case "Transfer":
			fallthrough
		case "Approval":
			if len(v.Inputs) != 3 {
				return fmt.Errorf("%s can't have %d inputs", name, len(v.Inputs))
			}
			if !v.Inputs[0].Indexed || v.Inputs[0].Type.T != AddressTy {
				return fmt.Errorf("%s, invalid input", name)
			}
			if !v.Inputs[1].Indexed || v.Inputs[1].Type.T != AddressTy {
				return fmt.Errorf("%s, invalid input", name)
			}
			if v.Inputs[2].Indexed || v.Inputs[2].Type.T != UintTy {
				return fmt.Errorf("%s, invalid input", name)
			}
		}
	}

	return nil
}
