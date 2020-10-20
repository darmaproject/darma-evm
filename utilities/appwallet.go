package appwallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/dvm/accounts/abi"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/wavm"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/stake"
	"github.com/darmaproject/darmasuite/structures"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/darmaproject/darmasuite/walletapi"
	"github.com/romana/rlog"
	"github.com/toolkits/file"
	"github.com/ybbus/jsonrpc"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

//TODO: add checksum support
type WalletBackupInfo struct {
	Name      string `json:"name"`
	Ctime     int64  `json:"ctime"`
	Height    int64  `json:"Height"`
	Seeds     string `json:"seeds"`
	CheckSum  string `json:"checksum"`
	Signature string `json:"signature"`
}

type SeedsExportInfo struct {
	Seeds       string `json:"seeds"`
	StartHeight int64  `json:"start_height"`
}

type WalletStats struct {
	Unlocked_balance           uint64  `json:"unlocked_balance"`
	Locked_balance             uint64  `json:"locked_balance"`
	Total_balance              uint64  `json:"total_balance"`
	Wallet_height              uint64  `json:"wallet_height"`
	Daemon_height              uint64  `json:"daemon_height"`
	Wallet_topo_height         int64   `json:"wallet_topo_height"`
	Daemon_topo_height         uint64  `json:"daemon_topo_height"`
	Wallet_initial_height      int64   `json:"wallet_initial_height"`
	Wallet_available           bool    `json:"wallet_available"`
	Wallet_complete            bool    `json:"wallet_complete"`
	Wallet_online              bool    `json:"wallet_online"`
	Wallet_balance_changed     bool    `json:"wallet_balance_changed"`
	Wallet_mixin               int     `json:"wallet_mixin"`
	Wallet_fees_multiplier     float64 `json:"wallet_fees_multiplier"`
	Wallet_sync_time           int64   `json:"wallet_sync_time"`
	Wallet_minimum_topo_height int64   `json:"wallet_minimum_topo_height"`
}

type WalletKeys struct {
	Spendkey_Secret string
	Spendkey_Public string
	Viewkey_Secret  string
	Viewkey_Public  string
}

type MobileWalletAddress struct {
	IsValid             bool   `json:"isValid"`
	IsIntegratedAddress bool   `json:"isIntegratedAddress"`
	Address             string `json:"address"`
	PaymentID           string `json:"paymentID"`
}

type AppWalletTransferTX struct {
	Transfer_fee        uint64 `json:"fee"`
	Transfer_amount     uint64 `json:"amount"`
	Transfer_change     uint64 `json:"change"`
	Transfer_inputs_sum uint64 `json:"input_sum"`
	Transfer_txid       string `json:"txid"`
	Transfer_txhex      string `json:"transfer_txhex"`
	Transfer_address    string `json:"address"`
	Transfer_txsize     int    `json:"txsize"`
}

type AppWalletStats interface {
	DumpWalletStats(stats string)
}

type AppAutomaticTransferStates interface {
	DumpAutomaticTransferStats(stats string)
}

type MobileWallet struct {
	FileName       string
	Wallet         interface{}
	TimeZoneOffset int
	AppWalletStats

	IsSync   bool
	quitChan chan int
	wg       sync.WaitGroup

	automaticTS           AutomaticTransferState
	quitAutomaticTransfer chan int
	automaticWG           sync.WaitGroup
	AppAutomaticTransferStates
}

type AutomaticTransferState struct {
	IsAutomatic       bool   `json:"isautomatic"`
	TargetAmount      uint64 `json:"targetamount"`
	TotalAmount       uint64 `json:"totalamount"`
	TotalFees         uint64 `json:"totalfees"`
	Rate              uint64 `json:"rate"`
	TargetTransferNum int    `json:"targettransfernum"`
	TotalUTXONumber   uint64 `json:"totalutxonumber"`
	FinishUTXONumber  uint64 `json:"finishutxonumber"`
	TransferIndex     uint64 `json:"transferindex"`
	SuccessTransfer   uint64 `json:"successtransfer"`
	ErrorTransfer     uint64 `json:"errortransfer"`
}

type OutputStatistic struct {
	Number      uint64 `json:"number"`
	TotalAmount uint64 `json:"totalamount"`
	AboutFee    uint64 `json:"aboutfee"`
}

type FragmentUTXO struct {
	Num         int
	TotalAmount uint64
}

type AppWalletError struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
}

type AppWalletPingResult struct {
	Status     int   `json:"status"`
	Delay      int64 `json:"delay"`
	Height     int64 `json:"height"`
	TopoHeight int64 `json:"topo_height"`
	Score      int64 `json:"score"`
}

type ShareResult struct {
	structures.GetShareResult
	TotalNum uint32 `json:"total_num"` // == init_num, just for ios
	BuyTime  int64  `json:"buy_time"`
	PoolName string `json:"pool_name"`
}

type ListBuySharesResult struct {
	Transfer walletapi.Entry `json:"transfer"`
	ShareId  string          `json:"share_id"`
	Closed   bool            `json:"closed"`
	Amount   uint64          `json:"amount"`
}

type GetProfitWeeklyResult struct {
	Date   string `json:"date"`
	Amount uint64 `json:"amount"`
}

type StakeInfoResult struct {
	Profit          uint64 `json:"profit"`
	ShareNum        uint32 `json:"share_num"`
	LockAmount      uint64 `json:"lock_amount"`
	ProfitYestarday uint64 `json:"profit_yestarday"`
	ProfitRate      string `json:"profit_rate"`
}

var errApp AppWalletError
var chainType string

func removeTempWalletFile(filename string) bool {
	if file.IsExist(filename) == false {
		return true
	}

	err := os.Remove(filename)
	if err != nil {
		return false
	}
	return true
}

func setLastError(code int, message string, errArray ...interface{}) string {
	errApp.ErrCode = code
	if errApp.ErrCode == ErrSuccess {
		errApp.ErrMsg = "Success"
	} else {
		errApp.ErrMsg = message
	}
	if len(errArray) > 0 {
		err := errArray[0].(error)
		errApp.ErrMsg += err.Error()
	}

	return errApp.ErrMsg
}

func Discovery(endpoints string) string {
	return endpointDiscovery(endpoints)
}

func endpointDiscovery(endpoints string) string {
	addrlist := strings.Split(endpoints, ",")
	if len(addrlist) <= 1 {
		return endpoints
	}

	var wg sync.WaitGroup

	pingResult := make([]AppWalletPingResult, len(addrlist))
	for k, v := range addrlist {
		wg.Add(1)

		go func(endpoint string, result *AppWalletPingResult) {
			defer wg.Done()

			str := Ping_Daemon_Address(endpoint)
			if str != "" {
				json.Unmarshal([]byte(str), result)
			}
		}(v, &pingResult[k])
	}

	//TODO: wait all ping result
	wg.Wait()

	var fasterNode, slowerNode []int

	for k, v := range pingResult {
		if v.Status == 0 {
			continue
		}
		if v.Delay < 300 {
			fasterNode = append(fasterNode, k)
		} else {
			slowerNode = append(slowerNode, k)
		}
	}
	len1 := len(fasterNode)
	len2 := len(slowerNode)
	rand.Seed(time.Now().Unix())

	var i, candidate int
	if len1 > 0 {
		if len1 == 1 {
			i = 0
		} else {
			i = rand.Intn(len1)
		}
		candidate = fasterNode[i]
	} else if len2 > 0 {
		if len2 == 1 {
			i = 0
		} else {
			i = rand.Intn(len2)
		}
		candidate = slowerNode[i]
	} else {
		candidate = 0
	}

	rlog.Infof("Discovery endpoint '%s'\n", addrlist[candidate])
	return addrlist[candidate]
}

func Create_Encrypted_Wallet(filename string, password string) bool {
	wallet, err := walletapi.CreateEncryptedWalletRandom(filename, password)
	if err != nil {
		setLastError(ErrSystemInternal, "Create encrypted wallet failed: ", err)
		return false
	}

	wallet.Close_Encrypted_Wallet()
	rlog.Infof("Create encrypted wallet '%s' success.\n", filename)
	return true
}

func Recovery_Encrypted_Wallet(filename string, password string, electrum_seed string) bool {
	account, err := walletapi.GenerateAccountFromRecoveryWords(electrum_seed)
	if err != nil {
		setLastError(ErrSystemInternal, "Recovery encrypted wallet failed: ", err)
		return false
	}

	wallet, err := walletapi.CreateEncryptedWallet(filename, password, account.Keys.Spendkey_Secret)
	if err != nil {
		setLastError(ErrSystemInternal, "Error occurred while restoring wallet: ", err)
		return false
	}
	wallet.Close_Encrypted_Wallet()
	rlog.Infof("Recovery_Encrypted_Wallet success")

	return true
}

func Create_Encrypted_Wallet_ViewOnly(filename string, password string, viewkey string) bool {
	wallet, err := walletapi.Create_Encrypted_Wallet_ViewOnly(filename, password, viewkey)
	if err != nil {
		setLastError(ErrSystemInternal, "Error while reconstructing view only wallet using view key: ", err)
		return false
	}
	wallet.Close_Encrypted_Wallet()
	rlog.Infof("Create_Encrypted_Wallet_ViewOnly success")

	return true
}

func Verify_Amount(amount string) bool {
	_, err := globals.ParseAmount(amount, false)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount: ", err)
		return false
	}

	return true
}

func Format_Money(amountstr string) string {
	if amountstr == "" {
		setLastError(ErrInvalidAmount, "Invalid amount.")
		return "0.0"
	}

	amount, err := strconv.ParseUint(amountstr, 10, 64)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount.")
		return "0.0"
	}

	value := globals.FormatMoney(amount)
	if strings.Contains(value, ".") == false {
		return value
	}

	i := len(value) - 1
	for ; i >= 0; i-- {
		if value[i] != '0' || (i > 0 && value[i-1] == '.') {
			break
		}
	}

	i++
	if i != len(value) {
		value = value[0:i]
	}
	return value
}

