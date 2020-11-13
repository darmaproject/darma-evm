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
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"net/http"
	"net/http/pprof"

	"github.com/osamingo/jsonrpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/darmaproject/darmasuite/blockchain"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/metrics"
	"github.com/darmaproject/darmasuite/structures"
)

var DEBUG_MODE bool

/* this file implements the rpcserver api, so as wallet and block explorer tools can work without migration */

// all components requiring access to blockchain must use , this struct to communicate
// this structure must be update while mutex
type RPCServer struct {
	srv        *http.Server
	mux        *http.ServeMux
	ExitEvents chan bool // blockchain is shutting down and we must quit ASAP
	sync.RWMutex
}

var ExitInProgress bool
var chain *blockchain.Blockchain
var logger *log.Entry

func RpcServerStart(params map[string]interface{}) (*RPCServer, error) {
	var err error
	var r RPCServer

	_ = err

	r.ExitEvents = make(chan bool)

	logger = globals.Logger.WithFields(log.Fields{"source": "RPC"}) // all components must use this logger
	chain = params["chain"].(*blockchain.Blockchain)

	go r.Run()
	logger.Infof("RPC server started")
	atomic.AddUint32(&globals.SubsystemActive, 1) // increment subsystem

	return &r, nil
}

// shutdown the rpc server component
func (r *RPCServer) RpcServerStop() {
	r.Lock()
	defer r.Unlock()
	ExitInProgress = true
	close(r.ExitEvents) // send signal to all connections to exit

	if r.srv != nil {
		r.srv.Shutdown(context.Background()) // shutdown the server
	}
	// TODO we  must wait for connections to kill themselves
	time.Sleep(1 * time.Second)
	logger.Infof("RPC Shutdown")
	atomic.AddUint32(&globals.SubsystemActive, ^uint32(0)) // this decrement 1 fom subsystem
}

