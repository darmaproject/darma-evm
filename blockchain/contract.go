package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/block"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core"
	"github.com/darmaproject/darmasuite/dvm/core/state"
	"github.com/darmaproject/darmasuite/dvm/core/types"
	"github.com/darmaproject/darmasuite/dvm/core/rawdb"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/ringct"
	"github.com/darmaproject/darmasuite/storage"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
	"github.com/vmihailenco/msgpack"
	"math/big"
)

var (
	// ErrInvalidSender is returned if the transaction contains an invalid sender address.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrInvalidSigner is returned if the transaction contains an invalid signature.
	ErrInvalidSigner = errors.New("invalid signature")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrTooManyVout is returned if the transaction contains too many vouts.
	ErrTooManyVout = errors.New("too many vout")

	// ErrContractNotFound is returned if the contract address not found from database.
	ErrContractNotFound = errors.New("contract not found")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")
)

type SCTransferE struct {
	Address string `msgpack:"A,omitempty" json:"A,omitempty"` //  transfer to this blob
	Amount  uint64 `msgpack:"V,omitempty" json:"V,omitempty"` // Amount in Atomic units
}

type SCStorage struct {
	SCID      crypto.Key    `msgpack:"S,omitempty"`
	TransferE []SCTransferE `msgpack:"T,omitempty"`
}

const INVALID_CHAIN_HEIGHT = 0x7fffffffffffffff

func (chain *Blockchain) IsCreateContract(tx *transaction.Transaction) bool {
	if tx.IsContract() == false {
		return false
	}
	scData := tx.ExtraMap[transaction.TX_EXTRA_CONTRACT].(*transaction.SCData)
	var ZEROSCADDR common.Address
	if scData.Recipient != ZEROSCADDR {
		return false
	}
	return true
}

func (chain *Blockchain) IsContractTransaction(tx *transaction.Transaction) bool {
	if tx.ExtraMap[transaction.TX_EXTRA_CONTRACT] == nil {
		return false
	}

	return true
}

func (chain *Blockchain) VerifyTransactionContract(dbtx storage.DBTX, tx *transaction.Transaction) error {
	createContract := tx.IsCreateContract()
	rlog.Infof("--------start VerifyTransactionContract------------------%s", tx.GetHash().String())
	if len(tx.Vout) > 2 {
		return ErrTooManyVout
	}

	if tx.IsContractDW() {
		// TODO: Do not verify DW contract tx ?
		return nil
	}

	scData := tx.ExtraMap[transaction.TX_EXTRA_CONTRACT].(*transaction.SCData)

	if createContract && len(scData.Payload) < int(config.MIN_CONTRACT_DATASIZE) {
		return ErrOversizedData
	}

	if scData.GasLimit < config.MIN_GASLIMIT {
		return ErrGasLimit
	}

	if scData.Price < config.MIN_GASPRICE {
		return ErrUnderpriced
	}

	// Ensure the transaction has more gas than the basic tx fee.
	intrGas, err := dvm.IntrinsicGas(scData.Payload, createContract)
	if err != nil {
		return err
	}
	if scData.GasLimit < intrGas {
		rlog.Error("limit ", scData.GasLimit, " ", intrGas)
		return ErrIntrinsicGas
	}

	if !createContract {
		_, err := chain.LoadContractTxidByAddress(dbtx, scData.Recipient)
		if err != nil {
			rlog.Errorf("no such contract %s", scData.Recipient.String())
			return ErrContractNotFound
		}
	}

	if crypto.VerifySign(scData.Payload, crypto.Key(scData.Sender), scData.Sig) == false {
		return ErrInvalidSigner
	}

	rlog.Infof("--------end VerifyTransactionContract------------------")
	return nil
}

