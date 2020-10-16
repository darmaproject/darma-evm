// Copyright 2018-2020 Darma Project. All rights reserved.
package transaction

import (
	"errors"
	"fmt"
	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/romana/rlog"
)

import "github.com/darmaproject/darmasuite/crypto"
import "github.com/darmaproject/darmasuite/ringct"

const TXIN_GEN = byte(0xff)
const TXIN_TO_SCRIPT = byte(0)
const TXIN_TO_SCRIPTHASH = byte(1)
const TXIN_TO_KEY = byte(2)

const TXOUT_TO_SCRIPT = byte(0)
const TXOUT_TO_SCRIPTHASH = byte(1)
const TXOUT_TO_KEY = byte(2)
const TXOUT_TO_REGISTERPOOL = byte(3)
const TXOUT_TO_CLOSEPOOL = byte(4)
const TXOUT_TO_BUYSHARE = byte(5)
const TXOUT_TO_REPOSHARE = byte(6)
const TXOUT_TO_SUBADDRESS = byte(7)

const LOCKEDTYPE_NONE = byte(0)
const LOCKEDTYPE_LOCKED = byte(1)
const LOCKEDTYPE_UNLOCKED = byte(2)

var TX_IN_NAME = map[byte]string{
	TXIN_GEN:           "Coinbase",
	TXIN_TO_SCRIPT:     "To Script",
	TXIN_TO_SCRIPTHASH: "To Script hash",
	TXIN_TO_KEY:        "To key",
}

const TRANSACTION = byte(0xcc)
const BLOCK = byte(0xbb)

/*
VARIANT_TAG(binary_archive, cryptonote::txin_to_script, 0x0);
VARIANT_TAG(binary_archive, cryptonote::txin_to_scripthash, 0x1);
VARIANT_TAG(binary_archive, cryptonote::txin_to_key, 0x2);
VARIANT_TAG(binary_archive, cryptonote::txout_to_script, 0x0);
VARIANT_TAG(binary_archive, cryptonote::txout_to_scripthash, 0x1);
VARIANT_TAG(binary_archive, cryptonote::txout_to_key, 0x2);
VARIANT_TAG(binary_archive, cryptonote::transaction, 0xcc);
VARIANT_TAG(binary_archive, cryptonote::block, 0xbb);
*/
/* outputs */

//Tx extra data for CreateTXv2
type TxCreateExtra struct {
	ContractData *SCData     `json:"contract_data"`
	VoutTarget   interface{} `json:"vout_target"`
	LockedType   uint8       `json:"locked_type"`
	LockedTxId   crypto.Hash `json:"locked_txid"`
}

type Txout_to_script struct {
	// std::vector<crypto::public_key> keys;
	//  std::vector<uint8_t> script;

	Keys   [][32]byte
	Script []byte

	/*  BEGIN_SERIALIZE_OBJECT()
	      FIELD(keys)
	      FIELD(script)
	    END_SERIALIZE()

	*/
}

type Txout_to_scripthash struct {
	//crypto::hash hash;
	Hash [32]byte
}

type TxoutToKey struct {
	Key crypto.Key
	//	Mask [32]byte `json:"-"`
	/*txout_to_key() { }
	  txout_to_key(const crypto::public_key &_key) : key(_key) { }
	  crypto::public_key key;*/
}

type TxoutToSubAddress struct {
	TxoutToKey
	PubKey crypto.Key // txsecret * subaddress spendkey
}

type TxoutToRegisterPool struct {
	TxoutToKey
	Name   string
	Id     crypto.Hash
	Vote   address.Address
	Reward address.Address
	Value  uint64
}

type TxoutToClosePool struct {
	TxoutToKey
	Sign   crypto.Key
	PoolId crypto.Hash
}

type TxoutToBonus struct {
	TxoutToKey
	PoolId crypto.Hash
}

type TxoutToBuyShare struct {
	TxoutToKey
	Value  uint64
	Reward address.Address
	Pool   crypto.Hash
}

type TxoutToRepoShare struct {
	TxoutToKey
	ShareId crypto.Hash
}

// there can be only 4 types if inputs

// used by miner
type Txin_gen struct {
	Height uint64 // stored as varint
}

type Txin_to_script struct {
	Prev    [32]byte
	Prevout uint64
	Sigset  []byte

	/* BEGIN_SERIALIZE_OBJECT()
	     FIELD(prev)
	     VARINT_FIELD(prevout)
	     FIELD(sigset)
	   END_SERIALIZE()
	*/
}

