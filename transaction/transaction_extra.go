// Copyright 2018-2020 Darma Project. All rights reserved.
package transaction

//import "fmt"
import (
	"bytes"
	"encoding/binary"
)

//import "runtime/debug"

//import "encoding/binary"

import "github.com/romana/rlog"

import "github.com/darmaproject/darmasuite/crypto"

// refer https://cryptonote.org/cns/cns005.txt to understand slightly more ( it DOES NOT cover everything)
// much of these constants are understood from tx_extra.h and cryptonote_format_utils.cpp
// TODO pending test case
type EXTRA_TAG byte

const TX_EXTRA_PADDING EXTRA_TAG = 0 // followed by 1 byte of size, and then upto 255 bytes of padding
const TX_PUBLIC_KEY EXTRA_TAG = 1    // follwed by 32 bytes of tx public key
const TX_EXTRA_NONCE EXTRA_TAG = 2   // followed by 1 byte of size, and then upto 255 bytes of empty nonce
const TX_EXTRA_LOCKED EXTRA_TAG = 3  // followed by 1 byte of size, and then upto 255 bytes of padding
const TX_PRIVATE_KEY EXTRA_TAG = 4
const OMNI_TOKEN EXTRA_TAG = 7
const TOKEN_TX EXTRA_TAG = 8
const TX_EXTRA_CONTRACT EXTRA_TAG = 10

// TX_EXTRA_MERGE_MINING_TAG  we do NOT suppport merged mining at all
// TX_EXTRA_MYSTERIOUS_MINERGATE_TAG  as the name says mysterious we will not bring it

// these 2 fields have complicated parsing of extra, other the code was really simple
const TX_EXTRA_NONCE_PAYMENT_ID EXTRA_TAG = 0           // extra nonce within a non coinbase tx, can be unencrypted, is 32 bytes in size
const TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID EXTRA_TAG = 1 // this is encrypted and is 9 bytes in size

type ExtraPublicKey struct {
	Pubkey crypto.Key
	Vout   int
}