func (chain *Blockchain) DecodeContractAmount(tx *transaction.Transaction) (uint64, error) {
	spendSecret, viewSecret := crypto.ZeroKeys()

	outputIndex := uint64(0)
	public_spend := spendSecret.PublicKey()
	tx_public_key := tx.ExtraMap[transaction.TX_PUBLIC_KEY].(crypto.Key)
	ehphermal_public_key := tx.Vout[outputIndex].GetKey()

	derivation := crypto.KeyDerivation(&tx_public_key, &viewSecret)
	derivation_public_key := derivation.KeyDerivationToPublicKey(outputIndex, *public_spend)
	rlog.Error("tx_public_key", tx_public_key, "sceret_view", viewSecret)
	rlog.Error("derivation_public_key", derivation_public_key, "outputIndex", outputIndex, "public_spend", *public_spend)
	if ehphermal_public_key != derivation_public_key {
		return 0, fmt.Errorf("ehphermal public key not matched '%s'.", ehphermal_public_key.String())
	}

	scalar_key := derivation.KeyDerivationToScalar(outputIndex)

	tuple := tx.RctSignature.ECdhInfo[outputIndex]

	mask := tx.RctSignature.OutPk[outputIndex].Mask
	amount, _, result := ringct.Decode_Amount(tuple, *scalar_key, mask)
	if result == false {
		return 0, fmt.Errorf("RingCT decode amount failed from contract transaction.")
	}

	return amount, nil
}

func (chain *Blockchain) loadContractTransfer(dbtx storage.DBTX, txid crypto.Hash) (*SCStorage, error) {
	sctxBlob, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_CONTRACT_TRANSFER_BLOB)
	if err != nil {
		return nil, err
	}

	if len(sctxBlob) == 0 {
		return nil, fmt.Errorf("no sc transfer")
	}

	var sctxData SCStorage
	err = msgpack.Unmarshal(sctxBlob, &sctxData)
	if err != nil {
		return nil, err
	}

	return &sctxData, nil
}

func (chain *Blockchain) loadErc20Transfers(dbtx storage.DBTX, txid crypto.Hash) ([]globals.Erc20Transfer, error) {
	value, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_TOKEN_TRANSFER_BLOB)
	if err != nil {
		return nil, err
	}

	var transfers []globals.Erc20Transfer
	err = msgpack.Unmarshal(value, &transfers)
	if err != nil {
		return nil, err
	}

	return transfers, nil
}

func (chain *Blockchain) storeErc20Transfers(dbtx storage.DBTX, txid crypto.Hash, transfer []globals.Erc20Transfer) {
	blob, _ := msgpack.Marshal(transfer)
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_TOKEN_TRANSFER_BLOB, blob)
}

func (chain *Blockchain) storeContractTransfer(dbtx storage.DBTX, txid crypto.Hash, sctxData *SCStorage) {
	sctxBlob, _ := msgpack.Marshal(sctxData)
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_CONTRACT_TRANSFER_BLOB, sctxBlob)
}

func (chain *Blockchain) createContrackBase58Addr(sender string, txid crypto.Hash) (string, error) {
	addr, err := address.NewAddress(sender)
	if err != nil {
		return "", err
	}

	key0 := crypto.Keccak256(addr.SpendKey[:], txid[:])
	key1 := crypto.Keccak256(addr.ViewKey[:], txid[:])

	addrStr := address.NewAddressFromKeys(addr.Network, crypto.Key(key0), crypto.Key(key1))

	return addrStr.String(), nil
}

