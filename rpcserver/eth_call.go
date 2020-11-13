// Copyright 2018-2020 Darma Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package rpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/common/hexutil"
	"github.com/darmaproject/darmasuite/dvm/common/math"
	"github.com/darmaproject/darmasuite/structures"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
	"reflect"
	"strings"
)

import "github.com/intel-go/fastjson"
import "github.com/osamingo/jsonrpc"

type EthWeb3JsRpcHandler_eth_call struct {}

type EthWeb3JsRpcParams_eth_call struct {
	args CallArgs
	blockNrOrHash BlockNumberOrHash
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *Web3Address    `json:"from"`
	To       *Web3Address    `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
}

type Web3Address [20]byte

var (
	addressT = reflect.TypeOf(Web3Address{})
)


func (bn *EthWeb3JsRpcParams_eth_call) UnmarshalJSON(data []byte) error {
	var params []interface{}
	if err := json.Unmarshal(data, &params); err != nil {
		return fmt.Errorf("unmarshal params from data error: {%s}",err.Error())
	}

	var bytes []byte
	var err error
	bytes,err = json.Marshal(params[0])
	if err != nil {
		return fmt.Errorf("marshal params[0] error: {%s}",err.Error())
	}

	if err := json.Unmarshal(bytes, &bn.args); err != nil {
		return fmt.Errorf("unmarshal bn.args from bytes(%s) error: {%s}",bytes,err.Error())
	}

	bytes,err = json.Marshal(params[1])
	if err != nil {
		return fmt.Errorf("marshal params[1] error: {%s}",err.Error())
	}

	if err := bn.blockNrOrHash.UnmarshalJSON(bytes); err != nil {
		return fmt.Errorf("unmarshal bn.blockNrOrHash from bytes(%x) error: {%s}",bytes,err.Error())
	}

	return nil
}

type BlockNumber int64

const (
	PendingBlockNumber  = BlockNumber(-2)
	LatestBlockNumber   = BlockNumber(-1)
	EarliestBlockNumber = BlockNumber(0)
)

// UnmarshalJSON parses the given JSON fragment into a BlockNumber. It supports:
// - "latest", "earliest" or "pending" as string arguments
// - the block number
// Returned errors:
// - an invalid block number error when the given argument isn't a known strings
// - an out of range error when the given block number is either too little or too large
func (bn *BlockNumber) UnmarshalJSON(data []byte) error {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	switch input {
	case "earliest":
		*bn = EarliestBlockNumber
		return nil
	case "latest":
		*bn = LatestBlockNumber
		return nil
	case "pending":
		*bn = PendingBlockNumber
		return nil
	}

	blckNum, err := hexutil.DecodeUint64(input)
	if err != nil {
		return err
	}
	if blckNum > math.MaxInt64 {
		return fmt.Errorf("block number larger than int64")
	}
	*bn = BlockNumber(blckNum)
	return nil
}

func (bn BlockNumber) Int64() int64 {
	return (int64)(bn)
}

type BlockNumberOrHash struct {
	BlockNumber      *BlockNumber `json:"blockNumber,omitempty"`
	BlockHash        *common.Hash `json:"blockHash,omitempty"`
	RequireCanonical bool         `json:"requireCanonical,omitempty"`
}

func (bnh *BlockNumberOrHash) UnmarshalJSON(data []byte) error {
	type erased BlockNumberOrHash
	e := erased{}
	err := json.Unmarshal(data, &e)
	if err == nil {
		if e.BlockNumber != nil && e.BlockHash != nil {
			return fmt.Errorf("cannot specify both BlockHash and BlockNumber, choose one or the other")
		}
		bnh.BlockNumber = e.BlockNumber
		bnh.BlockHash = e.BlockHash
		bnh.RequireCanonical = e.RequireCanonical
		return nil
	}
	var input string
	err = json.Unmarshal(data, &input)
	if err != nil {
		return err
	}
	switch input {
	case "earliest":
		bn := EarliestBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case "latest":
		bn := LatestBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case "pending":
		bn := PendingBlockNumber
		bnh.BlockNumber = &bn
		return nil
	default:
		if len(input) == 66 {
			hash := common.Hash{}
			err := hash.UnmarshalText([]byte(input))
			if err != nil {
				return err
			}
			bnh.BlockHash = &hash
			return nil
		} else {
			blckNum, err := hexutil.DecodeUint64(input)
			if err != nil {
				return err
			}
			if blckNum > math.MaxInt64 {
				return fmt.Errorf("blocknumber too high")
			}
			bn := BlockNumber(blckNum)
			bnh.BlockNumber = &bn
			return nil
		}
	}
}

func (bnh *BlockNumberOrHash) Number() (BlockNumber, bool) {
	if bnh.BlockNumber != nil {
		return *bnh.BlockNumber, true
	}
	return BlockNumber(0), false
}

func (bnh *BlockNumberOrHash) Hash() (common.Hash, bool) {
	if bnh.BlockHash != nil {
		return *bnh.BlockHash, true
	}
	return common.Hash{}, false
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Web3Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

func (h EthWeb3JsRpcHandler_eth_call) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	rlog.Infof("eth_call: params=%s\n",string(*params))

	var callParams EthWeb3JsRpcParams_eth_call
	if err := callParams.UnmarshalJSON(*params); err != nil {
		rlog.Errorf("unmarshal callparams from params error: {%s}\n",err.Error())
		return nil, jsonrpc.ErrInvalidParams()
	}

	if callParams.args.From == nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("`from` is null")}
	}
	if callParams.args.To == nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("`to` is null")}
	}
	if callParams.args.Data == nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("`data` is null")}
	}

	var from, to common.Address
	copy(from[12:],(*callParams.args.From)[:])
	copy(to[12:],(*callParams.args.To)[:])

	data := make([]byte,len(*callParams.args.Data))
	copy(data,*callParams.args.Data)

	rlog.Debugf("eth_call from=0x%x, to=0x%x, data=%x\n",from,to,data)

	blockNumber,_ := callParams.blockNrOrHash.Number()
	rlog.Debugf("eth_call blockNumber=%d\n",blockNumber)

	var p structures.CallContractParams
	p.To = hexutil.Encode(to[:])
	p.Data = hexutil.Encode(data)
	if blockNumber > 0 {
		p.TopoHeight = blockNumber.Int64()
	}

	if p.Gas > 0 && p.Gas < config.MIN_GASLIMIT {
		rlog.Warnf("Request param 'gas' is not enough")
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Gas is not enough")}
	}
	if p.Gas == 0 {
		p.Gas = config.DEFAULT_GASLIMIT
	}
	if p.GasPrice > 0 && p.GasPrice < config.MIN_GASPRICE {
		rlog.Warnf("Request param 'gasPrice' is not enough")
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("GasPrice is not enough")}
	}
	if p.GasPrice == 0 {
		p.GasPrice = config.DEFAULT_GASPRICE
	}

	payload, err := hexutil.Decode(p.Data)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Data is invalid")}
	}

	if callParams.args.From == nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("From is null")}
	}

	sender := from

	scdata := &transaction.SCData{
		Sender:       sender,
		AccountNonce: p.Nonce,
		Price:        p.GasPrice,
		GasLimit:     p.Gas,
		Amount:       p.Amount,
		Recipient:    to,
		Payload:      payload,
	}

	rlog.Debugf("eth_call scdata: {sender:%x, nonce:%d, price:%d, gaslimit:%d, amount:%d, recipient:%x, payload:%x}",scdata.Sender,scdata.AccountNonce,scdata.Price,scdata.GasLimit,scdata.Amount,scdata.Recipient,scdata.Payload)
	res, err := chain.CallContact(scdata, p.TopoHeight)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("call failed: %s",err.Error())}
	}

	return fmt.Sprintf("0x%x", res), nil
}