// the field is just named extra and contains CRITICAL information, though some is optional
// parse extra data such as
// tx public key must
// payment id optional
// encrypted payment id optional
func (tx *Transaction) ParseExtra() (result bool) {
	var err error
	var length int

	if len(tx.ExtraMap) > 0 {
		return true
	}

	/*defer func (){
		if r := recover(); r != nil {
				fmt.Printf("Recovered while parsing extra, Stack trace below block_hash %s", tx.GetHash())
				fmt.Printf("Stack trace  \n%s", debug.Stack())
				result = false
			}
	        }()*/

	buf := bytes.NewReader(tx.Extra)

	tx.ExtraMap = map[EXTRA_TAG]interface{}{}
	tx.PaymentIDMap = map[EXTRA_TAG]interface{}{}

	b := make([]byte, 1)
	var n int
	for i := 0; ; i++ {
		if buf.Len() == 0 {
			return true
		}
		n, err = buf.Read(b)
		//if err != nil { // we make that the buffer has atleast 1 byte to read
		//	return false
		//}

		switch EXTRA_TAG(b[0]) {
		case TX_EXTRA_PADDING: // this is followed by 1 byte length, then length bytes of padding
			n, err = buf.Read(b)
			if err != nil {
				rlog.Tracef(1, "Extra padding length could not be parsed")
				return false
			}
			length = int(b[0])
			padding := make([]byte, length, length)
			n, err = buf.Read(padding)
			if err != nil || n != int(length) {
				rlog.Tracef(1, "Extra padding could not be read  ")
				return false
			}

			// Padding is not added to the extra map

		case TX_PUBLIC_KEY: // next 32 bytes are tx public key
			var pkey crypto.Key
			n, err = buf.Read(pkey[:])
			if err != nil || n != 32 {
				rlog.Tracef(1, "Tx public key could not be parsed len=%d err=%s ", n, err)
				return false
			}
			tx.ExtraMap[TX_PUBLIC_KEY] = pkey

		case TX_EXTRA_NONCE: // this is followed by 1 byte length, then length bytes of data
			n, err = buf.Read(b)
			if err != nil {
				rlog.Tracef(1, "Extra nonce length could not be parsed ")
				return false
			}

			length = int(b[0])

			extra_nonce := make([]byte, length, length)
			n, err = buf.Read(extra_nonce)
			if err != nil || n != int(length) {
				rlog.Tracef(1, "Extra Nonce could not be read ")
				return false
			}

			switch length {
			case 33: // unencrypted 32 byte  payment id
				if extra_nonce[0] == byte(TX_EXTRA_NONCE_PAYMENT_ID) {
					tx.PaymentIDMap[TX_EXTRA_NONCE_PAYMENT_ID] = extra_nonce[1:]
				} else {
					rlog.Tracef(1, "Extra Nonce contains invalid payment id ")
					return false
				}

			case 9: // encrypted 9 byte payment id
				if extra_nonce[0] == byte(TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID) {
					tx.PaymentIDMap[TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID] = extra_nonce[1:]
				} else {
					rlog.Tracef(1, "Extra Nonce contains invalid encrypted payment id ")
					return false
				}

			default: // consider it as general nonce
				// ignore anything else
			}

			tx.ExtraMap[TX_EXTRA_NONCE] = extra_nonce

		case TX_EXTRA_LOCKED:
			n, err = buf.Read(b)
			if err != nil {
				rlog.Tracef(1, "Extra locked length could not be parsed ")
				return false
			}

			length = int(b[0])
			extra_data := make([]byte, length, length)
			n, err = buf.Read(extra_data)
			if err != nil || n != int(length) {
				rlog.Tracef(1, "Extra locked could not be read ")
				return false
			}

			tx.ExtraMap[TX_EXTRA_LOCKED] = &TransactionLocked{
				LockType: extra_data[0],
				Extra:    extra_data[1:],
			}

			//txLocked := tx.ExtraMap[TX_EXTRA_LOCKED].(*TransactionLocked)
			//rlog.Debugf("Parsed locked tx %s: %+v", tx.GetHash(), txLocked)

		case TX_PRIVATE_KEY: // next 32 bytes are tx public key
			var pkey crypto.Key
			n, err = buf.Read(pkey[:])
			if err != nil || n != 32 {
				rlog.Tracef(1, "Tx private key could not be parsed len=%d err=%s ", n, err)
				return false
			}
			tx.ExtraMap[TX_PRIVATE_KEY] = pkey

		case OMNI_TOKEN:
			var token_data_len uint64
			token_data_len, err = binary.ReadUvarint(buf)
			if err != nil || token_data_len == 0 {
				rlog.Debugf("Extra OMNI Token could not be read ")
				return false
			}

			if token_data_len > 128*1024 { // stop ddos right now
				rlog.Debugf("Extra OMNI Token attempting Ddos, stopping attack ")
				return false
			}

			data_bytes := make([]byte, token_data_len, token_data_len)
			n, err = buf.Read(data_bytes)
			if err != nil || n != int(token_data_len) {
				rlog.Debugf("Extra OMNI Token could not be read, err %s n %d token_data_len %d ", err, n, token_data_len)
				return false
			}
			omniToken := &OmniToken{}
			omniToken.Deserialize(data_bytes)
			tx.ExtraMap[OMNI_TOKEN] = omniToken

		case TOKEN_TX:
			var token_tx_len uint64
			token_tx_len, err = binary.ReadUvarint(buf)
			if err != nil || token_tx_len == 0 {
				rlog.Debugf("Extra Token Data could not be read ")
				return false
			}

			if token_tx_len > 128*1024 { // stop ddos right now
				rlog.Debugf("Extra Token Data attempting Ddos, stopping attack ")
				return false
			}

			data_bytes := make([]byte, token_tx_len, token_tx_len)
			n, err = buf.Read(data_bytes)
			if err != nil || n != int(token_tx_len) {
				rlog.Debugf("Extra Token Data could not be read, err %s n %d token_data_len %d ", err, n, token_tx_len)
				return false
			}
			tx.ExtraMap[TOKEN_TX] = data_bytes

		case TX_EXTRA_CONTRACT:
			scdata := new(SCData)
			_, err := scdata.Deserialize(buf)
			if err != nil {
				rlog.Tracef(1, "Extra contract could not be read: err= %s",err.Error())
				return false
			}
			tx.ExtraMap[TX_EXTRA_CONTRACT] = scdata

		default: // any any other unknown tag or data, fails the parsing
			rlog.Tracef(1, "Unhandled TAG %d \n", b[0])
			result = false
			return
		}
	}

	// we should not reach here
	//return true
}