func Get_Info_Request(endpoint string) (*structures.GetInfoResult, error) {
	var rpcClient jsonrpc.RPCClient
	opts := &jsonrpc.RPCClientOpts{
		HTTPClient:    &http.Client{Timeout: time.Duration(3 * time.Second)},
		CustomHeaders: nil,
	}

	if endpoint == "" {
		return nil, errors.New("Daemon address is not specified")
	}

	//^_^
	if strings.HasPrefix(endpoint, "http://") == false {
		endpoint = "http://" + endpoint
	}

	// create client
	rpcClient = jsonrpc.NewClientWithOpts(endpoint+"/json_rpc", opts)
	// execute rpc to service
	response, err := rpcClient.Call("get_info")
	if err != nil {
		return nil, errors.New("Connect Daemon address failed.")
	}

	var info structures.GetInfoResult
	err = response.GetObject(&info)
	if err != nil {
		return nil, errors.New("Decode getinfo response data failed.")
	}

	return &info, nil
}

func Ping_Daemon_Address(endpoint string) string {
	var result AppWalletPingResult

	if len(endpoint) > 0 {
		starttime := time.Now().UnixNano()
		info, err := Get_Info_Request(endpoint)
		if err != nil {
			rlog.Warnf("Ping daemon address '%s' failed: %s.", endpoint, err)
		} else {
			endtime := time.Now().UnixNano()
			result.Status = 1
			result.Height = info.Height
			result.TopoHeight = info.TopoHeight
			result.Delay = (endtime - starttime) / 1e6
			result.Score = 100 - result.Delay/10 + 5
			if result.Score > 90 {
				result.Score = 100
			} else if result.Score < 0 {
				result.Score = 0
			}
		}
	} else {
		rlog.Warnf("Daemon address is empty.")
	}

	buffer, _ := json.Marshal(result)
	return string(buffer)
}

func GetLastError() string {
	buffer, err := json.Marshal(errApp)
	if err != nil {
		return ""
	}
	return string(buffer)
}

func Import_Seeds(text, password string) string {
	if text == "" {
		rlog.Errorf("Text cannot be empty!")
		setLastError(ErrSystemInternal, "Text cannot be empty!")
		return ""
	}
	decrypted, err := decryptWallet(text, password)
	if err != nil {
		rlog.Errorf("failed decrypt %s, err %s", text, err)
		setLastError(ErrInvalidPassword, err.Error())
		return ""
	}

	var seeds SeedsExportInfo
	if err = json.Unmarshal([]byte(decrypted), &seeds); err != nil {
		setLastError(ErrDecodeData, "seed or password err")
		return ""
	}

	return decrypted
}

func Get_Wallet_Address(filename string, password string) string {
	w := NewMobileWallet()
	res := w.Open_Encrypted_Wallet(filename, password)
	if res == false {
		return ""
	}
	address := w.Get_Wallet_Address()
	w.Close_Encrypted_Wallet()

	return address
}

func Check_Backup_WalletFile(backupname string) string {
	buffer, err := ioutil.ReadFile(backupname)
	if err != nil {
		setLastError(ErrInvalidFileName, "Open wallet file failed: ", err)
		return ""
	}

	var binfo WalletBackupInfo
	json.Unmarshal(buffer, &binfo)

	checksum := binfo.CheckSum
	binfo.CheckSum = ""

	metadata := *(*[]byte)(unsafe.Pointer(&binfo))
	checksum2 := crypto.Keccak256(metadata).String()
	//fmt.Println("Checksum: ", checksum, " ", checksum2)
	if checksum != checksum2 {
		setLastError(ErrBadWalletFile, "Bad wallet file: invalid checksum.")
		return ""
	}

	return string(buffer)
}

func Restore_WalletFile(backupname string, pathname string, password string, start_height int64) bool {
	buffer, err := ioutil.ReadFile(backupname)
	if err != nil {
		setLastError(ErrSystemInternal, "Restore wallet failed: ", err)
		return false
	}

	var binfo WalletBackupInfo
	json.Unmarshal(buffer, &binfo)

	seeds, err := decryptWallet(binfo.Seeds, password)
	sig := crypto.Keccak256([]byte(seeds)).String()

	if sig != binfo.Signature {
		setLastError(ErrBadWalletFile, "Restore wallet failed: Invalid password or wallet file is corrupted.")
		return false
	}

	//fmt.Println(seeds)
	if err != nil || seeds == "" {
		setLastError(ErrInvalidPassword, "Restore wallet failed: invalid password.")
		return false
	}
	//fmt.Println(seeds)
	walletName := pathname

	if file.IsExist(walletName) == true {
		setLastError(ErrExist, "Restore wallet failed: wallet file already exists.", errors.New(pathname))
		return false
	}

	w := NewMobileWallet()
	if w.Recovery_Encrypted_Wallet(walletName, password, seeds) == false {
		return false
	}

	height := binfo.Height
	if start_height > 0 {
		height = start_height
	}
	w.Set_Initial_Height(height)

	w.Close_Encrypted_Wallet()

	return true
}

func (w *MobileWallet) GetWallet() *walletapi.Wallet {
	if w.Wallet == nil {
		return nil
	}

	return (w.Wallet).(*walletapi.Wallet)
}

func (w *MobileWallet) buildDaemonAddress(endpoint string) string {
	prefix := "http://"

	if strings.IndexAny(endpoint, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") >= 0 { // url is already complete
		if strings.HasPrefix(endpoint, prefix) == false {
			//When the address is a domain name, buildurl needs 'http://'
			return prefix + endpoint
		}

	}
	return endpoint
}

func (w *MobileWallet) askPasswordForTransfer(wallet *walletapi.Wallet, amount uint64) bool {
	if amount >= uint64(1000*config.COIN_UNIT) {
		return true
	}

	if wallet.Get_Height() < wallet.Get_Daemon_Height() {
		rlog.Warnf("Wallet is out of sync")
		return false
	}

	var transfers []walletapi.Entry
	data := w.Get_Transfers(false, true, "", "")
	json.Unmarshal([]byte(data), &transfers)

	var sum uint64
	for k, v := range transfers {
		if k >= 3 {
			return false
		}

		sum += v.Amount
		if sum > uint64(10000*config.COIN_UNIT) {
			return true
		}
	}

	return false
}

func (w *MobileWallet) Create_Encrypted_Wallet(filename string, password string) bool {
	wallet, err := walletapi.CreateEncryptedWalletRandom(filename, password)
	if err != nil {
		setLastError(ErrSystemInternal, "Create encrypted wallet failed: ", err)
		return false
	}

	w.Wallet = wallet
	w.FileName = filename
	w.Set_Lightweight_Mode(true)

	rlog.Infof("Create encrypted wallet '%s' success.\n", filename)
	return true
}

func (w *MobileWallet) Recovery_Encrypted_Wallet(filename string, password string, electrum_seed string) bool {
	if file.IsExist(filename) == true {
		setLastError(ErrExist, "Recovery wallet failed: wallet file already exists.", errors.New(filename))
		return false
	}

	account, err := walletapi.GenerateAccountFromRecoveryWords(electrum_seed)
	if err != nil {
		setLastError(ErrSystemInternal, "Recovery encrypted wallet failed: ", err)
		return false
	}

	wallet, err := walletapi.CreateEncryptedWallet(filename, password, account.Keys.Spendkey_Secret)
	if err != nil {
		removeTempWalletFile(filename)

		setLastError(ErrSystemInternal, "Error occurred while restoring wallet: ", err)
		return false
	}

	w.Wallet = wallet
	w.FileName = filename
	w.Set_Lightweight_Mode(true)

	rlog.Infof("Recovery_Encrypted_Wallet success")

	return true
}

func (w *MobileWallet) Create_Encrypted_Wallet_ViewOnly(filename string, password string, viewkey string) bool {
	if file.IsExist(filename) == true {
		setLastError(ErrExist, "Create wallet failed: wallet file already exists.", errors.New(filename))
		return false
	}

	wallet, err := walletapi.Create_Encrypted_Wallet_ViewOnly(filename, password, viewkey)
	if err != nil {
		removeTempWalletFile(filename)

		setLastError(ErrSystemInternal, "Error while reconstructing view only wallet using view key: ", err)
		return false
	}

	w.Wallet = wallet
	w.FileName = filename
	w.Set_Lightweight_Mode(true)

	rlog.Infof("Create_Encrypted_Wallet_ViewOnly success")

	return true
}

func (w *MobileWallet) Create_Encrypted_Wallet_SpendKey(filename string, password string, spendkey string) bool {
	var seedkey crypto.Key

	if file.IsExist(filename) == true {
		setLastError(ErrExist, "Create wallet failed: wallet file already exists.", errors.New(filename))
		return false
	}

	seed_raw, err := hex.DecodeString(spendkey) // hex decode
	if len(spendkey) != 64 || err != nil {      //sanity check
		setLastError(ErrInvalidKeys, "Seed must be 64 chars hexadecimal chars")
		return false
	}

	copy(seedkey[:], seed_raw[:32])

	wallet, err := walletapi.CreateEncryptedWallet(filename, password, seedkey)
	if err != nil {
		removeTempWalletFile(filename)

		setLastError(ErrSystemInternal, "Error while recovering wallet using seed key err %s\n", err)
		return false
	}

	w.Wallet = wallet
	w.FileName = filename
	w.Set_Lightweight_Mode(true)

	rlog.Infof("Successfully recovered wallet from hex seed")

	return true
}

