package transaction

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/romana/rlog"
	"math/big"
)

type SCData struct {
	Sender       common.Address `json:"from"`
	AccountNonce uint64         `json:"nonce"`
	Price        uint64         `json:"gasPrice"`
	GasLimit     uint64         `json:"gas"`
	Recipient    common.Address `json:"to"`
	Amount       uint64         `json:"value"`
	Payload      []byte         `json:"input"`
	Sig          [64]byte       `json:"sig"`
	Type         uint8          `json:"type"`
}

// enum SCData.Type
const (
	SCDATA_DEFAULT_TYPE uint8 = iota
	SCDATA_DEPOSIT_TYPE
	SCDATA_WITHDRAW_TYPE
)

func (scdata *SCData) Serialize(buf *bytes.Buffer) {
	//TODO:
	var scDataBuf bytes.Buffer
	buffer := make([]byte, binary.MaxVarintLen64)

	scDataBuf.Write(scdata.Sender[:])

	n := binary.PutUvarint(buffer, scdata.AccountNonce)
	scDataBuf.Write(buffer[:n])

	n = binary.PutUvarint(buffer, scdata.Price)
	scDataBuf.Write(buffer[:n])

	n = binary.PutUvarint(buffer, scdata.GasLimit)
	scDataBuf.Write(buffer[:n])

	scDataBuf.Write(scdata.Recipient[:])

	n = binary.PutUvarint(buffer, scdata.Amount)
	scDataBuf.Write(buffer[:n])

	n = binary.PutUvarint(buffer, uint64(len(scdata.Payload)))
	scDataBuf.Write(buffer[:n])
	scDataBuf.Write(scdata.Payload[:])

	scDataBuf.Write(scdata.Sig[:])

	scDataBytes := scDataBuf.Bytes()
	n = binary.PutUvarint(buffer, uint64(len(scDataBytes)))
	buf.Write(buffer[:n])
	buf.Write(scDataBytes)

	n = binary.PutUvarint(buffer, uint64(scdata.Type))
	scDataBuf.Write(buffer[:n])
}

func (scdata *SCData) Deserialize(buf *bytes.Reader) ([]byte, error) {
	//TODO:
	//b := make([]byte, 1)
	//n, err := buf.Read(b)
	//if err != nil {
	//	rlog.Tracef(1, "Extra contract length could not be parsed")
	//	return nil, err
	//}
	//length := int(b[0])
	//rlog.Warnf("length:%v", length)

	length := buf.Size()
	contract := make([]byte, length)
	_, err := buf.Read(contract)
	if err != nil {
		rlog.Tracef(1, "Extra contract could not be parsed ")
		return nil, err
	}

	scdataLength, done := binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid Contract\n")
	}
	contract = contract[done:]
	contract = contract[:scdataLength]

	done = len(scdata.Sender)
	copy(scdata.Sender[:], contract[:done])
	contract = contract[done:]

	scdata.AccountNonce, done = binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid AccountNonce in Contract\n")
	}

	contract = contract[done:]
	scdata.Price, done = binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid Price in Contract\n")
	}

	contract = contract[done:]
	scdata.GasLimit, done = binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid GasLimit in Contract\n")
	}
	contract = contract[done:]

	done = len(scdata.Recipient)
	copy(scdata.Recipient[:], contract[:done])
	contract = contract[done:]

	scdata.Amount, done = binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid Amount in Contract\n")
	}

	contract = contract[done:]
	payloadLen, done := binary.Uvarint(contract)
	if done <= 0 {
		return nil, fmt.Errorf("Invalid Payload in Contract\n")
	}

	contract = contract[done:]
	scdata.Payload = make([]byte, payloadLen)
	copy(scdata.Payload[:], contract[:payloadLen])

	contract = contract[payloadLen:]
	const contractLen = 64
	if len(contract) < contractLen {
		rlog.Warnf("Invalid Contract Length in Contract\n")
		return nil, fmt.Errorf("Invalid Contract Length in Contract\n")
	}
	copy(scdata.Sig[:], contract[:contractLen])
	contract = contract[contractLen:]

	typeU64, done := binary.Uvarint(contract)
	if done != 1 { // sizeof(uint8) == 1
		return nil, fmt.Errorf("Invalid Type in Contract\n")
	}
	scdata.Type = uint8(typeU64)
	contract = contract[done:]

	return nil, nil
}

func contrackBase58Addr(sender string, salt []byte) (string, error) {
	addr, err := address.NewAddress(sender)
	if err != nil {
		return "", err
	}

	key0 := crypto.Keccak256(addr.SpendKey[:], salt)
	key1 := crypto.Keccak256(addr.ViewKey[:], salt)

	addrStr := address.NewAddressFromKeys(addr.Network, crypto.Key(key0), crypto.Key(key1))

	return addrStr.String(), nil
}

type SCMessage struct {
	from       common.Address  `json:"from"`
	to         *common.Address `json:"to"`
	data       []byte          `json:"data"`
	amount     *big.Int        `json:"amount"`
	gasLimit   uint64          `json:"gas"`
	gasPrice   *big.Int        `json:"gasPrice"`
	nonce      uint64          `json:"nonce"`
	sig        [64]byte        `json:"sig"`
	checkNonce bool            `json:"checkNonce"`
}

func AsMessage(scdata *SCData) (*SCMessage, error) {
	var addr *common.Address
	var ZEROSCADDR common.Address

	if scdata.Recipient == ZEROSCADDR {
		addr = nil
	} else {
		addr = new(common.Address)
		*addr = common.DarmaAddressToContractAddress(scdata.Recipient)
	}

	msg := SCMessage{
		from:       common.DarmaAddressToContractAddress(scdata.Sender),
		to:         addr,
		data:       scdata.Payload,
		amount:     new(big.Int).SetUint64(scdata.Amount),
		gasLimit:   scdata.GasLimit,
		gasPrice:   new(big.Int).SetUint64(scdata.Price),
		nonce:      scdata.AccountNonce,
		sig:        scdata.Sig,
		checkNonce: false,
	}

	return &msg, nil
}

func (m SCMessage) From() common.Address { return m.from }
func (m SCMessage) To() *common.Address  { return m.to }
func (m SCMessage) GasPrice() *big.Int   { return m.gasPrice }
func (m SCMessage) Value() *big.Int      { return m.amount }
func (m SCMessage) Gas() uint64          { return m.gasLimit }
func (m SCMessage) Nonce() uint64        { return m.nonce }
func (m SCMessage) Data() []byte         { return m.data }
func (m SCMessage) CheckNonce() bool     { return m.checkNonce }
func (m SCMessage) ToIsEmpty() bool      { return m.to == nil }
