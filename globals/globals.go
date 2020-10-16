// Copyright 2018-2020 Darma Project. All rights reserved.

package globals

import (
	"fmt"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
	"math"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/config"
)

type ChainState int // block chain can only be in 2 state, either SYNCRONISED or syncing

const (
	SYNCRONISED ChainState = iota // 0
	SYNCING                       // 1
)

// all the the global variables used by the program are stored here
// since the entire logic is designed around a state machine driven by external events
// once the core starts nothing changes until there is a network state change

var Incoming_Block = make([]byte, 100) // P2P feeds it, blockchain consumes it
var Outgoing_Block = make([]byte, 100) // blockchain feeds it, P2P consumes it  only if a block has been mined

var Incoming_Tx = make([]byte, 100) // P2P feeds it, blockchain consumes it
var Outgoing_Tx = make([]byte, 100) // blockchain feeds it, P2P consumes it  only if a  user has created a Tx mined

var SubsystemActive uint32 // atomic counter to show how many subsystems are active
var Exit_In_Progress bool

// on init this variable is updated to setup global config in 1 go
var Config config.ChainConfig

// global logger all components will use it with context

var Logger *logrus.Logger
var ilog_formatter *logrus.TextFormatter // used while tracing code

var Dialer proxy.Dialer = proxy.Direct // for proxy and direct connections
// all outgoing connections , including DNS requests must be made using this

// all program arguments are available here
var Arguments map[string]interface{}

func Initialize() {
	var err error
	_ = err

	Config = config.MainNet // default is mainnnet

	if Arguments["--testNet"].(bool) == true { // setup testNet if requested
		Config = config.TestNet
	}

	if ok := Arguments["--netEnv"]; ok != nil {
		if Arguments["--netEnv"].(string) == "devNet" {
			Config.NetEnv = "devNet"
			Config.NetworkId = uuid.FromBytesOrNil([]byte{0x64, 0x6d, 0x61, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0xf6, 0xe0, 0x9a, 0x04, 0x07, 0x24})
		}
	}

	Logger = logrus.New()
	Logger.Formatter = &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02 15:04:05"}

	level := "info"
	Logger.SetLevel(logrus.InfoLevel)
	if Arguments["--log-level"] != nil {
		level = Arguments["--log-level"].(string)
		switch strings.ToLower(level) {
		case "trace":
			Logger.SetLevel(logrus.TraceLevel)
		case "debug":
			Logger.SetLevel(logrus.DebugLevel)
		case "info":
			Logger.SetLevel(logrus.InfoLevel)
		case "warn":
			Logger.SetLevel(logrus.WarnLevel)
		case "error":
			Logger.SetLevel(logrus.ErrorLevel)
		}
	}

	initRlog(level)
	Logger.AddHook(&HOOK) // add rlog hook

	// choose  socks based proxy if user requested so
	if Arguments["--socks-proxy"] != nil {
		log.Debugf("Setting up proxy using %s", Arguments["--socks-proxy"].(string))
		//uri, err := url.Parse("socks5://127.0.0.1:9000") // "socks5://demo:demo@192.168.99.100:1080"
		uri, err := url.Parse("socks5://" + Arguments["--socks-proxy"].(string)) // "socks5://demo:demo@192.168.99.100:1080"
		if err != nil {
			log.Fatalf("Error parsing socks proxy: err %s", err)
		}

		Dialer, err = proxy.FromURL(uri, proxy.Direct)
		if err != nil {
			log.Fatalf("Error creating socks proxy: err \"%s\" from data %s ", err, Arguments["--socks-proxy"].(string))
		}
	}

	// windows and logrus have issues while printing colored messages, so disable them right now
	ilog_formatter = &logrus.TextFormatter{} // this needs to be created after after top logger has been intialised
	ilog_formatter.DisableColors = true
	ilog_formatter.DisableTimestamp = true

	// lets create data directories
	err = os.MkdirAll(GetDataDirectory(), 0750)
	if err != nil {
		fmt.Printf("Error creating/accessing directory %s , err %s\n", GetDataDirectory(), err)
	}
}

// tells whether we are in mainNet mode
// if we are not mainNet, we are a testNet,
// we will only have a single mainNet ,( but we may have one or more testnets )
func IsMainnet() bool {
	if Config.Name == "mainNet" {
		return true
	}

	return false
}

func GetBonusLifetimeHeight() int64 {
	if IsMainnet() {
		return config.MAINNET_BONUS_LIFETIME_HEIGHT
	} else {
		return config.TESTNET_BONUS_LIFETIME_HEIGHT
	}
}

func GetOmniTokenHardforkHeight() int64 {
	if IsMainnet() {
		return config.MAINNET_OMNI_TOKEN_HEIGHT
	} else {
		return config.TESTNET_OMNI_TOKEN_HEIGHT
	}
}