func (w *MobileWallet) Open_Encrypted_Wallet(filename string, password string) bool {
	var wallet *walletapi.Wallet

	wallet = w.GetWallet()
	if wallet != nil {
		setLastError(ErrWalletBusy, "Wallet have alrealy opened.")
		return false
	}

	wallet, err := walletapi.OpenEncryptedWallet(filename, password)
	if err != nil {
		setLastError(ErrSystemInternal, "Open encrypted wallet failed: ", err)
		return false
	}

	w.Wallet = wallet
	w.FileName = filename
	w.Set_Lightweight_Mode(true)

	rlog.Infof("Open_Encrypted_Wallet success")
	return true
}

func (w *MobileWallet) Close_Encrypted_Wallet() bool {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Open encrypted wallet failed: ")
		return false
	}
	w.Stop_Automatic_Transfer()
	wallet.Close_Encrypted_Wallet()

	w.Wallet = nil
	w.AppWalletStats = nil
	w.IsSync = false

	rlog.Debugf("Close_Encrypted_Wallet success")
	return true
}

func (w *MobileWallet) Is_View_Only() bool {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Invalid wallet object.")
		return false
	}

	return wallet.Is_View_Only()
}

func (w *MobileWallet) Export_Seeds(password string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	data, _ := json.Marshal(SeedsExportInfo{
		Seeds:       wallet.GetSeed(),
		StartHeight: wallet.GetInitialHeight(),
	})
	encrypted, _ := encryptWallet(string(data), password)
	return encrypted
}

func (w *MobileWallet) Get_Seeds(password string) string {
	wallet := w.GetWallet()

	if wallet.Check_Password(password) == false {
		setLastError(ErrPaswordMisMatch, "Wallet password mismatch.")
		return ""
	}

	return wallet.GetSeed()
}

func (w *MobileWallet) Get_Keys(password string) string {
	var wkeys WalletKeys

	wallet := w.GetWallet()
	keys := wallet.Get_Keys()

	if wallet.Check_Password(password) == false {
		setLastError(ErrPaswordMisMatch, "Wallet password mismatch.")
		return ""
	}

	wkeys.Spendkey_Secret = keys.Spendkey_Secret.String()
	wkeys.Spendkey_Public = keys.Spendkey_Public.String()
	wkeys.Viewkey_Secret = keys.Viewkey_Secret.String()
	wkeys.Viewkey_Public = keys.Viewkey_Public.String()

	buffer, err := json.Marshal(wkeys)
	if err != nil {
		setLastError(ErrEncodeData, "Data encode failed:", err)
		return ""
	}

	return string(buffer)
}

func (w *MobileWallet) Get_Wallet_Address() string {
	wallet := w.GetWallet()

	addr := wallet.GetAddress()
	rlog.Debugf("Get_Wallet_address addr %s", addr.String())

	return addr.String()
}

func (w *MobileWallet) Generate_Intergrated_Address(payment_id_len int) string {
	var address, payment_id string

	wallet := w.GetWallet()
	switch payment_id_len {
	case 8:
		addr := wallet.GetRandomIAddress8()
		address = addr.String()
		payment_id = hex.EncodeToString(addr.PaymentID)
	case 32:
		addr := wallet.GetRandomIAddress32()
		address = addr.String()
		payment_id = hex.EncodeToString(addr.PaymentID)
	default:
		addr := wallet.GetRandomIAddress8()
		address = addr.String() // default return 8 byte encrypted payment ids
		payment_id = hex.EncodeToString(addr.PaymentID)
	}

	rlog.Debugf("Get_Wallet_address addr %s %s", address, payment_id)

	return address
}

func (w *MobileWallet) Get_Transfers(in bool, out bool, max_height_str, limit_str string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	pool := false
	maxHeight, _ := strconv.ParseUint(max_height_str, 10, 64)
	if maxHeight == 0 {
		pool = true
		maxHeight = wallet.Get_Height()
	} else {
		maxHeight -= 1 // exclusive
	}

	limit, _ := strconv.Atoi(limit_str)
	if limit <= 0 {
		limit = 10
	}
	entries := wallet.ShowMergedTransfers(in, out, true, pool, 0, maxHeight, limit)
	buffer, _ := json.Marshal(entries)
	return string(buffer)
}

func (w *MobileWallet) Get_Transfer(tx_id string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}
	tx, err := wallet.GetPendingTransferByTXID(crypto.HashHexToHash(tx_id))
	if err != nil {
		tx, err = wallet.GetTransferByTXID(crypto.HashHexToHash(tx_id))
		if err != nil {
			setLastError(ErrInvalidTxid, err.Error())
			return ""
		}
	}

	if err = wallet.FixEntry(&tx); err != nil {
		setLastError(ErrSystemInternal, err.Error())
		return ""
	}

	buffer, _ := json.Marshal(&tx)
	return string(buffer)
}

func (w *MobileWallet) Get_TotalOutput(limit_str string, limitType string, amount_str string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}
	fees_per_kb := uint64(0)
	if wallet.Get_Height() < uint64(globals.GetVotingStartHeight()) {
		fees_per_kb = config.BEFORE_DPOS_FEE_PER_KB
	} else {
		fees_per_kb = config.FEE_PER_KB
	}

	var limit uint64
	var err error
	if limit_str == "0" {
		limit = 0
	} else {
		limit, err = globals.ParseAmount(limit_str, false)
		if err != nil {
			setLastError(ErrInvalidAmount, "Invalid amount: ", err)
			return ""
		}
	}
	var amount uint64
	if amount_str == "0" {
		banlance, _ := wallet.GetBalance()
		amount = banlance
	} else {
		amount, err = globals.ParseAmount(amount_str, false)
		if err != nil {
			setLastError(ErrInvalidAmount, "Invalid amount: ", err)
			return ""
		}
	}

	selectedOutputIndex, sum := wallet.TotalOutput(limit, limitType, amount)
	var result OutputStatistic
	result.Number = uint64(len(selectedOutputIndex))
	if sum > amount {
		result.TotalAmount = amount
	} else {
		result.TotalAmount = sum
	}
	result.AboutFee = result.Number * 44 / 100 * fees_per_kb

	buffer, _ := json.Marshal(result)
	return string(buffer)
}

func (w *MobileWallet) Get_Automatic_Status() string {
	buffer, _ := json.Marshal(w.automaticTS)

	return string(buffer)
}

func (w *MobileWallet) Set_DumpAutomaticStats_Callback(s AppAutomaticTransferStates) {
	w.AppAutomaticTransferStates = s
}

func (w *MobileWallet) Cancel_DumpAutomaticStats_Callback() {
	w.AppAutomaticTransferStates = nil
}

func (w *MobileWallet) Stop_Automatic_Transfer() {
	if w.automaticTS.IsAutomatic == false {
		return
	}
	w.quitAutomaticTransfer <- 1
	w.automaticWG.Wait()

	w.Cancel_DumpAutomaticStats_Callback()
}

func (w *MobileWallet) Start_Automatic_Transfer(toaddr string, amountstr string, unlock_time_str string, payment_id string, mixin int, password string, limit_str string) bool {
	if w.automaticTS.IsAutomatic {
		setLastError(ErrInvalidWalletObject, "Automatic transfer is running.")
		return false
	}

	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}

	_, err := globals.ParseValidateAddress(toaddr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address: ", err)
		return false
	}

	var limit uint64
	if limit_str == "0" {
		limit = 0
	} else {
		limit, err = globals.ParseAmount(limit_str, false)
		if err != nil {
			setLastError(ErrInvalidAmount, "Invalid amount: ", err)
			return false
		}
	}
	var amount uint64
	if amountstr == "0" {
		banlance, _ := wallet.GetBalance()
		amount = banlance
	} else {
		amount, err = globals.ParseAmount(amountstr, false)
		if err != nil {
			setLastError(ErrInvalidAmount, "Invalid amount: ", err)
			return false
		}
	}

	go w.dumpAutomaticStats()
	go w.automatic_Transfer(toaddr, amount, unlock_time_str, payment_id, mixin, password, limit)
	w.automaticWG.Add(1)
	return true
}

func (w *MobileWallet) dumpAutomaticStats() {
	ticker := time.NewTicker(time.Duration(3) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			buffer, err := json.Marshal(w.automaticTS)
			if err != nil {
				continue
			}
			if w.AppAutomaticTransferStates != nil {
				w.AppAutomaticTransferStates.DumpAutomaticTransferStats(string(buffer))
			} else {
				return
			}
		}
	}
}

func (w *MobileWallet) automatic_Transfer(toaddr string, amount uint64, unlock_time_str string, payment_id string, mixin int, password string, limit uint64) {
	defer w.automaticWG.Done()

	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return
	}

	selectedOutputIndex, _ := wallet.TotalOutput(limit, "max", amount)
	w.automaticTS.IsAutomatic = true
	w.automaticTS.TargetAmount = amount
	w.automaticTS.TotalAmount = 0
	w.automaticTS.TotalFees = 0
	w.automaticTS.Rate = 0
	w.automaticTS.FinishUTXONumber = 0
	w.automaticTS.TotalUTXONumber = uint64(len(selectedOutputIndex))
	w.automaticTS.TransferIndex = 1
	w.automaticTS.SuccessTransfer = 0
	w.automaticTS.ErrorTransfer = 0
	w.automaticTS.TargetTransferNum = int(w.automaticTS.TotalUTXONumber)/config.MAX_INPUT_SIZE + 1
	for {
		select {
		case <-w.quitAutomaticTransfer:
			w.automaticTS.IsAutomatic = false
			return
		default:
			if wallet != nil {
				tx := w.TransferV2(toaddr, globals.FormatMoney8(amount), "0", payment_id, 0, true, password, limit, "max")
				if tx == "" {
					w.automaticTS.ErrorTransfer++
					time.Sleep(time.Second * 3)
				} else {
					var txResult AppWalletTransferTX
					json.Unmarshal([]byte(tx), &txResult)
					w.automaticTS.TotalAmount += txResult.Transfer_amount
					w.automaticTS.TotalFees += txResult.Transfer_fee
					w.automaticTS.FinishUTXONumber += config.MAX_INPUT_SIZE
					w.automaticTS.Rate = w.automaticTS.FinishUTXONumber * 100 / w.automaticTS.TotalUTXONumber
					w.automaticTS.SuccessTransfer++
					if w.automaticTS.FinishUTXONumber > w.automaticTS.TotalUTXONumber {
						w.automaticTS.FinishUTXONumber = w.automaticTS.TotalUTXONumber
						w.automaticTS.Rate = 100
						w.automaticTS.IsAutomatic = false
						return
					}
					if amount > txResult.Transfer_amount+txResult.Transfer_fee+txResult.Transfer_change {
						amount = amount - (txResult.Transfer_amount + txResult.Transfer_fee + txResult.Transfer_change)
					} else {
						w.automaticTS.Rate = 100
						w.automaticTS.IsAutomatic = false
						return
					}
					if w.automaticTS.TotalAmount == w.automaticTS.TargetAmount {
						w.automaticTS.IsAutomatic = false
						return
					}
				}
				w.automaticTS.TransferIndex++
			}
		}
	}
}