type Txin_to_scripthash struct {
	Prev    [32]byte
	Prevout uint64
	Script  Txout_to_script
	Sigset  []byte

	/* BEGIN_SERIALIZE_OBJECT()
	     FIELD(prev)
	     VARINT_FIELD(prevout)
	     FIELD(script)
	     FIELD(sigset)
	   END_SERIALIZE()
	*/
}

type TxinToKey struct {
	Amount     uint64
	KeyOffsets []uint64 // this is encoded as a varint for length and then all offsets are stored as varint
	//crypto::key_image k_image;      // double spending protection
	K_image crypto.Hash `json:"k_image"` // key image

	/* BEGIN_SERIALIZE_OBJECT()
	     VARINT_FIELD(amount)
	     FIELD(key_offsets)
	     FIELD(k_image)
	   END_SERIALIZE()
	*/
}

type Txin_v interface{} // it can only be txin_gen, txin_to_script, txin_to_scripthash, txin_to_key

type TransactionLocked struct {
	LockType uint8  `json:"locktype"`
	Extra    []byte `json:"extra"` // 255 bytes at most
}

type TxOut struct {
	Amount uint64
	Target interface{} // txout_target_v ;, it can only be  txout_to_script, txout_to_scripthash, txout_to_key

	/* BEGIN_SERIALIZE_OBJECT()
	     VARINT_FIELD(amount)
	     FIELD(target)
	   END_SERIALIZE()
	*/

}

// the core transaction
type TransactionPrefix struct {
	Version      uint64 `json:"version"`
	UnlockTime   uint64 `json:"unlock_time"` // used to lock first output
	Vin          []Txin_v
	Vout         []TxOut
	Extra        []byte
	ExtraMap     map[EXTRA_TAG]interface{} `json:"-"` // all information parsed from extra is placed here
	PaymentIDMap map[EXTRA_TAG]interface{} `json:"-"` // payments id parsed or set are placed her
	ExtraType    byte                      `json:"-"` // NOT used, candidate for deletion
}

type Transaction struct {
	TransactionPrefix
	// same as TransactionPrefix
	// Signature  not sure of what form
	Signature []Signature_v1 `json:"-"` // old format, the array size is always equal to vin length,
	//Signature_RCT RCT_Signature  // version 2

	RctSignature *ringct.RctSig
	Expanded     bool `json:"-"`
}

func (vout *TxOut) GetKey() crypto.Key {
	switch vout.Target.(type) {
	case TxoutToKey:
		return vout.Target.(TxoutToKey).Key
	case TxoutToRegisterPool:
		return vout.Target.(TxoutToRegisterPool).Key
	case TxoutToClosePool:
		return vout.Target.(TxoutToClosePool).Key
	case TxoutToBuyShare:
		return vout.Target.(TxoutToBuyShare).Key
	case TxoutToRepoShare:
		return vout.Target.(TxoutToRepoShare).Key
	case TxoutToSubAddress:
		return vout.Target.(TxoutToSubAddress).Key
	default:
		panic(fmt.Sprintf("invalid vout %+v", vout))
	}
}

func (tx *Transaction) GetHash() (result crypto.Hash) {
	switch tx.Version {

	/*case 1:
	result = crypto.Hash(crypto.Keccak256(tx.SerializeHeader()))
	*/

	case config.TX_VERSION_NORMAL, config.TX_VERSION_LOCKED:
		// version 2 requires first computing 3 separate hashes
		// prefix, rctBase and rctPrunable
		// and then hashing the hashes together to get the final hash
		prefixHash := tx.GetPrefixHash()
		rctBaseHash := tx.RctSignature.BaseHash()
		rctPrunableHash := tx.RctSignature.PrunableHash()
		result = crypto.Hash(crypto.Keccak256(prefixHash[:], rctBaseHash[:], rctPrunableHash[:]))
	default:
		panic("Transaction version unknown")

	}

	return
}

func (tx *Transaction) GetPrefixHash() (result crypto.Hash) {
	result = crypto.Keccak256(tx.SerializeHeader())
	return result
}

// returns whether the tx is coinbase
func (tx *Transaction) IsCoinbase() (result bool) {
	switch tx.Vin[0].(type) {
	case Txin_gen:
		return true
	default:
		return false
	}
}

// calculated prefi has signature
func (tx *Transaction) Clear() {
	// clean the transaction everything
	tx.Version = 0
	tx.UnlockTime = 0
	tx.Vin = tx.Vin[:0]
	tx.Vout = tx.Vout[:0]
	tx.Extra = tx.Extra[:0]
}

