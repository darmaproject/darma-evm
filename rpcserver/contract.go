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

// get block template handler not implemented

//import "fmt"
import (
	"context"
	"fmt"
	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/common/hexutil"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
)

//import	"log"
//import 	"net/http"

import "github.com/intel-go/fastjson"
import "github.com/osamingo/jsonrpc"

import "github.com/darmaproject/darmasuite/crypto"
import "github.com/darmaproject/darmasuite/structures"

type GetContractHandler struct{}

func (h GetContractHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p structures.GetContractAddressByTxHashParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	hash := crypto.HashHexToHash(p.Hash)
	addr, err := chain.LoadContractAddress(nil, hash)

	if err != nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Load Contract Address Failed, err %s", err)}
	}

	return structures.GetContractAddressByTxHashResult{ // return success
		Address: addr.String(),
	}, nil
}

type CallContractHandler struct {
	r *RPCServer
}

func (h CallContractHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p structures.CallContractParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
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
		p.Gas = config.DEFAULT_GASPRICE
	}

	payload, err := hexutil.Decode(p.Data)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Data is invalid")}
	}

	var sender common.Address
	from, err := address.NewAddress(p.From)
	if err != nil {
		rlog.Warnf("Request param 'From' is invalid")
	} else {
		sender = from.ToContractAddress()
	}

	to, err := address.NewAddress(p.To)
	if err != nil {
		rlog.Warnf("Request param 'To' is invalid")
		return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("'To' is not a valid address")}
	}

	scdata := &transaction.SCData{
		Sender:       sender,
		AccountNonce: p.Nonce,
		Price:        p.GasPrice,
		GasLimit:     p.Gas,
		Amount:       p.Amount,
		Recipient:    to.ToContractAddress(),
		Payload:      payload,
	}

	res, err := chain.CallContact(scdata, p.TopoHeight)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -2, Message: err.Error()}
	}

	return structures.CallContractResult{
		Data: fmt.Sprintf("%x", res),
	}, nil
}

type GetContractResultHandler struct {
	r *RPCServer
}

func (h GetContractResultHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p structures.GetContractResultParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	txHash := crypto.HashHexToHash(p.TXHash)

	ret, err := chain.LoadContractTxResult(nil, txHash)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -1, Message: fmt.Sprintf("no result of tx %s", p.TXHash)}
	}

	return structures.GetContractResultResult{
		Data: fmt.Sprintf("%x", ret),
	}, nil
}

type GetBalanceOfContractAccountHandler struct {
	r *RPCServer
}

func (h GetBalanceOfContractAccountHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p structures.GetBalanceOfContractAccountParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	addr, err := address.NewAddress(p.Address)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -1, Message: fmt.Sprintf("internal error: address is invalid")}
	}
	darmaBytesAddres := addr.ToContractAddress()
	account := common.DarmaAddressToContractAddress(darmaBytesAddres)

	balance,err := chain.GetBalanceOfContractAccount(account)

	if err != nil {
		return nil, &jsonrpc.Error{Code: -1, Message: fmt.Sprintf("internal error: %s", err.Error())}
	}

	return structures.GetBalanceOfContractAccountResult{
		fmt.Sprintf("%d", balance.Uint64()),
	}, nil
}