func (w *MobileWallet) Transfer(toaddr string, amountstr string, unlock_time_str string, payment_id string, mixin int, sendtx bool, password string) string {
	return w.TransferV2(toaddr, amountstr, unlock_time_str, payment_id, mixin, sendtx, password, 0, "")
}

func (w *MobileWallet) TransferV2(toaddr string, amountstr string, unlock_time_str string, payment_id string, mixin int, sendtx bool, password string, limit uint64, limitType string) string {
	var addr_list []address.Address
	var amount_list []uint64

	wallet := w.GetWallet()

	unlock_time, _ := strconv.ParseUint(unlock_time_str, 10, 64)

	addr, err := globals.ParseValidateAddress(toaddr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address: ", err)
		return ""
	}
	amount, err := globals.ParseAmount(amountstr, false)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount: ", err)
		return ""
	}

	addr_list = append(addr_list, *addr)
	amount_list = append(amount_list, amount)

	if w.askPasswordForTransfer(wallet, amount) == true {
		if w.Check_Password(password) == false {
			setLastError(ErrInvalidPassword, "Invalid password")
			return ""
		}
	}

	if len(payment_id) > 0 { // parse payment_id
		if len(payment_id) == 64 || len(payment_id) == 16 {
			_, err := hex.DecodeString(payment_id)
			if err != nil {
				setLastError(ErrInvalidPaymentID, "Error parsing payment ID, it should be in hex 16 or 64 chars: ", err)
				return ""
			}
		} else {
			setLastError(ErrInvalidPaymentID, "Invalid payment ID: ")
			return ""
		}
	}

	if addr.IsIntegratedAddress() {
		if len(payment_id) > 0 {
			setLastError(ErrInvalidPaymentID, "Payment ID provided in both integrated address and separatel: ")
			return ""
		}

		rlog.Infof("Payment ID is integreted in address ID:%x", addr.PaymentID)
	}

	tx, inputs, input_sum, change, err := wallet.Transfer(addr_list, amount_list, unlock_time, payment_id, 0, 0, limit, limitType)

	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("input_sum = %d change = %d\n", input_sum, change)
	var txResult AppWalletTransferTX
	Transfer_amount := amount
	fees := tx.RctSignature.GetTXFee()
	//fmt.Printf("--------------------->input_sum = %d change = %d  fee = %d\n",input_sum,change,fees)
	if input_sum < amount+fees {
		Transfer_amount = input_sum - fees - change
	}
	txResult.Transfer_fee = fees
	txResult.Transfer_amount = Transfer_amount
	txResult.Transfer_change = change
	txResult.Transfer_inputs_sum = input_sum
	txResult.Transfer_txid = tx.GetHash().String()
	txResult.Transfer_txhex = hex.EncodeToString(tx.Serialize())
	txResult.Transfer_address = toaddr
	txResult.Transfer_txsize = len([]byte(txResult.Transfer_txhex))/1024/2 + 1

	//fmt.Println("txsize:",txResult.Transfer_txsize)
	//fmt.Println("--------------------->txamount:",Transfer_amount)
	if sendtx == true {
		err = wallet.SendTransaction(tx) // relay tx to daemon/network
		if err != nil {
			setLastError(ErrSystemInternal, "Send transfer failed: ", err)
			return ""
		}
	}

	rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())

	buffer, err := json.Marshal(txResult)

	return string(buffer)
}

func (w *MobileWallet) Transfer_Everything(address string, unlock_time_str string, payment_id_hex string, mixin int, sendtx bool, password string) string {
	if w.Check_Password(password) == false {
		return ""
	}

	var wallet *walletapi.Wallet

	wallet = w.GetWallet()

	unlock_time, _ := strconv.ParseUint(unlock_time_str, 10, 64)

	addr, err := globals.ParseValidateAddress(address)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address for TransferEverything: ", err)
		return ""
	}
	fees_per_kb := uint64(0) // fees  must be calculated by walletapi

	tx, inputs, input_sum, err := wallet.TransferEverything(*addr, "", unlock_time, fees_per_kb, 5)

	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Build transfer failed: ", err)
		return ""
	}

	rlog.Infof("input_sum = %d\n", input_sum)

	var txResult AppWalletTransferTX

	txResult.Transfer_fee = tx.RctSignature.GetTXFee()
	txResult.Transfer_amount = input_sum - txResult.Transfer_fee
	txResult.Transfer_change = 0
	txResult.Transfer_inputs_sum = input_sum
	txResult.Transfer_txid = tx.GetHash().String()
	txResult.Transfer_txhex = hex.EncodeToString(tx.Serialize())
	txResult.Transfer_address = address
	txResult.Transfer_txsize = len([]byte(txResult.Transfer_txhex))/1024 + 1

	if sendtx == true {
		err = wallet.SendTransaction(tx) // relay tx to daemon/network
		if err != nil {
			setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
			return ""
		}
	}
	rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())

	buffer, err := json.Marshal(txResult)

	return string(buffer)
}

func (w *MobileWallet) Transfer_Locked(toaddr string, amountstr string, payment_id string, mixin int, sendtx bool) string {
	wallet := w.GetWallet()

	addr, err := globals.ParseValidateAddress(toaddr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address: ", err)
		return ""
	}
	amount, err := globals.ParseAmount(amountstr, false)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount: ", err)
		return ""
	}

	if len(payment_id) > 0 { // parse payment_id
		if len(payment_id) == 64 || len(payment_id) == 16 {
			_, err := hex.DecodeString(payment_id)
			if err != nil {
				setLastError(ErrInvalidPaymentID, "Error parsing payment ID, it should be in hex 16 or 64 chars: ", err)
				return ""
			}
		} else {
			setLastError(ErrInvalidPaymentID, "Invalid payment ID: ")
			return ""
		}
	}

	if addr.IsIntegratedAddress() {
		if len(payment_id) > 0 {
			setLastError(ErrInvalidPaymentID, "Payment ID provided in both integrated address and separatel: ")
			return ""
		}

		rlog.Infof("Payment ID is integreted in address ID:%x", addr.PaymentID)
	}

	tx_extra := &transaction.TxCreateExtra{
		LockedType: transaction.LOCKEDTYPE_LOCKED,
	}

	tx, inputs, input_sum, change, err := wallet.TransferLocked(*addr, amount, payment_id, 0, 0, 0, tx_extra, 0, "")

	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("input_sum = %d change = %d\n", input_sum, change)

	var txResult AppWalletTransferTX

	txResult.Transfer_fee = tx.RctSignature.GetTXFee()
	txResult.Transfer_amount = amount
	txResult.Transfer_change = change
	txResult.Transfer_inputs_sum = input_sum
	txResult.Transfer_txid = tx.GetHash().String()
	txResult.Transfer_txhex = hex.EncodeToString(tx.Serialize())
	txResult.Transfer_address = toaddr
	txResult.Transfer_txsize = len([]byte(txResult.Transfer_txhex))/1024 + 1

	if sendtx == true {
		err = wallet.SendTransaction(tx) // relay tx to daemon/network
		if err != nil {
			setLastError(ErrSystemInternal, "Send transfer failed: ", err)
			return ""
		}
	}

	rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())

	buffer, err := json.Marshal(txResult)

	return string(buffer)
}

func (w *MobileWallet) Transfer_UnLocked(toaddr string, txid_str string, payment_id string, mixin int, sendtx bool) string {
	wallet := w.GetWallet()

	addr, err := globals.ParseValidateAddress(toaddr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address: ", err)
		return ""
	}

	if len(payment_id) > 0 { // parse payment_id
		if len(payment_id) == 64 || len(payment_id) == 16 {
			_, err := hex.DecodeString(payment_id)
			if err != nil {
				setLastError(ErrInvalidPaymentID, "Error parsing payment ID, it should be in hex 16 or 64 chars: ", err)
				return ""
			}
		} else {
			setLastError(ErrInvalidPaymentID, "Invalid payment ID: ")
			return ""
		}
	}

	if addr.IsIntegratedAddress() {
		if len(payment_id) > 0 {
			setLastError(ErrInvalidPaymentID, "Payment ID provided in both integrated address and separatel: ")
			return ""
		}

		rlog.Infof("Payment ID is integreted in address ID:%x", addr.PaymentID)
	}

	txid := crypto.HashHexToHash(txid_str)
	if txid == crypto.ZeroHash {
		setLastError(ErrInvalidTxid, "Invalid Txid")
		return ""
	}

	tx_extra := &transaction.TxCreateExtra{
		LockedType: transaction.LOCKEDTYPE_UNLOCKED,
		LockedTxId: txid,
	}

	tx, inputs, input_sum, change, err := wallet.TransferLocked(*addr, 0, payment_id, 0, 0, 0, tx_extra, 0, "")

	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("input_sum = %d change = %d\n", input_sum, change)

	var txResult AppWalletTransferTX

	txResult.Transfer_fee = tx.RctSignature.GetTXFee()
	txResult.Transfer_amount = input_sum - txResult.Transfer_fee
	txResult.Transfer_change = change
	txResult.Transfer_inputs_sum = input_sum
	txResult.Transfer_txid = tx.GetHash().String()
	txResult.Transfer_txhex = hex.EncodeToString(tx.Serialize())
	txResult.Transfer_address = toaddr
	txResult.Transfer_txsize = len([]byte(txResult.Transfer_txhex))/1024 + 1

	if sendtx == true {
		err = wallet.SendTransaction(tx) // relay tx to daemon/network
		if err != nil {
			setLastError(ErrSystemInternal, "Send transfer failed: ", err)
			return ""
		}
	}

	rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())

	buffer, err := json.Marshal(txResult)

	return string(buffer)
}