func (tx *Transaction) GetLockedExtra() (*TransactionLocked, error) {
	if _, ok := tx.ExtraMap[TX_EXTRA_LOCKED]; ok {
		return tx.ExtraMap[TX_EXTRA_LOCKED].(*TransactionLocked), nil
	}

	return nil, fmt.Errorf("TX_EXTRA_LOCKED is not present.")
}

func (tx *Transaction) GetLockedType() uint8 {
	txLocked, err := tx.GetLockedExtra()
	if err != nil {
		return LOCKEDTYPE_NONE
	}
	return txLocked.LockType
}

func (tx *Transaction) IsLocked() bool {
	txLocked, err := tx.GetLockedExtra()
	if err != nil {
		return false
	}
	if txLocked.LockType != LOCKEDTYPE_LOCKED {
		return false
	}

	return true
}

func (tx *Transaction) IsUnlocked() bool {
	txLocked, err := tx.GetLockedExtra()
	if err != nil {
		return false
	}
	if txLocked.LockType != LOCKEDTYPE_UNLOCKED {
		return false
	}

	return true
}

func (tx *Transaction) CheckLockedExtra() (bool, error) {
	txLocked, err := tx.GetLockedExtra()
	if err != nil {
		//No extra
		return true, nil
	}

	if len(tx.Vout) != 1 {
		err = fmt.Errorf("Lock/Unlock transaction can only have one destination")
		return false, err
	}

	if len(tx.Vin) == 0 {
		err = fmt.Errorf("Locking Miner tx is not supported")
		return false, err
	}

	if txLocked.LockType == LOCKEDTYPE_UNLOCKED {
		//TODO:
	}

	return true, nil
}

func (tx *Transaction) IsCreateContract() bool {
	if tx.IsContract() == false {
		return false
	}
	scData := tx.ExtraMap[TX_EXTRA_CONTRACT].(*SCData)
	var ZEROSCADDR common.Address
	if scData.Recipient != ZEROSCADDR {
		return false
	}
	return true
}

func (tx *Transaction) IsContractDW() bool {
	if tx.IsContract() == false {
		return false
	}
	scData := tx.ExtraMap[TX_EXTRA_CONTRACT].(*SCData)
	return scData.Type == SCDATA_DEPOSIT_TYPE || scData.Type == SCDATA_WITHDRAW_TYPE
}

func (tx *Transaction) IsContract() bool {
	if tx.ExtraMap[TX_EXTRA_CONTRACT] == nil {
		return false
	}

	return true
}

func (tx *Transaction) GetFeeForContract(hardForkVersion int64) uint64 {
	if hardForkVersion < config.CONTRACT_FORK_VERSION {
		return 0
	}
	if !tx.IsContract() {
		return 0
	}
	scData := tx.ExtraMap[TX_EXTRA_CONTRACT].(*SCData)

	return scData.GasLimit * scData.Price
}

/*

func (tx *Transaction) IsCoinbase() (result bool){

  // check whether the type is Txin.get

   if len(tx.Vin) != 0 { // coinbase transactions have no vin
    return
   }

   if tx.Vout[0].(Target) != 0 { // coinbase transactions have no vin
    return
   }


}*/

func (tx *Transaction) OmniToken() (result *OmniToken, err error) {
	extraParsed := tx.ParseExtra()

	omniToken := &OmniToken{}
	if extraParsed {
		if _, ok := tx.ExtraMap[OMNI_TOKEN]; ok {
			omniToken = tx.ExtraMap[OMNI_TOKEN].(*OmniToken)

			return omniToken, nil
		}

		return nil, errors.New("no OMNI_TOKEN")
	}

	return nil, errors.New("extraParsed err")
}

func (tx *Transaction) TokenTx() (result *Transaction, err error) {
	extraParsed := tx.ParseExtra()

	var tokenTx Transaction
	if extraParsed {
		if _, ok := tx.ExtraMap[TOKEN_TX]; ok {
			txBytes := tx.ExtraMap[TOKEN_TX].([]byte)
			err := tokenTx.DeserializeHeader(txBytes)
			if err != nil {
				rlog.Debugf("Token TX could NOT be deserialized")
				return nil, err
			}

			return &tokenTx, nil
		}

		return nil, errors.New("no TOKEN_TX")
	}

	return nil, errors.New("extraParsed err")
}