func GetBonusDelayHeight() int64 {
	if IsMainnet() {
		return config.MAINNET_BLOCK_DELAY_HEIGHT
	} else {
		return config.TESTNET_BONUS_DELAY_HEIGHT
	}
}

func IsBonusHeight(height int64) bool {
	currentBonusTxHeight := height - GetVotingStartHeight() - GetBonusDelayHeight()
	return currentBonusTxHeight > 0 && currentBonusTxHeight%GetBonusLifetimeHeight() == 0
}

func GetVotingStartHeight() int64 {
	if strings.ToLower(Config.Name) == "mainnet" {
		return config.MAINNET_DPOS_HEIGHT + config.MAINNET_DPOS_PREPARE_HEIGHT
	}
	return config.TESTNET_DPOS_HEIGHT + config.TESTNET_DPOS_PREPARE_HEIGHT
}

func GetCycleStartHeight() int64 {
	if strings.ToLower(Config.Name) == "mainnet" {
		return config.CYCLE_DPOS_REWARD
	}
	return config.TESTNET_CYCLE_DPOS_REWARD
}

func GetTokenId(symbol string) (id crypto.Hash) {
	if len(symbol) == 0 {
		return
	}

	id = crypto.Keccak256([]byte("SYMBOL"), []byte(symbol))
	return
}

// return different directories for different networks ( mainly mainNet, testNet, simulation )
// this function is specifically for daemon
func GetDataDirectory() string {
	data_directory, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error obtaining current directory, using temp dir err %s\n", err)
		data_directory = os.TempDir()
	}

	// if user provided an option, override default
	if Arguments["--data-dir"] != nil {
		data_directory = Arguments["--data-dir"].(string)
	}

	if IsMainnet() {
		return filepath.Join(data_directory, "mainNet")
	}

	return filepath.Join(data_directory, "testNet")
}

/* this function converts a logrus entry into a txt formater based entry with no colors  for tracing*/
func CTXString(entry *logrus.Entry) string {

	entry.Level = logrus.DebugLevel
	data, _ := ilog_formatter.Format(entry)
	return string(data)
}

func FormatTokenMoney(amount *big.Int, decimals int) string {
	famount, _ := new(big.Float).SetString(amount.String())
	famount.Quo(famount, new(big.Float).SetFloat64(math.Pow10(decimals)))
	return famount.Text('f', decimals)
}

// never do any division operation on money due to floating point issues
// newbies, see type the next in python interpretor "3.33-3.13"
//
func FormatMoney(amount uint64) string {
	decimals := math.Log10(config.COIN_UNIT)
	return FormatMoneyPrecision(amount, int(decimals)) // default is 8 precision after floating point
}

func MultiplyMoney(amount uint64, factor float64) (a uint64) {
	factorBig := new(big.Float).SetFloat64(factor)
	amountBig, _, _ := big.ParseFloat(fmt.Sprintf("%d", amount), 10, 0, big.ToZero)
	result := new(big.Float)
	result.Mul(amountBig, factorBig)
	a, _ = strconv.ParseUint(result.Text('f', 0), 10, 64)
	return
}

// 0
func FormatMoney0(amount uint64) string {
	return FormatMoneyPrecision(amount, 0)
}

//8 precision
func FormatMoney8(amount uint64) string {
	return FormatMoneyPrecision(amount, 8)
}

// 12 precision
func FormatMoney12(amount uint64) string {
	return FormatMoneyPrecision(amount, 12)
}

// 5 precision
func FormatMoney5(amount uint64) string {
	return FormatMoneyPrecision(amount, 5) // default is 8 precision after floating point
}

// format money with specific precision
func FormatMoneyPrecision(amount uint64, precision int) string {
	hard_coded_decimals := new(big.Float).SetFloat64(config.COIN_UNIT)
	float_amount, _, _ := big.ParseFloat(fmt.Sprintf("%d", amount), 10, 0, big.ToZero)
	result := new(big.Float)
	result.Quo(float_amount, hard_coded_decimals)
	return result.Text('f', precision) // 8 is display precision after floating point
}

func ParsePoolName(str string) (name string, err error) {
	name = strings.TrimSpace(str)
	if len(name) == 0 || len(name) > 32 {
		return "", fmt.Errorf("Name length must be >= 1 and <= 32")
	}

	return
}

func ParseTokenName(str string) (name string, err error) {
	name = strings.TrimSpace(str)
	if len(name) == 0 || len(name) > 32 {
		return "", fmt.Errorf("Name length must be >= 1 and <= 32")
	}

	return
}