func (w *MobileWallet) Send_Raw_Transaction(txraw string) string {

	wallet := w.GetWallet()

	hex_tx := strings.TrimSpace(txraw)
	tx_bytes, err := hex.DecodeString(hex_tx)
	if err != nil {
		setLastError(ErrDecodeData, "Transaction Could NOT be hex decoded: ", err)
		return ""
	}

	var tx transaction.Transaction

	err = tx.DeserializeHeader(tx_bytes)
	if err != nil {
		setLastError(ErrDecodeData, "Transaction Could NOT be deserialized: ", err)
		return ""
	}

	err = wallet.SendTransaction(&tx) // relay tx to daemon/network
	if err != nil {
		setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
		return ""
	}
	rlog.Infof("Send raw transaction successfully. txid = %s", tx.GetHash())

	return tx.GetHash().String()
}

func (w *MobileWallet) Rescan_From_Height() {
	wallet := w.GetWallet()

	if wallet.GetMode() { // trigger rescan we the wallet is online
		wallet.StopSync()
		wallet.Clean() // clean existing data from wallet

		wallet.Rescan_From_Height(wallet.GetInitialHeight())
	}
}

func (w *MobileWallet) Rescan_Force() bool {
	wallet := w.GetWallet()

	if wallet.GetMode() { // trigger rescan we the wallet is online
		res := w.Stop_Update_Blance()
		if res == false {
			return false
		}
	}

	time.Sleep(3 * time.Second)

	wallet.Clean() // clean existing data from wallet
	wallet.Rescan_From_Height(0)

	rlog.Info("Reset wallet completed, restart update.")

	res := w.Update_Wallet_Balance()
	if res == false {
		return false
	}
	return true
}

func (w *MobileWallet) Set_Online_Mode() bool {
	wallet := w.GetWallet()

	return wallet.SetOnlineMode()
}

func (w *MobileWallet) Set_Offline_Mode() bool {
	wallet := w.GetWallet()

	return wallet.SetOfflineMode()
}

func (w *MobileWallet) Change_Password(password string) bool {
	if len(password) == 0 {
		setLastError(ErrInvalidPassword, "Password is empty. ")
		return false
	}

	wallet := w.GetWallet()

	err := wallet.Set_Encrypted_Wallet_Password(password)
	if err != nil {
		setLastError(ErrSystemInternal, "Change password failed: ", err)
		return false
	}

	return true
}

func (w *MobileWallet) Set_Initial_Height(startheight int64) bool {
	wallet := w.GetWallet()

	wallet.SetInitialHeight(startheight)
	return true
}

func (w *MobileWallet) Set_Initial_Height_Default() int64 {
	wallet := w.GetWallet()

	endpoint := wallet.DaemonEndpoint
	if endpoint == "" {
		setLastError(ErrDaemonIsEmpty, "Daemon address is not specified")
		return 0
	}

	var startheight int64
	info, err := Get_Info_Request(endpoint)
	if err != nil {
		startheight = 0
	} else {
		if info.TopoHeight <= 10 {
			startheight = 0
		} else {
			startheight = info.TopoHeight - 10
		}
	}

	rlog.Infof("Set default initial height is %d.\n ", startheight)
	wallet.SetInitialHeight(startheight)
	return startheight
}

func (w *MobileWallet) Set_Mixin(mixin int) int {

	if mixin <= 0 {
		rlog.Warnf("Invalid minin value: %d.\n", mixin)
		return mixin
	}

	wallet := w.GetWallet()

	return wallet.SetMixin(mixin)
}

func (w *MobileWallet) Get_Mixin() int {
	wallet := w.GetWallet()

	return wallet.GetMixin()
}

func (w *MobileWallet) Set_Fee_Multiplier(x float32) float32 {
	wallet := w.GetWallet()
	return wallet.SetFeeMultiplier(x)
}

func (w *MobileWallet) Get_Fee_Multiplier() float32 {
	wallet := w.GetWallet()
	return wallet.GetFeeMultiplier()
}

func (w *MobileWallet) Set_Delay_Sync(delay int64) int64 {
	wallet := w.GetWallet()

	return wallet.SetDelaySync(delay)
}

func (w *MobileWallet) Get_Delay_Sync() int64 {
	wallet := w.GetWallet()

	return wallet.GetDelaySync()
}

func (w *MobileWallet) Get_Daemon_Address() string {
	wallet := w.GetWallet()
	if wallet == nil {
		rlog.Errorf("Failed get daemon addres: no wallet open.")
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}
	return wallet.DaemonEndpoint
}

func (w *MobileWallet) Set_Daemon_Address(endpoints string) bool {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}

	endpoint := endpointDiscovery(endpoints)
	rlog.Infof("Discovery endpoint %s ==> %s", endpoints, endpoint)
	val := wallet.SetDaemonAddress(endpoint)
	if val != endpoint {
		setLastError(ErrSystemInternal, "Failed set daemon address to "+endpoint)
		return false
	}

	// trigger online cache state
	if err := wallet.IsDaemonOnline(); err != nil {
		rlog.Warnf("Daemon is not online, err %s", err)
	}

	return true
}

func (w *MobileWallet) Verify_Address(address string) string {
	//TODO:
	var wa MobileWalletAddress

	addr, err := globals.ParseValidateAddress(address)
	if err == nil {
		wa.IsValid = true
		if addr.IsIntegratedAddress() {
			wa.IsIntegratedAddress = true
			wa.PaymentID = fmt.Sprintf("%x", addr.PaymentID)
			//Fixme:
			wa.Address = addr.String()
		} else {
			wa.IsIntegratedAddress = false
			wa.Address = address
		}
	} else {
		wa.IsValid = false
	}

	buffer, err := json.Marshal(wa)

	return string(buffer)
}

func update_balance(w *MobileWallet) {
	wallet := w.GetWallet()
	var wst WalletStats
	var prev_balance uint64
	var available_count int

	count := 0
	wst.Wallet_available = true
	rlog.Infof("Enter update balance thread.")

	ticker := time.NewTicker(time.Duration(3) * time.Second)
	defer ticker.Stop()

	defer w.wg.Done()
	for {
		select {
		case <-w.quitChan:
			rlog.Infof("Leave update balance thread.")
			return
		case <-ticker.C:
			if wallet != nil {
				unlocked_balance, locked_balance := wallet.GetBalance()

				wst.Total_balance = unlocked_balance + locked_balance
				wst.Unlocked_balance = unlocked_balance
				wst.Locked_balance = locked_balance

				wst.Wallet_height = wallet.Get_Height()
				wst.Daemon_height = wallet.Get_Daemon_Height()
				wst.Wallet_topo_height = wallet.Get_TopoHeight()
				wst.Daemon_topo_height = wallet.Get_Daemon_TopoHeight()

				wst.Wallet_complete = !wallet.Is_View_Only()
				wst.Wallet_initial_height = wallet.GetInitialHeight()
				wst.Wallet_online = wallet.GetMode()
				wst.Wallet_mixin = wallet.GetMixin()
				wst.Wallet_fees_multiplier = float64(wallet.GetFeeMultiplier())
				wst.Wallet_sync_time = wallet.SetDelaySync(0)
				wst.Wallet_minimum_topo_height = wallet.GetMinimumTopoHeight()

				available := wallet.IsDaemonOnlineCached()
				if available == false {
					available_count++
					if available_count > 2 {
						wst.Wallet_available = false
					}
				} else {
					available_count = 0
					wst.Wallet_available = true
				}

				if prev_balance != wst.Total_balance {
					prev_balance = wst.Total_balance
					wst.Wallet_balance_changed = true
				}

				rlog.Infof("unlocked_balance = %s locked_balance = %s total = %s\n",
					globals.FormatMoney(wst.Unlocked_balance), globals.FormatMoney(wst.Locked_balance), globals.FormatMoney(wst.Total_balance))
				rlog.Infof("height = (%d/%d), topo height = (%d/%d), count = %d, available = %t\n",
					wst.Wallet_height, wst.Daemon_height, wst.Wallet_topo_height, wst.Daemon_topo_height, count, wst.Wallet_available)
				buffer, err := json.Marshal(wst)
				if err != nil {
					continue
				}
				if w.AppWalletStats != nil {
					w.AppWalletStats.DumpWalletStats(string(buffer))
				}
				wst.Wallet_balance_changed = false
			}

			if time.Now().Unix()%60 == 0 {
				//TODO:
				wallet.Save_Wallet()
			}
		}
	}
}

func (w *MobileWallet) Update_Wallet_Balance() bool {
	if w.IsSync == true {
		setLastError(ErrIsSync, "Balance update is already running.")
		return false
	}
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}
	if len(wallet.DaemonEndpoint) == 0 {
		setLastError(ErrInvalidDaemon, "Daemon address is not specified.")
		return false
	}

	w.Set_Online_Mode()

	w.wg.Add(1)
	go update_balance(w)

	w.IsSync = true
	return true
}

func (w *MobileWallet) Stop_Update_Blance() bool {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}
	if !w.IsSync {
		return true
	}

	w.quitChan <- 1
	w.wg.Wait()

	wallet.StopSync()

	w.Set_Offline_Mode()
	w.IsSync = false
	return true
}