func (chain *Blockchain) ApplyContract(dbtx storage.DBTX,
	statedb *state.StateDB,
	bl *block.Block,
	tx *transaction.Transaction,
	blid crypto.Hash,
	txHash crypto.Hash,
	topoHeight int64) error {

	if tx.IsContract() == false {
		//return fmt.Errorf("not contract transaction.")
		return nil
	}

	rlog.Info("---ApplyContract---")

	scdata := tx.ExtraMap[transaction.TX_EXTRA_CONTRACT].(*transaction.SCData)

	msg, err := transaction.AsMessage(scdata)
	if err != nil {
		return err
	}
	origin := msg.From()
	if !msg.ToIsEmpty() {
		origin, err = chain.loadContractOrigin(dbtx, msg.To().Bytes())
	}

	statedb.Prepare(common.Hash(txHash), common.Hash(blid), 0)
	blockGaslimit := chain.GetBlockGaslimit()
	gp := new(dvm.GasPool).AddGas(blockGaslimit)

	header := &types.Header{
		Number:     new(big.Int).SetInt64(topoHeight),
		Difficulty: new(big.Int).Set(chain.LoadBlockDifficulty(dbtx, blid)),
		Time:       bl.BlockHeader.Timestamp,
		GasLimit:   blockGaslimit,
	}

	// Create a new context to be used in the VM environment
	context := dvm.NewVMContext(msg, header, origin, chain.GetHashFn(dbtx), chain.GetAddrStrToBytesFn(), chain.GetBytesToAddrStrFn())
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	/*	var (
		gas    uint64
		failed bool
		origin common.Address
	)*/

	vmenv := dvm.GetVM(msg, context, statedb, dvm.GetChainCOnfig(), dvm.GetVMConfig())
	if vmenv == nil {
		return fmt.Errorf("failed to call contract!")
	}

	if scdata.Type == transaction.SCDATA_DEPOSIT_TYPE || scdata.Type == transaction.SCDATA_WITHDRAW_TYPE { // if tx is type of DEPOSIT or WITHDRAW
		caller := msg.From()
		switch scdata.Type {
		case transaction.SCDATA_DEPOSIT_TYPE:
			if scdata.Amount > 0 {
				amount, err := chain.DecodeContractAmount(tx)
				if err != nil {
					return fmt.Errorf("decode amount from tx failed: %s", err.Error())
				}
				if scdata.Amount != amount {
					return fmt.Errorf("amount not matched. scdata.Amount = %d, amount = %d", scdata.Amount, amount)
				}
			}
			amount := msg.Value()
			rlog.Debugf("address %x deposit %s into VM",caller,amount.String())
			err = vmenv.Deposit(caller,*amount)
		case transaction.SCDATA_WITHDRAW_TYPE:
			bytesAmount := msg.Data()
			uintAmount := bytesAmountToUintAmount(bytesAmount)
			amount := new(big.Int).SetUint64(uintAmount)
			rlog.Debugf("address %x withdraw %s from VM",caller,amount.String())
			err = vmenv.Withdraw(caller,*amount)
			if err == nil {
				//: create new UTXO in blockchain
				var sctxData SCStorage
				sctxData.TransferE = append(sctxData.TransferE, SCTransferE{scdata.Sender.String(), uintAmount}) // sender is Darma address format, caller is contract address format
				chain.storeContractTransfer(dbtx, txHash, &sctxData)
			}
		}
		return err
	}

	ret, gasUsed, contractAddr, err := dvm.ApplyMessage(vmenv, msg, gp)

	if msg.To() != nil {
		rlog.Infof("statedb.GetBalance(%x): %s", *msg.To(), statedb.GetBalance(*msg.To()))
	}

	rlog.Infof("ApplyMessage ret: %x", ret)
	if err != nil {
		return err
	}

	chain.storeContractTxResult(dbtx, txHash, ret)
	if tx.IsCreateContract() {
		chain.StoreContractAddress(dbtx, txHash, contractAddr)
		chain.storeContractOrigin(dbtx, contractAddr, msg.From())
	}

	totalGasSupply := msg.Gas()
	if totalGasSupply > gasUsed {
		remainingGas := totalGasSupply - gasUsed
		price := msg.GasPrice()
		totalValue := new(big.Int).Mul(new(big.Int).SetUint64(totalGasSupply), price)
		remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), msg.GasPrice())
		statedb.AddBalance(msg.From(),remainingValue)
		rlog.Debugf("gas total supply %d(value:%s), used %d, remain %d(value:%s), tx= %s", totalGasSupply, totalValue.String(), gasUsed, remainingGas, remainingValue.String(), txHash)
	}

	logs := statedb.GetLogs(common.Hash(txHash))
	var transfers []globals.Erc20Transfer
	for _, v := range logs {
		if len(v.Topics) == 3 {
			// erc20 transfer and approval event id
			// approval: 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925
			if v.Topics[0] == common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") {
				from := v.Topics[1]
				to := v.Topics[2]
				amount := bytes.TrimLeft(v.Data, "\x00")
				transfers = append(transfers, globals.Erc20Transfer{
					Contract: v.Address,
					From:     common.BytesToAddress(from[:]),
					To:       common.BytesToAddress(to[:]),
					Amount:   new(big.Int).SetBytes(amount).String(),
				})
			}
		}
	}
	if len(transfers) > 0 {
		chain.storeErc20Transfers(dbtx, txHash, transfers)
	}

	rlog.Debugf("Apply contact success, contract address %x", contractAddr)
	return nil
}