// setup handlers
func (r *RPCServer) Run() {
	mr := jsonrpc.NewMethodRepository()

	if err := mr.RegisterMethod("Main.Echo", EchoHandler{}, EchoParams{}, EchoResult{}); err != nil {
		log.Fatalln(err)
	}

	// install getblockcount handler
	if err := mr.RegisterMethod("getblockcount", GetBlockCountHandler{}, structures.GetBlockCountParams{}, structures.GetBlockCountResult{}); err != nil {
		log.Fatalln(err)
	}

	// install on_getblockhash
	if err := mr.RegisterMethod("on_getblockhash", GetBlockHashHandler{}, structures.OnGetBlockHashParams{}, structures.OnGetBlockHashResult{}); err != nil {
		log.Fatalln(err)
	}

	// install getblocktemplate handler
	if err := mr.RegisterMethod("getblocktemplate", GetBlockTemplateHandler{}, structures.GetBlockTemplateParams{}, structures.GetBlockTemplateResult{}); err != nil {
		log.Fatalln(err)
	}

	// submitblock handler
	if err := mr.RegisterMethod("submitblock", SubmitBlockHandler{}, structures.SubmitBlockParams{}, structures.SubmitBlockResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getlastblockheader", GetLastBlockHeaderHandler{}, structures.GetLastBlockHeaderParams{}, structures.GetLastBlockHeaderResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblockheaderbyhash", GetBlockHeaderByHashHandler{}, structures.GetBlockHeaderByHashParams{}, structures.GetBlockHeaderByHashResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("check_tx_key", ProveTxSpent_Handler{}, structures.ProveTxSpentParams{}, structures.ProveTxSpentResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblockheaderbyheight", GetBlockHeaderByHeightHandler{}, structures.GetBlockHeaderByHeightParams{}, structures.GetBlockHeaderByHeightResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblockheaderbytopoheight", GetBlockHeaderByTopoHeightHandler{}, structures.GetBlockHeaderByTopoHeightParams{}, structures.GetBlockHeaderByHeightResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblockshashbytopoheight", GetBlocksHashByTopoHeightHandler{}, structures.GetBlocksHashByTopoHeightParams{}, structures.GetBlocksHashByTopoHeightResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblock", GetBlockHandler{}, structures.GetBlockParams{}, structures.GetBlockResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblocklist", GetBlockListHandler{}, structures.GetBlockListParams{}, structures.GetBlockListResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getblockinfo", GetBlockInfoHandler{}, structures.GetBlockParams{}, structures.BlockInfo{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("gettxinfo", GetTxInfoHandler{}, structures.GetTransactionParams{}, structures.TxInfo{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("gettreegraph", GetTreeGraph_Handler{}, structures.GetTreeGraphParams{}, structures.GetTreeGraphResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("debug_stable", DebugStable_Handler{}, structures.DebugStableParams{}, structures.DebugStableResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_token_output", GetTokenOutputHandler{}, structures.GetTokenOutputParams{}, structures.GetTokenOutputResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getoutputs", GetOutputsHandler{}, structures.GetOutputsParams{}, structures.GetOutputsResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_info", GetInfoHandler{}, structures.GetInfoParams{}, structures.GetInfoResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_difficulty", GetDifficultyHandler{}, structures.GetDifficultyParams{}, structures.GetDifficultyResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("gettxpool", GetTxPoolHandler{}, structures.GetTxPoolParams{}, structures.GetTxPoolResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_poolsvotetats", GetPoolVoteStats_Handler{}, structures.GetPoolVoteStatsParams{}, structures.GetPoolVoteStatsResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_poolbonusstats", GetPoolBonusStats_Handler{}, structures.GetPoolBonusStatsParams{}, structures.GetPoolBonusStatsResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("getp2psession", GetP2pSessionHandler{}, structures.GetP2pSessionParams{}, structures.GetP2pSessionResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_stake_pool", GetStakePoolHandler{}, structures.GetStakePoolParams{}, structures.GetStakePoolResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_stake_info", GetStakeInfoHandler{}, structures.GetDaemonStakeInfoParams{}, structures.GetDaemonStakeInfoParams{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("list_stake_pool", ListStakePoolHandler{}, structures.ListStakePoolParams{}, structures.ListStakePoolResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("list_share", ListShareHandler{}, structures.ListShareParams{}, structures.ListShareResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_share", GetShareHandler{r: r}, structures.GetShareParams{}, structures.GetShareResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_contract_address_by_txhash", GetContractHandler{}, structures.GetContractAddressByTxHashParams{}, structures.GetContractAddressByTxHashResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("call_contract", CallContractHandler{}, structures.CallContractParams{}, structures.CallContractResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_contract_result", GetContractResultHandler{}, structures.GetContractResultParams{}, structures.GetContractResultResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_dot_graph", GetDotGraphHandler{}, structures.GetDotGraphParams{}, structures.GetDotGraphResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_balance_of_contract_account", GetBalanceOfContractAccountHandler{}, structures.GetBalanceOfContractAccountParams{}, structures.GetBalanceOfContractAccountResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_contract_account_address", GetContractAccountAddressHandler{}, nil, nil); err != nil {
		log.Fatalln(err)
	}

	// For support ETH Web3.js RPC

	if err := mr.RegisterMethod("eth_blockNumber", EthWeb3JsRpcHandler_eth_blockNumber{}, nil, nil); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("eth_sendRawTransaction", EthWeb3JsRpcHandler_eth_sendRawTransaction{}, nil, nil); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("eth_call", EthWeb3JsRpcHandler_eth_call{}, EthWeb3JsRpcParams_eth_call{}, nil); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("eth_getTransactionReceipt", EthWeb3JsRpcHandler_eth_getTransactionReceipt{}, nil, nil); err != nil {
		log.Fatalln(err)
	}

	// create a new mux
	r.mux = http.NewServeMux()

	defaultAddress := "127.0.0.1:" + fmt.Sprintf("%d", config.MainNet.RpcDefaultPort)
	if !globals.IsMainnet() {
		defaultAddress = "127.0.0.1:" + fmt.Sprintf("%d", config.TestNet.RpcDefaultPort)
	}

	if _, ok := globals.Arguments["--rpc-bind"]; ok && globals.Arguments["--rpc-bind"] != nil {
		addr, err := net.ResolveTCPAddr("tcp", globals.Arguments["--rpc-bind"].(string))
		if err != nil {
			logger.Warnf("--rpc-bind address is invalid, err = %s", err)
		} else {
			if addr.Port == 0 {
				logger.Infof("RPC server is disabled, No ports will be opened for RPC")
				return
			} else {
				defaultAddress = addr.String()
			}
		}
	}

	logger.Infof("RPC  will listen on %s", defaultAddress)
	r.Lock()
	r.srv = &http.Server{Addr: defaultAddress, Handler: r.mux}
	r.Unlock()

	r.mux.HandleFunc("/", hello)
	r.mux.Handle("/json_rpc", mr)

	// handle nasty http requests
	r.mux.HandleFunc("/getheight", getheight)
	r.mux.HandleFunc("/getoutputs.bin", getoutputs) // stream any outputs to server, can make wallet work offline
	r.mux.HandleFunc("/gettransactions", gettransactions)
	r.mux.HandleFunc("/sendrawtransaction", SendRawTransactionHandler)
	r.mux.HandleFunc("/is_key_image_spent", iskeyimagespent)
	r.mux.HandleFunc("/is_token_keyimage_spent", isTokenKeyImageSpent)
	r.mux.HandleFunc("/is_token_id_spent", isTokenIdSpent)

	if DEBUG_MODE {
		// Register pprof handlers individually if required
		r.mux.HandleFunc("/debug/pprof/", pprof.Index)
		r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// register metrics handler
		r.mux.HandleFunc("/metrics", prometheus.InstrumentHandler("darma", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))
	}

	//r.mux.HandleFunc("/json_rpc/debug", mr.ServeDebug)
	if err := r.srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Warnf("ERR listening to address err %s", err)
	}
}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}