func (w *MobileWallet) Set_DumpStats_Callback(s AppWalletStats) {
	w.AppWalletStats = s
}

func (w *MobileWallet) Get_Wallet_Name() string {
	filename := filepath.Base(w.FileName)
	fileSuffix := path.Ext(filename)

	walletname := strings.TrimSuffix(filename, fileSuffix)

	return walletname
}

func (w *MobileWallet) Backup_WalletFile(basedir string, password string) string {
	var binfo WalletBackupInfo

	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	if wallet.Check_Password(password) == false {
		setLastError(ErrPaswordMisMatch, "Backup wallet failed: Password mismatch.")
		return ""
	}

	nowtime := time.Now()
	seeds := wallet.GetSeed()

	binfo.Name = w.Get_Wallet_Name()
	binfo.Height = wallet.GetInitialHeight()
	binfo.Ctime = nowtime.Unix()
	binfo.Signature = crypto.Keccak256([]byte(seeds)).String()

	var err error
	binfo.Seeds, err = encryptWallet(seeds, password)
	if err != nil {
		setLastError(ErrSystemInternal, "Backup wallet failed", err)
		return ""
	}

	metadata := *(*[]byte)(unsafe.Pointer(&binfo))
	binfo.CheckSum = crypto.Keccak256(metadata).String()

	filename := fmt.Sprintf("%s/Backup_%04d%02d%02d_%s.bak",
		basedir, nowtime.Year(), nowtime.Month(), nowtime.Day(), binfo.Name)

	buffer, err := json.Marshal(binfo)

	err = ioutil.WriteFile(filename, buffer, 0644)
	if err != nil {
		setLastError(ErrSystemInternal, "Backup wallet failed: ", err)
		return ""
	}

	return filename
}

func (w *MobileWallet) Check_Password(password string) bool {
	if password == "" {
		setLastError(ErrInvalidPassword, "Password is empty.")
		return false
	}

	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}

	res := wallet.Check_Password(password)
	if res == false {
		setLastError(ErrPaswordMisMatch, "Password mismatch.")
	}

	return res
}

func (w *MobileWallet) Get_Tx_Key(txid string) string {
	if len(txid) != 64 {
		setLastError(ErrInvalidTxid, "Bad txid length.")
		return ""
	}
	_, err := hex.DecodeString(txid)
	if err != nil {
		setLastError(ErrInvalidTxid, "Error while decode txid.")
		return ""
	}

	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	key := wallet.GetTXKey(crypto.HexToHash(txid))

	return key
}

func (w *MobileWallet) Set_Lightweight_Mode(lightweight bool) bool {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return false
	}

	wallet.Lightweight_mode = lightweight
	return true
}

func (w *MobileWallet) Set_Time_Zone_Offset(offset int) {
	w.TimeZoneOffset = offset
	rlog.Infof("timezone %d", offset)
}

func (w *MobileWallet) Get_Time_Zone_Offset() int {
	return w.TimeZoneOffset
}

// package=github.com/darmaproject/darmasuite/utilities/appwallet
// gomobile bind -target android -ldflags "-X $package.chainType=testnet" $package
func NewMobileWallet() *MobileWallet {
	if strings.ToLower(chainType) == "testnet" {
		globals.Config = config.TestNet
	} else {
		globals.Config = config.MainNet
	}

	return &MobileWallet{
		quitChan:              make(chan int),
		quitAutomaticTransfer: make(chan int),
		FileName:              "wallet.db",
	}
}

func (w *MobileWallet) Stake_Buy_Share(pool_id string, amount_str string, reward string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	poolRaw, err := hex.DecodeString(pool_id)
	if err != nil {
		setLastError(ErrInvalidPool, "Bad pool id format.")
		return ""
	}
	var poolID crypto.Hash
	copy(poolID[:], poolRaw[:32])

	pool := wallet.GetStakePool(poolID)
	if pool == nil {
		setLastError(ErrInvalidPool, "No such stake pool.")
		return ""
	}

	amount, err := globals.ParseAmount(amount_str, false)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount: ", err)
		return ""
	}
	if amount < 10000000000 {
		setLastError(ErrInvalidAmount, "Min amount is 100 ", err)
		return ""
	}
	price := stake.GetSharePrice()
	if (amount % price) != 0 {
		setLastError(ErrInvalidAmount, "The number of votes cannot have a decimal")
		return ""
	}

	balance, _ := wallet.GetBalance()
	fmt.Println("balance:", balance, "  amount:", amount)
	if balance <= amount {
		setLastError(ErrInvalidAmount, "Insufficient unlocked balance.")
		return ""
	}

	rewardAddr, err := address.NewAddress(reward)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid reward address.")
		return ""
	}

	target := transaction.TxoutToBuyShare{
		Reward: *rewardAddr,
		Value:  amount,
		Pool:   poolID,
	}

	tx_extra := &transaction.TxCreateExtra{
		VoutTarget: target,
		LockedType: transaction.LOCKEDTYPE_LOCKED,
	}

	tx, inputs, _, _, err := wallet.TransferLocked(wallet.GetAddress(), amount, "", 0, 0, 0, tx_extra, 0, "")
	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	err = wallet.SendTransaction(tx) // relay tx to daemon/network
	if err != nil {
		setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
		return ""
	}

	txid := tx.GetHash()
	txResult := structures.BuyShareResult{
		TxId:    txid.String(),
		ShareId: stake.GetShareId(poolID, txid, *rewardAddr).String(),
	}

	buffer, err := json.Marshal(txResult)
	return string(buffer)
}

func (w *MobileWallet) Stake_Repo_Share(share_id string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	share, err := wallet.GetBuyShare(crypto.HashHexToHash(share_id))
	if err != nil {
		setLastError(ErrInvalidShare, "No such share.", err)
		return ""
	}

	target := transaction.TxoutToRepoShare{
		ShareId: crypto.HashHexToHash(share_id),
	}

	tx_extra := &transaction.TxCreateExtra{
		VoutTarget: target,
		LockedType: transaction.LOCKEDTYPE_UNLOCKED,
		LockedTxId: share.TxId,
	}

	tx, inputs, _, _, err := wallet.TransferLocked(wallet.GetAddress(), 0, "", 0, 0, 0, tx_extra, 0, "")
	_ = inputs
	if err != nil {
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	err = wallet.SendTransaction(tx) // relay tx to daemon/network
	if err != nil {
		setLastError(ErrTxIsRejected, "Transaction sending failed: ", err)
		return ""
	}

	txid := tx.GetHash()
	return txid.String()
}

func (w *MobileWallet) Stake_Get_Share(share_id string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	r, err := wallet.GetShareRpc(share_id, false)
	if err != nil {
		rlog.Errorf("Rpc failed, err %s", err)
		setLastError(ErrInvalidShare, err.Error())
		return ""
	}

	share, err := wallet.GetBuyShare(crypto.HashHexToHash(share_id))
	if err != nil {
		setLastError(ErrDecodeData, "No such share.", err)
		return ""
	}
	result := ShareResult{
		GetShareResult: *r,
		BuyTime:        share.Time,
		PoolName:       share.PoolName,
		TotalNum:       r.InitNum,
	}

	buffer, _ := json.Marshal(result)
	return string(buffer)
}

func (w *MobileWallet) Stake_List_Buy_Shares(max_height_str string, limit_str string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	maxHeight, _ := strconv.ParseUint(max_height_str, 10, 64)
	if maxHeight == 0 {
		maxHeight = wallet.Get_Height()
	} else {
		maxHeight -= 1
	}
	limit, _ := strconv.Atoi(limit_str)
	if limit == 0 {
		limit = 10
	}

	var result []ListBuySharesResult
	shares, _ := wallet.ListBuyShares(maxHeight, limit, "")
	var err error
	for _, share := range shares {
		var buyshare ListBuySharesResult
		if buyshare.Transfer, err = wallet.GetTransferByTXID(share.TxId); err != nil {
			rlog.Warnf("no tx %s, share %s", share.TxId, share.ShareId)
			continue
		}

		buyshare.ShareId = share.ShareId.String()
		buyshare.Amount = share.Amount
		buyshare.Closed = share.Closed

		result = append(result, buyshare)
	}

	buffer, _ := json.Marshal(result)
	return string(buffer)
}

func (w *MobileWallet) Stake_List_Shares(max_height_str string, limit_str string, pool_id string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	maxHeight, _ := strconv.ParseUint(max_height_str, 10, 64)
	if maxHeight == 0 {
		maxHeight = wallet.Get_Height()
	} else {
		maxHeight -= 1
	}
	limit, _ := strconv.Atoi(limit_str)
	if limit == 0 {
		limit = 10
	}

	shares, err := wallet.ListBuyShares(maxHeight, limit, pool_id)
	if err != nil {
		setLastError(ErrSystemInternal, err.Error())
		return ""
	}

	buffer, _ := json.Marshal(shares)
	return string(buffer)
}

func (w *MobileWallet) Stake_Register_Pool(pool_name, vote_addr, reward_addr string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	if wallet.Is_View_Only() {
		setLastError(ErrSystemInternal, "View Only wallet cannot transfer")
		return ""
	}

	vote, err := address.NewAddress(vote_addr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid vote address")
		return ""
	}
	if vote.IsSubAddress() == true {
		setLastError(ErrInvalidAddress, "Subaddress is not supported")
		return ""
	}

	reward, err := address.NewAddress(reward_addr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid reward address")
		return ""
	}

	name, err := globals.ParsePoolName(pool_name)
	from := wallet.GetAddress()

	poolId := stake.GetStakePoolId(vote, reward, name)
	amount := stake.TESTNET_STAKE_POOL_AMOUNT
	if globals.IsMainnet() {
		amount = stake.GetStakePoolAmount(wallet.DaemonHeight)
	}

	tx_extra := &transaction.TxCreateExtra{
		VoutTarget: transaction.TxoutToRegisterPool{
			Name:   name,
			Id:     poolId,
			Vote:   *vote,
			Reward: *reward,
			Value:  amount,
		},
		LockedType: transaction.LOCKEDTYPE_LOCKED,
	}

	tx, _, _, _, err := wallet.TransferLocked(from, amount, "", 0, 0, 5, tx_extra, 0, "")
	if err != nil {
		setLastError(ErrAgain, "Create transaction failed", err)
		return ""
	}

	err = wallet.SendTransaction(tx) // relay tx to daemon/network
	if err != nil {
		setLastError(ErrSystemInternal, "Failed to send registration pool transaction: ", err)
		return ""
	}

	rlog.Infof("Send registration pool transaction successfully: txid=%s", tx.GetHash())

	return poolId.String()
}

func (w *MobileWallet) Stake_Get_Pool(pool_id string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	pool_ids := strings.Split(pool_id, ",")
	results := []structures.GetStakePoolResult{}
	for _, id := range pool_ids {
		result, err := wallet.GetStakePoolRpc(crypto.HashHexToHash(id))
		if err != nil {
			setLastError(ErrInvalidPool, "No such Pool.")
			continue
		}
		results = append(results, *result)
	}

	buffer, _ := json.Marshal(results)
	return string(buffer)
}

func (w *MobileWallet) Stake_List_All_Pools(owner bool) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	response := wallet.ListStakePoolsRpc()
	if response == nil {
		setLastError(ErrInvalidPool, "Pools get failed.")
		return ""
	}
	var result structures.ListStakePoolResult
	err := response.GetObject(&result)
	if err != nil {
		setLastError(ErrInvalidPool, "Pools get failed.", err)
		return ""
	}

	if owner == true {
		res := structures.ListStakePoolResult{
			Pools: make([]*structures.GetStakePoolResult, 0),
			Count: 0,
		}
		for _, v := range result.Pools {
			entry := wallet.GetTxOutputs(crypto.HexToHash(v.TxHash))
			if len(entry) > 0 {
				res.Pools = append(res.Pools, v)
				res.Count++
			}
		}
		result = res
	}

	buffer, err := json.Marshal(result.Pools)
	return string(buffer)
}

