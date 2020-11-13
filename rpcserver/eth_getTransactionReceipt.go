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
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/common/hexutil"
	"github.com/darmaproject/darmasuite/dvm/core/types"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
)

import "github.com/intel-go/fastjson"
import "github.com/osamingo/jsonrpc"

type CompatibleLog struct {

	// length of `common.Address` is 32 bytes, but this `Address` field in log needs 20 bytes only!
	// so we use `EthAddress` to format the 20 bytes address.
	Address EthAddress `json:"address" gencodec:"required"`

	// see Log.MarshalJSON() in gen_log_json.go
	Topics      []common.Hash  `json:"topics" gencodec:"required"`
	Data        hexutil.Bytes  `json:"data" gencodec:"required"`
	BlockNumber hexutil.Uint64 `json:"blockNumber"`
	TxHash      common.Hash    `json:"transactionHash" gencodec:"required"`
	TxIndex     hexutil.Uint   `json:"transactionIndex"`
	BlockHash   common.Hash    `json:"blockHash"`
	Index       hexutil.Uint   `json:"logIndex"`
	Removed     bool           `json:"removed"`
}

type EthWeb3JsRpcHandler_eth_getTransactionReceipt struct {
	r *RPCServer
}

type EthAddress common.Address

func (h EthWeb3JsRpcHandler_eth_getTransactionReceipt) ServeJSONRPC(ctx context.Context, rawMessage *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	rlog.Debugf("eth_getTransactionReceipt: rawMessage=%s\n",string(*rawMessage))

	var params []interface{}
	if err := json.Unmarshal(*rawMessage, &params); err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("unmarshal params from raw message error: %s",err.Error())}
	}

	var params0bytes []byte // included '"' : "0x00"
	var err error
	params0bytes,err = json.Marshal(params[0])
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("marshal bytes from params[0] error: %s",err.Error())}
	}

	var txhashBytesString string
	if err := json.Unmarshal(params0bytes,&txhashBytesString); err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("unmarshal raw-tx-bytes-string from bytes error: %s",err.Error())}
	}

	txhashBytes, err := hexutil.Decode(txhashBytesString)
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("decode raw tx bytes error: %s",err.Error())}
	}

	var txhash crypto.Hash
	copy(txhash[:],txhashBytes)

	tx, err := chain.LoadTxFromId(nil, txhash)
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("load tx error: %s",err.Error())}
	}

	if !chain.IsContractTransaction(tx) {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("tx is not contract transaction")}
	}

	scData := tx.ExtraMap[transaction.TX_EXTRA_CONTRACT].(*transaction.SCData)

	receipt,err := chain.LoadTxReceipt(nil,txhash)
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("load tx receipt error: %s",err.Error())}
	}

	var blockNumber uint64
	txHeight := chain.LoadTxHeight(nil, txhash)
	if txHeight > 0 {
		blockNumber = uint64(txHeight)
	}

	var validBlockHash crypto.Hash
	blocks := chain.Load_TX_blocks(nil, txhash)
	for i := range blocks {
		if chain.IsTxValid(nil, blocks[i], txhash) && chain.Is_Block_Topological_order(nil, blocks[i]) {
			validBlockHash = blocks[i]
			break
		}
	}

	block,err := chain.LoadBlFromId(nil,validBlockHash)
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("load block error: %s",err.Error())}
	}

	txIndex := uint(0)
	for i,hash := range block.TxHashes {
		if hash == txhash {
			txIndex = uint(i)
			break
		}
	}

	receipt.DeriveFields(blockNumber,common.Hash(validBlockHash),common.Hash(txhash),txIndex)

	var clogs []*CompatibleLog
	for _,log := range receipt.Logs {
		clog := AdaptWeb3jsLog(log)
		clogs = append(clogs,clog)
	}

	var to *EthAddress
	var ZEROSCADDR common.Address
	if scData.Recipient != ZEROSCADDR {
		ethAddress := EthAddress(scData.Recipient)
		to = &ethAddress
	}

	fields := map[string]interface{}{
		"blockHash":         common.Hash(validBlockHash),
		"blockNumber":       hexutil.Uint64(blockNumber),
		"transactionHash":   common.Hash(txhash),
		"transactionIndex":  hexutil.Uint64(receipt.TransactionIndex),
		"from":              EthAddress(scData.Sender),
		"to":                to,
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              clogs, // NOTE: The address cannot be resolved when use `receipt.Logs`
		"logsBloom":         receipt.Bloom,
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	} else {
		fields["status"] = hexutil.Uint(receipt.Status)
	}
	if clogs == nil {
		fields["logs"] = [][]*CompatibleLog{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = EthAddress(receipt.ContractAddress)
	}
	return &fields, nil
}

func AdaptWeb3jsLog(log *types.Log) *CompatibleLog {
	var clog CompatibleLog
	clog.Address = EthAddress(log.Address)
	// copy other members from log
	clog.Topics = log.Topics
	clog.Data = log.Data
	clog.BlockNumber = hexutil.Uint64(log.BlockNumber)
	clog.TxHash = log.TxHash
	clog.TxIndex = hexutil.Uint(log.TxIndex)
	clog.BlockHash = log.BlockHash
	clog.Index = hexutil.Uint(log.Index)
	clog.Removed = log.Removed
	rlog.Debugf("AdaptWeb3jsLog | log.Data=%x clog.Data=%x",log.Data,clog.Data)
	return &clog
}

func (a EthAddress) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[12:]).MarshalText()
}