func bytesAmountToUintAmount(amount []byte) uint64 {
	return binary.BigEndian.Uint64(amount)
}

func (chain *Blockchain) CallContact(scdata *transaction.SCData, topoHeight int64) ([]byte, error) {
	dbtx, err := chain.store.BeginTX(false)
	defer dbtx.Rollback()

	blockGaslimit := chain.GetBlockGaslimit()
	gp := new(dvm.GasPool).AddGas(blockGaslimit)

	if topoHeight <= 0 {
		topoHeight = chain.LoadTopoHeight(dbtx)
	}

	hash, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, topoHeight)
	if err != nil {
		logger.Warnf("Errr could not find topo index of previous block")
		return nil, globals.ErrInvalidBlock
	}
	chain.LoadBlockTimestamp(dbtx, hash)
	bl, err := chain.LoadBlFromId(dbtx, hash)
	if err != nil {
		logger.Warnf("Errr could not find topo index of previous block")
		return nil, globals.ErrInvalidBlock
	}

	header := &types.Header{
		Number:     new(big.Int).SetInt64(chain.LoadHeightForBlId(dbtx, hash)),
		Difficulty: new(big.Int).Set(chain.LoadBlockDifficulty(dbtx, hash)),
		Time:       bl.BlockHeader.Timestamp,
		GasLimit:   blockGaslimit,
	}

	statedb, err := chain.NewStateDB(dbtx, topoHeight)
	msg, err := transaction.AsMessage(scdata)
	if err != nil {
		return nil, err
	}

	origin, err := chain.loadContractOrigin(dbtx, msg.To().Bytes())
	if err != nil {
		return nil, fmt.Errorf("no origin, contract %x, err %s", msg.To(), err)
	}

	ctx := dvm.NewVMContext(msg, header, origin, chain.GetHashFn(dbtx), chain.GetAddrStrToBytesFn(), chain.GetBytesToAddrStrFn())
	vmenv := dvm.GetVM(msg, ctx, statedb, dvm.GetChainCOnfig(), dvm.GetVMConfig())
	if vmenv == nil {
		return nil, fmt.Errorf("failed to call contract!")
	}

	res, _, _, err := dvm.ApplyMessage(vmenv, msg, gp)
	rlog.Infof("ApplyMessage res: %x", res)
	statedb.Finalise(true)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (chain *Blockchain) GetAddrStrToBytesFn() func(addStr string) []byte {
	return func(addStr string) []byte {
		addr, _ := address.NewAddress(addStr)
		return addr.SpendKey[:]
	}
}

func (chain *Blockchain) GetBytesToAddrStrFn() func(bytes []byte) string {
	return func(bytes []byte) string {
		var addr address.Address
		addr.Network = globals.GetNetwork()
		copy(addr.SpendKey[:], bytes)
		// fixme: view key
		return addr.String()
	}
}

func (chain *Blockchain) revertContract(dbtx storage.DBTX, bl *block.Block, blid crypto.Hash) error {
	return chain.RemoveStateRoot(dbtx, blid)
}

func (chain *Blockchain) GetHashFn(dbtx storage.DBTX) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		hash, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, int64(n))
		if err != nil {
			return common.Hash{}
		}

		return common.Hash(hash)
	}
}

func (chain *Blockchain) storeContractTxResult(dbtx storage.DBTX, txHash crypto.Hash, ret []byte) error {
	if len(ret) == 0 {
		return nil
	}
	return dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txHash[:], PLANET_CONTRACT_RESULT, ret)
}