func (w *MobileWallet) Stake_List_Pools() string {
	return w.Stake_List_All_Pools(false)
}

func (w *MobileWallet) Stake_List_MyPools() string {
	return w.Stake_List_All_Pools(true)
}

func (w *MobileWallet) Stake_List_Profits(max_height_str, limit_str string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	maxHeight, _ := strconv.ParseUint(max_height_str, 10, 64)
	if maxHeight == 0 {
		maxHeight = wallet.Get_Height()
	} else {
		maxHeight -= 1 // exclusive
	}

	limit, _ := strconv.Atoi(limit_str)
	if limit == 0 {
		limit = 10
	}

	// fixme: merge entries of the same tx ???
	result := wallet.ShowTransfersV3(true, false, true, false, false, false, 0, maxHeight, limit, globals.TX_TYPE_SHARE_PROFIT)

	buffer, err := json.Marshal(result)
	if err != nil {
		setLastError(ErrEncodeData, "Encode Profit records failed: ", err)
		return ""
	}
	return string(buffer)
}

func (w *MobileWallet) Stake_Get_Profit_Weekly() string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	location := time.FixedZone("GoTimeZone", w.TimeZoneOffset)

	var result []GetProfitWeeklyResult
	lastWeekProfit := wallet.GetLastWeekProfit(w.TimeZoneOffset)
	for i := len(lastWeekProfit) - 1; i >= 0; i-- {
		result = append(result, GetProfitWeeklyResult{
			Amount: lastWeekProfit[i].Amount,
			Date:   time.Unix(lastWeekProfit[i].StartTime, 0).In(location).Format("01.02"),
		})
	}

	buffer, err := json.Marshal(result)
	if err != nil {
		setLastError(ErrEncodeData, "Encode Profit weekly records failed: ", err)
		return ""
	}
	return string(buffer)
}

func (w *MobileWallet) Stake_Get_Price(amountstr string) string {
	amount, _ := globals.ParseAmount(amountstr, false)
	if amount == math.MaxUint64 {
		setLastError(ErrInvalidAmount, fmt.Sprintf("amount %s is too big", amountstr))
		return ""
	}

	price := stake.GetSharePrice()
	num := amount / price

	d := struct {
		Num   uint64 `json:"num"`
		Price uint64 `json:"price"`
	}{num, price}
	data, _ := json.Marshal(d)
	return string(data)
}

func (w *MobileWallet) Get_Sub_Address() string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	addr, ok := wallet.GetSubAddressRandom()
	if ok == false {
		setLastError(ErrSystemInternal, "Wallet is not open.")
		return ""
	}

	return addr.String()
}

func (w *MobileWallet) Get_Info() string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	info, err := Get_Info_Request(wallet.DaemonEndpoint)
	if err != nil {
		setLastError(ErrInvalidDaemon, err.Error())
		return ""
	}

	r, _ := json.Marshal(info)
	return string(r)
}

func (w *MobileWallet) Stake_Getinfo() string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	shares, err := wallet.ListBuyShares(wallet.Get_Height(), 0, "")
	if err != nil {
		setLastError(ErrInvalidShare, "Shares get failed.", err)
		return ""
	}

	var result StakeInfoResult
	result.Profit = wallet.GetProfit()
	result.ProfitYestarday = wallet.GetYesterdayProfit(w.TimeZoneOffset)

	for _, v := range shares {
		num, _, _ := stake.CalcShareNum(v.Amount)
		if v.Closed == false {
			result.LockAmount += v.Amount
			result.ShareNum += num
		}
	}
	var rate uint64
	if result.LockAmount > 0 {
		rate = result.Profit * 10000 / result.LockAmount
	}
	result.ProfitRate = strconv.FormatUint(rate/100, 10) + "." + strconv.FormatUint(rate%100, 10)

	buffer, err := json.Marshal(result)
	return string(buffer)
}

func (w *MobileWallet) ERC20_Get_Address() string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	subs := wallet.ListSubAddress()
	if len(subs) == 0 {
		sub := wallet.NewSubAddress(0)
		if sub == nil {
			setLastError(ErrSystemInternal, "Can't create subaddress")
			return ""
		} else {
			return sub.String()
		}
	}

	// use the 1st subaddress to receive erc20 tokens
	return subs[0].String()
}

func (w *MobileWallet) ERC20_Get_Transfers(in bool, out bool, max_height_str, limit_str, contract string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	maxHeight, _ := strconv.ParseUint(max_height_str, 10, 64)
	if maxHeight == 0 {
		maxHeight = wallet.Get_Height()
	} else {
		maxHeight -= 1 // exclusive
	}

	limit, _ := strconv.Atoi(limit_str)
	if limit == 0 {
		limit = 10
	}

	entries := wallet.ShowERC20Transfers(in, out, 0, maxHeight)
	var stripped []walletapi.Erc20Entry
	// align to topo height
	for k, v := range entries {
		if len(stripped) >= limit {
			if k < len(entries)-1 {
				next := entries[k+1]
				if next.TopoHeight != v.TopoHeight {
					break
				}
			}
		}

		stripped = append(stripped, v)
	}

	data, _ := json.Marshal(stripped)

	return string(data)
}

func (w *MobileWallet) ERC20_Transfer(toaddr, amountstr, password, contract string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	// get decimals
	decimals, err := wallet.GetTokenDecimals(contract)
	if err != nil {
		setLastError(ErrSystemInternal, err.Error())
		return ""
	}

	// validate address
	addr, err := globals.ParseValidateAddress(toaddr)
	if err != nil {
		setLastError(ErrInvalidAddress, "Invalid address: ", err)
		return ""
	}

	// parse amount
	amount, err := globals.ParseTokenAmount(amountstr, decimals)
	if err != nil {
		setLastError(ErrInvalidAmount, "Invalid amount: ", err)
		return ""
	}

	// pack input
	input := walletapi.ERC20PackInput("transfer", addr.ToContractAddress(), amount)

	return w.contractTransfer(input, contract, 0, 0, 0, false)
}

func (w *MobileWallet) ERC20_Getinfo(contract string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	name, err := wallet.GetTokenName(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get name.")
		return ""
	}

	symbol, err := wallet.GetTokenSymbol(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get symbol.")
		return ""
	}

	decimals, err := wallet.GetTokenDecimals(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get decimals.")
		return ""
	}

	supply, err := wallet.GetTokenSupply(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get supply.")
		return ""
	}

	info := struct {
		Name        string `json:"name"`
		Symbol      string `json:"symbol"`
		TotalSupply string `json:"total_supply"`
		Decimals    int    `json:"decimals"`
	}{name, symbol, supply.String(), decimals}
	data, _ := json.Marshal(info)
	return string(data)
}

func (w *MobileWallet) ERC20_GetBalance(contract string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	decimals, err := wallet.GetTokenDecimals(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get decimals.")
		return ""
	}

	balance, err := wallet.GetTokenBalance(contract)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "Failed get balance.")
		return ""
	}

	return globals.FormatTokenMoney(balance, decimals)
}

func getAbi(abiFile string) *abi.ABI {
	abiCode, err := ioutil.ReadFile(abiFile)
	if err != nil {
		setLastError(ErrInvalidWalletObject, "File read failed: ", err)
		return nil
	}
	Abi, err := wavm.GetAbi(abiCode)
	if err != nil {
		setLastError(ErrDecodeData, "Get Abi failed: ", err)
		return nil
	}
	return &Abi
}