// serialize an extra, this is only required while creating new transactions ( both miner and normal)
// doing this on existing transaction will cause them to fail ( due to different placement order )
func (tx *Transaction) SerializeExtra() []byte {
	buf := bytes.NewBuffer(nil)

	// this is mandatory
	if _, ok := tx.ExtraMap[TX_PUBLIC_KEY]; ok {
		buf.WriteByte(byte(TX_PUBLIC_KEY)) // write marker
		key := tx.ExtraMap[TX_PUBLIC_KEY].(crypto.Key)
		buf.Write(key[:]) // write the key
	} else {
		rlog.Tracef(1, "TX does not contain a Public Key, not possible, the transaction will be rejected")
		return buf.Bytes() // as keys are not provided, no point adding other fields
	}

	// extra nonce should be serialized only if other nonce are not provided, tx should contain max 1 nonce
	// it can be either, extra nonce, 32 byte payment id or 8 byte encrypted payment id

	// if payment id are set, they replace nonce
	// first place unencrypted payment id
	if _, ok := tx.PaymentIDMap[TX_EXTRA_NONCE_PAYMENT_ID]; ok {
		data_bytes := tx.PaymentIDMap[TX_EXTRA_NONCE_PAYMENT_ID].([]byte)
		if len(data_bytes) == 32 { // payment id is valid
			header := append([]byte{byte(TX_EXTRA_NONCE_PAYMENT_ID)}, data_bytes...)
			tx.ExtraMap[TX_EXTRA_NONCE] = header // overwrite extra nonce with this
		}
		rlog.Tracef(1, "unencrypted payment id size mismatch expected = %d actual %d", 32, len(data_bytes))
	}

	// if encrypted nonce is provide, it will overwrite 32 byte nonce
	if _, ok := tx.PaymentIDMap[TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID]; ok {
		data_bytes := tx.PaymentIDMap[TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID].([]byte)
		if len(data_bytes) == 8 { // payment id is valid
			header := append([]byte{byte(TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID)}, data_bytes...)
			tx.ExtraMap[TX_EXTRA_NONCE] = header // overwrite extra nonce with this
		}
		rlog.Tracef(1, "unencrypted payment id size mismatch expected = %d actual %d", 8, len(data_bytes))
	}

	// TX_EXTRA_NONCE is optional
	// if payment is present, it is packed as extra nonce
	if _, ok := tx.ExtraMap[TX_EXTRA_NONCE]; ok {
		buf.WriteByte(byte(TX_EXTRA_NONCE)) // write marker
		data_bytes := tx.ExtraMap[TX_EXTRA_NONCE].([]byte)

		if len(data_bytes) > 255 {
			rlog.Tracef(1, "TX extra none is spilling, trimming the nonce to 254 bytes")
			data_bytes = data_bytes[:254]
		}
		buf.WriteByte(byte(len(data_bytes))) // write length of extra nonce single byte
		buf.Write(data_bytes[:])             // write the nonce data
	}

	// TX_EXTRA_LOCKED is optional
	// if locked is present, it is packed as extra locked
	if _, ok := tx.ExtraMap[TX_EXTRA_LOCKED]; ok {
		buf.WriteByte(byte(TX_EXTRA_LOCKED)) // write marker

		txLocked := tx.ExtraMap[TX_EXTRA_LOCKED].(*TransactionLocked)
		data_len := 1 + len(txLocked.Extra)
		buf.WriteByte(byte(data_len)) // write length of extra nonce single byte

		buf.WriteByte(byte(txLocked.LockType))
		buf.Write(txLocked.Extra[:])
	}
	// NOTE: we do not support adding padding for the sake of it

	if _, ok := tx.ExtraMap[TX_PRIVATE_KEY]; ok {
		buf.WriteByte(byte(TX_PRIVATE_KEY)) // write marker
		key := tx.ExtraMap[TX_PRIVATE_KEY].(crypto.Key)
		buf.Write(key[:]) // write the key
	}

	if _, ok := tx.ExtraMap[OMNI_TOKEN]; ok {
		buf.WriteByte(byte(OMNI_TOKEN)) // write marker
		omniToken := tx.ExtraMap[OMNI_TOKEN].(*OmniToken)
		data_bytes := omniToken.Serialize() // tx.ExtraMap[OMNI_TOKEN].([]byte)

		tbuf := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(tbuf, uint64(len(data_bytes)))
		buf.Write(tbuf[:n])

		buf.Write(data_bytes[:])
	}

	if _, ok := tx.ExtraMap[TOKEN_TX]; ok {
		buf.WriteByte(byte(TOKEN_TX)) // write marker
		tokenTxBytes := tx.ExtraMap[TOKEN_TX].([]byte)

		tbuf := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(tbuf, uint64(len(tokenTxBytes)))
		buf.Write(tbuf[:n])
		//fmt.Printf("serializing extra len %x\n", tbuf[:n])
		//fmt.Printf("serializing extra data %x\n", tokenTxBytes)
		buf.Write(tokenTxBytes[:]) // write the data bytes
	}

	if _, ok := tx.ExtraMap[TX_EXTRA_CONTRACT]; ok {
		buf.WriteByte(byte(TX_EXTRA_CONTRACT)) // write marker

		scdata := tx.ExtraMap[TX_EXTRA_CONTRACT].(*SCData)
		scdata.Serialize(buf)
	}
	return buf.Bytes()

}

// resize the nonce by this much bytes,
// positive means add  byte
// negative means decrease size
// this is only required during miner tx to solve chicken and problem
/*
func (tx *Transaction) Resize_Extra_Nonce(int resize_amount) {

    nonce_bytes := tx.Extra_map[TX_EXTRA_NONCE].([]byte)
    nonce_bytes = make([]byte, len(nonce_bytes)+resize_amount, len(nonce_bytes)+resize_amount)
    tx.Extra_map[TX_EXTRA_NONCE] = nonce_bytes

    tx.Extra = tx.Serialize_Extra()

}
*/
