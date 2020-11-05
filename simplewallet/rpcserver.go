// Copyright 2018-2020 Darma Project. All rights reserved.

// the rpc server is an extension of walletapi and doesnot employ any global variables
// so a number can be simultaneously active ( based on resources)
package simplewallet

import (
	"github.com/darmaproject/darmasuite/walletapi"
	"io"
	"net/http/pprof"
)

//import "fmt"
import "time"
import "sync"
import "log"
import "strings"
import "net/http"

//import "github.com/intel-go/fastjson"
import "github.com/osamingo/jsonrpc"

import "github.com/darmaproject/darmasuite/globals"
import "github.com/darmaproject/darmasuite/structures"

const ENABLE_PPROF = false

// all components requiring access to wallet must use , this struct to communicate
// this structure must be update while mutex
type RPCServer struct {
	address          string
	srv              *http.Server
	mux              *http.ServeMux
	mr               *jsonrpc.MethodRepository
	Exit_Event       chan bool // wallet is shutting down and we must quit ASAP
	Exit_In_Progress bool

	w *walletapi.Wallet // reference to the wallet which is open
	sync.RWMutex
}

func RPCServer_Start(w *walletapi.Wallet, address string) (*RPCServer, error) {

	//var err error
	var r RPCServer

	//_ = err

	r.Exit_Event = make(chan bool)
	r.w = w
	r.address = address

	go r.Run()
	//logger.Infof("RPC server started")

	return &r, nil
}

// shutdown the rpc server component
func (r *RPCServer) RPCServer_Stop() {
	r.srv.Shutdown(nil) // shutdown the server
	r.Exit_In_Progress = true
	close(r.Exit_Event) // send signal to all connections to exit
	// TODO we  must wait for connections to kill themselves
	time.Sleep(1 * time.Second)
	//logger.Infof("RPC Shutdown")

}