func contractCode(compressFile, abiFile, funcName, input string) ([]byte, *abi.ABI) {
	code := make([]byte, 0)
	if compressFile != "" {
		compressCode, err := ioutil.ReadFile(compressFile)
		if err != nil {
			setLastError(ErrInvalidWalletObject, "File read failed: ", err)
			return nil, nil
		}
		code = append(code, compressCode...)
	}

	Abi := getAbi(abiFile)
	if Abi == nil {
		return nil, nil
	}

	var args []string
	if input != "" {
		err := json.Unmarshal([]byte(input), &args)
		if err != nil {
			setLastError(ErrDecodeData, "Decode params failed: ", err)
			return nil, Abi
		}
	}
	data, err := Abi.PackStrArgs(funcName, args...)
	if err != nil {
		setLastError(ErrDecodeData, err.Error())
		return nil, Abi
	}
	code = append(code, data...)
	return code, Abi
}

func (w *MobileWallet) contractTransfer(code []byte, contractAddr string, amount, gas, gasPrice uint64, isCreate bool) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	tx, _, input_sum, change, err := wallet.BuildContractTx(code, amount, gas, gasPrice, contractAddr, isCreate)
	if err != nil {
		rlog.Warnf("Error while building Transaction err %s\n", err)
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("Inputs Selected for %s \n", globals.FormatMoney(input_sum))
	rlog.Infof("Transfering total amount %s \n", globals.FormatMoney(amount))
	rlog.Infof("change amount ( will come back ) %s \n", globals.FormatMoney(change))
	rlog.Infof("fees %s \n", globals.FormatMoney(tx.RctSignature.GetTXFee()))

	err = wallet.SendTransaction(tx)
	if err == nil {
		rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())
	} else {
		rlog.Warnf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
		setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
	}

	return tx.GetHash().String()
}

func (w *MobileWallet) Contract_Create(compress_file, abi_file, input, amount, gas, gas_price string) string {
	return w.Contract_Create_Wavm_ContractTx(compress_file, abi_file, input, amount, gas, gas_price)
}

func (w *MobileWallet) Contract_Create_Wavm_ContractTx(compress_file, abi_file, input, amount, gas, gas_price string) string {
	code, _ := contractCode(compress_file, abi_file, "", input)
	if code == nil {
		return ""
	}
	return w.contractTransfer_string(code, "", amount, gas, gas_price, true)
}

func (w *MobileWallet) contractTransfer_string(code []byte, contractAddr, amount, gas, gas_price string, isCreate bool) string {
	amountUint, _ := strconv.ParseUint(amount, 10, 64)
	gasUint, _ := strconv.ParseUint(gas, 10, 64)
	gasPriceUint, _ := strconv.ParseUint(gas_price, 10, 64)
	return w.contractTransfer(code, contractAddr, amountUint, gasUint, gasPriceUint, isCreate)
}

func (w *MobileWallet) Create_Evm_ContractTx(code_file, abi_file, input, amount, gas, gas_price string) string {
	bytecode, err := getBytecode(code_file)
	if bytecode == nil || err != nil {
		return ""
	}
	abi := getAbi(abi_file)
	if abi == nil {
		return ""
	}
	code, _ := packContract(bytecode, abi, "", input)
	if code == nil {
		return ""
	}
	return w.contractTransfer_string(code, "", amount, gas, gas_price, true)
}

func getBytecode(hexFile string) ([]byte, error) {
	code := make([]byte, 0)
	if hexFile != "" {
		hexCode, err := ioutil.ReadFile(hexFile)
		if err != nil {
			setLastError(ErrInvalidWalletObject, "File read failed: ", err)
			return nil, nil
		}
		bytes, err := hex.DecodeString(string(hexCode))
		if err != nil {
			setLastError(ErrInvalidWalletObject, "Decode hex code failed: ", err)
			return nil, nil
		}
		code = append(code, bytes...)
	}
	return code, nil
}

func packContract(bytecode []byte, abi *abi.ABI, funcName, inputParams string) ([]byte, error) {
	var args []string
	if inputParams != "" {
		err := json.Unmarshal([]byte(inputParams), &args)
		if err != nil {
			setLastError(ErrDecodeData, "Decode input params failed: ", err)
			return nil, nil
		}
	}
	data, err := abi.PackStrArgs(funcName, args...)
	if err != nil {
		setLastError(ErrDecodeData, err.Error())
		return nil, nil
	}
	bytecode = append(bytecode, data...)
	return bytecode, nil
}

func (w *MobileWallet) Contract_Deposit(strAmount string) string {
	amount,err := ParseUint64Base10(strAmount)
	if err != nil {
		rlog.Warnf("Error while parsing amount %s\n", err)
		setLastError(ErrInvalidAmount, "Amount parse error: ", err)
		return ""
	}

	wallet := w.GetWallet()
	tx, err := wallet.BuildDepositTx(amount)
	if err != nil {
		rlog.Warnf("Error while building Transaction err %s\n", err)
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("fees %s \n", globals.FormatMoney(tx.RctSignature.GetTXFee()))

	err = wallet.SendTransaction(tx)
	if err == nil {
		rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())
	} else {
		rlog.Warnf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
		setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
	}

	return tx.GetHash().String()
}

func (w *MobileWallet) Contract_Withdraw(strAmount string) string {
	amount,err := ParseUint64Base10(strAmount)
	if err != nil {
		rlog.Warnf("Error while parsing amount %s\n", err)
		setLastError(ErrInvalidAmount, "Amount parse error: ", err)
		return ""
	}

	wallet := w.GetWallet()
	tx, err := wallet.BuildWithdrawTx(amount)
	if err != nil {
		rlog.Warnf("Error while building Transaction err %s\n", err)
		setLastError(ErrSystemInternal, "Error while building Transaction: ", err)
		return ""
	}

	rlog.Infof("fees %s \n", globals.FormatMoney(tx.RctSignature.GetTXFee()))

	err = wallet.SendTransaction(tx)
	if err == nil {
		rlog.Infof("Transaction sent successfully. txid = %s", tx.GetHash())
	} else {
		rlog.Warnf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
		setLastError(ErrSystemInternal, "Transaction sending failed: ", err)
	}

	return tx.GetHash().String()
}

func ParseUint64Base10(a string) (uint64,error) {
	return strconv.ParseUint(a, 10, 64)
}

func (w *MobileWallet) Contract_Send(abi_file, func_name, input, addr, amount, gas, gas_price string) string {
	code, _ := contractCode("", abi_file, func_name, input)
	if code == nil {
		return ""
	}
	amountUint, _ := strconv.ParseUint(amount, 10, 64)
	gasUint, _ := strconv.ParseUint(gas, 10, 64)
	gasPriceUint, _ := strconv.ParseUint(gas_price, 10, 64)
	return w.contractTransfer(code, addr, amountUint, gasUint, gasPriceUint, false)
}

func (w *MobileWallet) Contract_Addr(tx_hash string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	return wallet.GetContractAddr(tx_hash)
}

func (w *MobileWallet) Contract_Call(abi_file, func_name, input, addr, amount, gas, gas_price, height string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	code, Abi := contractCode("", abi_file, func_name, input)
	if code == nil {
		return ""
	}

	p := structures.CallContractParams{}
	p.Data = fmt.Sprintf("0x%x", code)
	p.Amount, _ = strconv.ParseUint(amount, 10, 64)
	p.Gas, _ = strconv.ParseUint(gas, 10, 64)
	p.GasPrice, _ = strconv.ParseUint(gas_price, 10, 64)
	p.From = wallet.GetAddress().String()
	p.TopoHeight, _ = strconv.ParseInt(height, 10, 64)
	p.To = addr

	result := wallet.CallContract(p)
	if result == "" {
		setLastError(ErrSystemInternal, "No result")
		return ""
	}

	output, _ := hex.DecodeString(result)
	val, err := Abi.UnpackStrArgs(func_name, output)
	if err != nil {
		setLastError(ErrSystemInternal, err.Error())
		return ""
	}
	return val
}

func (w *MobileWallet) Contract_Result(abi_file, func_name, tx_hash string) string {
	wallet := w.GetWallet()
	if wallet == nil {
		setLastError(ErrInvalidWalletObject, "Wallet is not open.")
		return ""
	}

	Abi := getAbi(abi_file)
	if Abi == nil {
		return ""
	}

	p := structures.GetContractResultParams{}
	p.TXHash = tx_hash
	result := wallet.GetContractResult(p)
	if result == "" {
		setLastError(ErrSystemInternal, "No result")
		return ""
	}

	output, _ := hex.DecodeString(result)
	val, err := Abi.UnpackStrArgs(func_name, output)
	if err != nil {
		setLastError(ErrSystemInternal, err.Error())
		return ""
	}
	return val
}

func (w *MobileWallet) Set_Rlog_Env() {
	if os.Getenv("RLOG_LOG_LEVEL") == "" {
		os.Setenv("RLOG_LOG_LEVEL", "INFO") // default logging in debug mode
	}

	if os.Getenv("RLOG_LOG_FILE") == "" {
		filename := w.FileName + ".log"
		os.Setenv("RLOG_LOG_FILE", filename) // default log file name
	}

	if os.Getenv("RLOG_LOG_STREAM") == "" {
		os.Setenv("RLOG_LOG_STREAM", "NONE") // do not log to stdout/stderr
	}

	if os.Getenv("RLOG_CALLER_INFO") == "" {
		os.Setenv("RLOG_CALLER_INFO", "RLOG_CALLER_INFO") // log caller info
	}

	rlog.UpdateEnv()
}

func (w *MobileWallet) GetContractAccountAddress(darmaAddress string) (string,error) {
	a, err := address.NewAddress(darmaAddress)
	if err != nil {
		return "", fmt.Errorf("address is invalid")
	}
	darmaBytesAddres := a.ToContractAddress()
	contractBytesAddress := common.DarmaAddressToContractAddress(darmaBytesAddres)
	contractStrAddress := fmt.Sprintf("%x",contractBytesAddress[12:]) // get last 20 bytes from 32 bytes
	return contractStrAddress, nil
}