func ParseTokenSymbol(str string) (symbol string, err error) {
	symbol = strings.TrimSpace(str)
	symbol = strings.ToUpper(symbol)

	if !IsLetter(symbol) {
		return "", fmt.Errorf("TokenSymbol must be uppercase letter")
	}

	if !IsLimitLength(symbol) {
		return "", fmt.Errorf("TokenSymbol length must be >= 2 and <= 5")
	}

	return
}

// this will parse and validate an address, in reference to the current main/test mode
func ParseValidateAddress(str string) (addr *address.Address, err error) {
	addr, err = address.NewAddress(strings.TrimSpace(str))
	if err != nil {
		return
	}

	// check whether the domain is valid
	if !addr.IsDARMANetwork() {
		err = fmt.Errorf("Invalid Darma address")
		return
	}

	if IsMainnet() != addr.IsMainnet() {
		if IsMainnet() {
			err = fmt.Errorf("Address belongs to Darma testNet and is invalid")
		} else {
			err = fmt.Errorf("Address belongs to Darma mainNet and is invalid")
		}
		return
	}

	return
}

func ParseTokenAmount(str string, decimals int) (amount *big.Int, err error) {
	if str == "" {
		return new(big.Int).SetInt64(0), nil
	}

	floatAmount, base, err := big.ParseFloat(strings.TrimSpace(str), 10, 0, big.ToZero)
	if err != nil {
		err = fmt.Errorf("Amount could not be parsed err: %s", err)
		return
	}
	if base != 10 {
		err = fmt.Errorf("Amount should be in base 10 (0123456789)")
		return
	}
	if floatAmount.Cmp(new(big.Float).Abs(floatAmount)) != 0 { // number and abs(num) not equal means number is neg
		err = fmt.Errorf("Amount cannot be negative")
		return
	}

	coinUnit := big.NewFloat(0).SetFloat64(math.Pow10(decimals))
	floatAmount.Mul(floatAmount, coinUnit)
	amount, _ = new(big.Int).SetString(floatAmount.Text('f', 0), 10)

	return
}

// this will covert an amount in string form to atomic units
func ParseAmount(str string, shouldBeInt bool) (amount uint64, err error) {
	if str == "" {
		return 0, nil
	}
	float_amount, base, err := big.ParseFloat(strings.TrimSpace(str), 10, 0, big.ToZero)

	if err != nil {
		err = fmt.Errorf("Amount could not be parsed err: %s", err)
		return
	}
	if base != 10 {
		err = fmt.Errorf("Amount should be in base 10 (0123456789)")
		return
	}
	if float_amount.Cmp(new(big.Float).Abs(float_amount)) != 0 { // number and abs(num) not equal means number is neg
		err = fmt.Errorf("Amount cannot be negative")
		return
	}

	// multiply by 12 zeroes
	hard_coded_decimals := big.NewFloat(0).SetFloat64(config.COIN_UNIT)
	float_amount.Mul(float_amount, hard_coded_decimals)

	if shouldBeInt && !float_amount.IsInt() {
		err = fmt.Errorf("Amount  is invalid %s ", float_amount.Text('f', 0))
		return
	}

	// convert amount to uint64
	//amount, _ = float_amount.Uint64() // sanity checks again
	amount, err = strconv.ParseUint(float_amount.Text('f', 0), 10, 64)
	if err != nil {
		err = fmt.Errorf("Amount  is invalid %s ", str /*float_amount.Text('f', 0)*/)
		return
	}
	if amount == 0 {
		err = fmt.Errorf("0 cannot be transferred")
		return
	}

	if amount == math.MaxUint64 {
		err = fmt.Errorf("Amount  is invalid")
		return
	}

	return // return the number
}

func GetEmptyAddress() *address.Address {
	var a address.Address // a blank address to 0
	if IsMainnet() {
		a.Network = config.MainNet.PublicAddressPrefix
	} else {
		a.Network = config.TestNet.PublicAddressPrefix
	}
	return &a
}

func GetNetwork() uint64 {
	return Config.PublicAddressPrefix
}

func IsLetter(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func IsLimitLength(s string) bool {
	if len(s) < 2 || len(s) > 5 {
		return false
	}

	return true
}

func StringCost(s string) (cost uint64) {
	switch len(s) {
	case 2:
		cost = SymbolSpring * config.COIN_UNIT
	case 3:
		cost = SymbolSummer * config.COIN_UNIT
	case 4:
		cost = SymbolAutumn * config.COIN_UNIT
	case 5:
		cost = SymbolWinter * config.COIN_UNIT
	default:
		//  ...
	}

	return
}

const (
	SymbolSpring uint64 = 5000 // token symbol 2 char
	SymbolSummer uint64 = 4000 // token symbol 3 char
	SymbolAutumn uint64 = 3000 // token symbol 4 char
	SymbolWinter uint64 = 2000 // token symbol 5 char
)