func (chain *Blockchain) LoadContractTxResult(dbtx storage.DBTX, txHash crypto.Hash) (ret []byte, err error) {
	if dbtx == nil {
		if dbtx, err = chain.store.BeginTX(false); err != nil {
			return
		}
		defer dbtx.Rollback()
	}
	return dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txHash[:], PLANET_CONTRACT_RESULT)
}

func (chain *Blockchain) storeContractOrigin(dbtx storage.DBTX, contract []byte, origin common.Address) error {
	if len(contract) != common.AddressLength {
		return fmt.Errorf("invalid contract %x", contract)
	}

	return dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, contract, PLANET_CONTRACT_ORIGIN, origin[:])
}

func (chain *Blockchain) loadContractOrigin(dbtx storage.DBTX, contract []byte) (origin common.Address, err error) {
	if len(contract) != common.AddressLength {
		err = fmt.Errorf("invalid contract %x", contract)
		return
	}

	var value []byte
	value, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, contract[:], PLANET_CONTRACT_ORIGIN)
	if err != nil {
		return
	}
	if len(value) != common.AddressLength {
		err = fmt.Errorf("invalid stored origin %x", value)
		return
	}

	copy(origin[:], value)
	return
}

func (chain *Blockchain) StoreContractAddress(dbtx storage.DBTX, txHash crypto.Hash, contractAddr []byte) bool {
	if len(contractAddr) != common.AddressLength {
		return false
	}

	err := dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, txHash[:], PLANET_CONTRACT_ADDR, contractAddr)
	if err != nil {
		return false
	}

	err = dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, contractAddr[:], PLANET_CONTRACT_TXID, txHash[:])
	if err != nil {
		return false
	}

	return true
}

func (chain *Blockchain) LoadContractAddressByTxid(dbtx storage.DBTX, txHash crypto.Hash) ([]byte, error) {
	var err error

	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			return nil, err
		}
		defer dbtx.Rollback()
	}

	address, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, txHash[:], PLANET_CONTRACT_ADDR)
	if err != nil {
		return nil, err
	}

	return address, nil
}

func (chain *Blockchain) LoadContractAddress(dbtx storage.DBTX, txHash crypto.Hash) (*address.Address, error) {

	data, err := chain.LoadContractAddressByTxid(dbtx, txHash)
	if err != nil {
		return nil, err
	}

	return address.MakeContractAddress(data, globals.GetNetwork())
}

func (chain *Blockchain) LoadContractTxidByAddress(dbtx storage.DBTX, contractAddr common.Address) (crypto.Hash, error) {
	var txHash crypto.Hash
	var err error

	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			return txHash, err
		}
		defer dbtx.Rollback()
	}

	hash, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, contractAddr[:], PLANET_CONTRACT_TXID)
	if err != nil {
		return txHash, err
	}

	copy(txHash[:], hash)
	return txHash, nil
}

func (chain *Blockchain) LoadStateRoot(dbtx storage.DBTX, blid crypto.Hash) (root crypto.Hash, err error) {
	var hash []byte

	if blid == ZERO_HASH {
		err = fmt.Errorf("invalid blid %s", blid)
		return
	}

	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			return
		}
		defer dbtx.Rollback()
	}

	hash, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, blid[:], PLANET_STATEROOT_BLOB)
	if err != nil {
		return
	}

	if bytes.Compare(hash, ZERO_HASH[:]) == 0 {
		err = fmt.Errorf("root of block %s is empty", blid)
		return
	}

	copy(root[:], hash)
	return
}

func (chain *Blockchain) StoreStateRootForBlock(dbtx storage.DBTX, blid crypto.Hash, root crypto.Hash) error {
	if blid == ZERO_HASH || root == ZERO_HASH {
		return fmt.Errorf("Invalid parameter")
	}

	err := dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, blid[:], PLANET_STATEROOT_BLOB, root[:])
	rlog.Debug("---StoreStateRoot-blid--", blid)
	if err != nil {
		return err
	}

	return nil
}

func (chain *Blockchain) RemoveStateRoot(dbtx storage.DBTX, blid crypto.Hash) error {
	return dbtx.Delete(BLOCKCHAIN_UNIVERSE, GALAXY_CONTRACT, blid[:], PLANET_STATEROOT_BLOB)
}

