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
	"github.com/darmaproject/darmasuite/dvm/common/hexutil"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
)

//import	"log"
//import 	"net/http"

import "github.com/intel-go/fastjson"
import "github.com/osamingo/jsonrpc"

type EthWeb3JsRpcHandler_eth_sendRawTransaction struct{}

func (h EthWeb3JsRpcHandler_eth_sendRawTransaction) ServeJSONRPC(c context.Context, rawMessage *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	rlog.Debugf("eth_sendRawTransaction: rawMessage=%s",string(*rawMessage))

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

	var rawTxBytesString string
	if err := json.Unmarshal(params0bytes,&rawTxBytesString); err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("unmarshal raw-tx-bytes-string from bytes error: %s",err.Error())}
	}

	txBytes, err := hexutil.Decode(rawTxBytesString)
	if err != nil {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("decode raw tx bytes error: %s",err.Error())}
	}

	var tx transaction.Transaction

	//lets decode the tx from hex
	if len(txBytes) < 100 {
		return nil, &jsonrpc.Error{Message:"TX insufficient length"}
	}
	// lets add tx to pool, if we can do it, so  can every one else
	err = tx.DeserializeHeader(txBytes)
	if err != nil {
		return nil, &jsonrpc.Error{Message:err.Error()}
	}

	// lets try to add it to pool
	success := chain.AddTxToPool(&tx)
	if !success {
		return nil, &jsonrpc.Error{Message:fmt.Sprintf("Transaction %s rejected by daemon, check daemon msgs", tx.GetHash())}
	}

	return fmt.Sprintf("0x%s",tx.GetHash()),nil
}