func (r *RPCServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	basic_auth_enabled := false
	var parts []string

	if globals.Arguments["--rpc-login"] != nil {
		userpass := globals.Arguments["--rpc-login"].(string)
		parts = strings.SplitN(userpass, ":", 2)

		basic_auth_enabled = true
		/*if len(parts) != 2 { // these checks are done and verified during program init
		  globals.Logger.Warnf("RPC user name or password invalid")
		  return
		 }*/
		//log.Infof("RPC username \"%s\" password \"%s\" ", parts[0],parts[1])
	}

	if basic_auth_enabled {
		u, p, ok := req.BasicAuth()
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if u != parts[0] || p != parts[1] {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

	}
	// log.Printf("basic_auth_handler") serve if everything looks okay
	r.mr.ServeHTTP(w, req)
}

// setup handlers
func (r *RPCServer) Run() {

	mr := jsonrpc.NewMethodRepository()
	r.mr = mr

	// install getbalance handler
	if err := mr.RegisterMethod("getbalance", GetBalance_Handler{r: r}, structures.GetBalanceParams{}, structures.GetBalanceResult{}); err != nil {
		log.Fatalln(err)
	}

	// install getaddress handler
	if err := mr.RegisterMethod("getaddress", GetAddress_Handler{r: r}, structures.GetAddressParams{}, structures.GetBalanceResult{}); err != nil {
		log.Fatalln(err)
	}

	// install getversion handler
	if err := mr.RegisterMethod("getversion", GetVersion_Handler{r: r}, structures.GetVersionParams{}, structures.GetVersionResult{}); err != nil {
		log.Fatalln(err)
	}

	// install getheight handler
	if err := mr.RegisterMethod("getheight", GetHeight_Handler{r: r}, structures.GetHeightParams{}, structures.GetBalanceResult{}); err != nil {
		log.Fatalln(err)
	}

	// install transfer handler
	if err := mr.RegisterMethod("transfer", Transfer_Handler{r: r}, structures.TransferParams{}, structures.TransferResult{}); err != nil {
		log.Fatalln(err)
	}
	// install transfer_split handler
	if err := mr.RegisterMethod("transfer_split", TransferSplit_Handler{r: r}, structures.TransferSplitParams{}, structures.TransferSplitResult{}); err != nil {
		log.Fatalln(err)
	}

	// install get_bulk_payments handler
	if err := mr.RegisterMethod("get_bulk_payments", Get_Bulk_Payments_Handler{r: r}, structures.GetBulkPaymentsParams{}, structures.GetBulkPaymentsResult{}); err != nil {
		log.Fatalln(err)
	}

	// install query_key handler
	if err := mr.RegisterMethod("query_key", Query_Key_Handler{r: r}, structures.QueryKeyParams{}, structures.QueryKeyResult{}); err != nil {
		log.Fatalln(err)
	}

	// install make_integrated_address handler
	if err := mr.RegisterMethod("make_integrated_address", Make_Integrated_Address_Handler{r: r}, structures.MakeIntegratedAddressParams{}, structures.MakeIntegratedAddressResult{}); err != nil {
		log.Fatalln(err)
	}

	// install split_integrated_address handler
	if err := mr.RegisterMethod("split_integrated_address", Split_Integrated_Address_Handler{r: r}, structures.SplitIntegratedAddressParams{}, structures.SplitIntegratedAddressResult{}); err != nil {
		log.Fatalln(err)
	}

	// install get_transfer_by_txid handler
	if err := mr.RegisterMethod("get_transfer_by_txid", Get_Transfer_By_TXID_Handler{r: r}, structures.GetTransferByTxidParams{}, structures.GetTransferByTxidResult{}); err != nil {
		log.Fatalln(err)
	}

	// install get_transfers
	if err := mr.RegisterMethod("get_transfers", GetTransfersHandler{r: r}, structures.GetTransfersParams{}, structures.GetTransfersResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_token_transfers", GetTokenTransfersHandler{r: r}, structures.GetTokenTransfersParams{}, structures.GetTokenTransfersResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("register_stake_pool", RegisterStakePoolHandler{r: r}, structures.RegisterStakePoolParams{}, structures.RegisterStakePoolResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_stake_info", GetStakeInfoHandler{r: r}, structures.GetStakeInfoParams{}, structures.GetStakeInfoResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("close_stake_pool", CloseStakePoolHandler{r: r}, structures.CloseStakePoolParams{}, structures.CloseStakePoolResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("buy_share", BuyShareHandler{r: r}, structures.BuyShareParams{}, structures.BuyShareResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("repo_share", RepoShareHandler{r: r}, structures.RepoShareParams{}, structures.RepoShareResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("sign", SignHandler{r: r}, structures.SignParams{}, structures.SignParams{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("verify", VerifyHandler{r: r}, structures.VerifyParams{}, structures.VerifyResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("create_contract", ContractHandler{r: r, isCreate: true}, structures.CreateContractParams{}, structures.CreateContractResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("send_contract", ContractHandler{r: r, isCreate: false}, structures.CreateContractParams{}, structures.CreateContractResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("get_contract_address_by_txhash", GetContractHandler{r: r}, structures.GetContractAddressByTxHashParams{}, structures.GetContractAddressByTxHashResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("call_contract", CallContractHandler{r: r}, structures.WalletCallContractParams{}, structures.WalletCallContractResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("get_contract_result", GetContractResultHandler{r: r}, structures.GetContractResultParams{}, structures.GetContractResultResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("create_address", CreateSubaddressHandler{r: r}, structures.CreateAddressParams{}, structures.CreateAddressResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("list_address", ListSubaddressHandler{r: r}, structures.ListAddressParams{}, structures.ListAddressResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("validate_address", ValidateAddressHandler{r}, structures.ValidateAddressParams{}, structures.ValidateAddressResult{}); err != nil {
		log.Fatalln(err)
	}

	if err := mr.RegisterMethod("list_utxo", ListUtxoHandler{r}, structures.ListUtxoParams{}, structures.ListUtxoResult{}); err != nil {
		log.Fatalln(err)
	}

	// create a new mux
	r.mux = http.NewServeMux()
	r.srv = &http.Server{Addr: r.address, Handler: r.mux}

	r.mux.HandleFunc("/", hello)
	r.mux.Handle("/json_rpc", r)

	if ENABLE_PPROF {
		r.mux.HandleFunc("/debug/pprof/", pprof.Index)
		r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	if err := r.srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ERR listening to address err %s", err)
	}
}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}