func (chain *Blockchain) GetStateDatabase() state.Database {
	//Fixme:
	db, err := rawdb.NewLevelDBDatabase(globals.GetDataDirectory()+"/state", 0, 0, "")
	_ = err

	return state.NewDatabase(db)
}

func (chain *Blockchain) GenesisStateRoot(dbtx storage.DBTX, blid crypto.Hash, height int64) (*state.StateDB, error) {
	statedb, _ := state.New(common.Hash{}, chain.GetStateDatabase(), nil)

	return statedb, nil
}

func (chain *Blockchain) NewStateDBByID(dbtx storage.DBTX, blid crypto.Hash) (*state.StateDB, error) {
	root, err := chain.LoadStateRoot(dbtx, blid)
	if err != nil {
		return nil, err
	}
	statedb, _ := state.New(common.Hash(root), chain.stateCache, nil)

	return statedb, nil
}

func (chain *Blockchain) NewStateDB(dbtx storage.DBTX, topoHeight int64) (*state.StateDB, error) {
	rlog.Debug("---NewStateDB---")
	rlog.Debug("topoheight:", topoHeight)

	prevBlid, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, topoHeight-1)
	if err != nil {
		return nil, err
	}

	prevHeight := chain.LoadHeightForBlId(dbtx, prevBlid)

	// if height of previous block reached hard fork, the root must not be empty
	if !globals.IsMainnet() && prevHeight >= config.TESTNET_CONTRACT_HEIGHT ||
		globals.IsMainnet() && prevHeight >= config.MAINNET_CONTRACT_HEIGHT {
		root, err := chain.LoadStateRoot(dbtx, prevBlid)
		if err != nil {
			return nil, err
		} else {
			rlog.Debugf("using prev root %x, topoheight %d", root, topoHeight-1)
			return state.New(common.Hash(root), chain.stateCache, nil)
		}
	} else {
		return state.New(common.Hash(ZERO_HASH), chain.stateCache, nil)
	}
}

func (chain *Blockchain) UpdateStateDB(dbtx storage.DBTX, statedb *state.StateDB, blid crypto.Hash) error {
	root := statedb.IntermediateRoot(true)

	rlog.Debug("---UpdateStateDB-Intermediate--")
	rlog.Debug("root:", root)
	err := chain.StoreStateRootForBlock(dbtx, blid, crypto.Hash(root))
	if err != nil {
		rlog.Error("---UpdateStateDB-StoreFailed--", err)
		return err
	}

	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true, nil)

	return nil
}

// get public and ephermal key to pay to address
// TODO we can also payto blobs which even hide address
// both have issues, this requires address to be public
// blobs require blobs as paramters
func (chain *Blockchain) getContractEphermalKey(txid crypto.Hash, index_within_tx uint64, pubkey crypto.Key) (tx_public_key, ehphermal_public_key crypto.Key) {

	var txSecretKey crypto.Key

	copy(txSecretKey[:], txid[:])
	crypto.ScReduce32(&txSecretKey)
	tx_public_key = *txSecretKey.PublicKey()

	publicViewkey := crypto.ContractSecretViewKey.PublicKey()

	derivation := crypto.KeyDerivation(publicViewkey, &txSecretKey) // keyderivation using wallet address view key

	// this becomes the key within Vout
	ehphermal_public_key = derivation.KeyDerivationToPublicKey(index_within_tx, pubkey)
	return
}

func (chain *Blockchain) GetBalanceOfContractAccount(account common.Address) (*big.Int, error) {
	dbtx, err := chain.store.BeginTX(false)
	if err != nil {
		return nil, err
	}

	defer dbtx.Rollback()

	topoHeight := chain.LoadTopoHeight(dbtx)

	statedb, err := chain.NewStateDB(dbtx, topoHeight)

	if err != nil {
		return nil, err
	}

	if statedb == nil {
		return nil, fmt.Errorf("new statedb failed")
	}

	return statedb.GetBalance(account), nil
}