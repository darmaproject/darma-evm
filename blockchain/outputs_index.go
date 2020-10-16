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

package blockchain

// NOTE: this is extremely critical code ( as a single error or typo here will lead to invalid transactions )
//
// thhis file implements code which controls output indexes
// rewrites them during chain reorganisation
import (
	"bytes"
	"fmt"
	"github.com/darmaproject/darmasuite/block"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/ringct"
	"github.com/darmaproject/darmasuite/storage"
	"github.com/darmaproject/darmasuite/transaction"
	"github.com/romana/rlog"
	"github.com/vmihailenco/msgpack"
	"sort"
)

type IndexData struct {
	InKey     ringct.CtKey
	ECDHTuple ringct.ECdhTuple // encrypted Amounts
	// Key crypto.Hash  // stealth address key
	// Commitment crypto.Hash // commitment public key
	Height       uint64 // height to which this belongs
	UnlockHeight uint64 // height at which it will unlock
}

type RewardInfo struct {
	Ids    []crypto.Hash
	Amount uint64
}

type StakeReward struct {
	Id     crypto.Hash
	Amount uint64
}

func (chain *Blockchain) writeMinerTx(reward uint64, dbtx storage.DBTX, bl *block.Block, index_start *int64, height int64, topoHeight int64) (result bool) {
	// ads miner tx separately as a special case
	var o globals.TXOutputData
	blockId := bl.GetHash()

	o.BLID = blockId // store block id
	o.TXID = bl.MinerTx.GetHash()

	o.Target = bl.MinerTx.Vout[0].Target
	o.InKey.Destination = bl.MinerTx.Vout[0].GetKey()

	// FIXME miner tx amount should be what we calculated
	o.InKey.Mask = ringct.ZeroCommitmentFromAmount(reward)
	o.Height = uint64(height)
	o.Unlock_Height = 0 // miner tx cannot be locked
	o.Index_within_tx = 0
	o.Index_Global = uint64(*index_start)
	o.Amount = reward
	o.SigType = 0
	o.Block_Time = bl.Timestamp
	o.TopoHeight = topoHeight
	o.TxType = globals.TX_TYPE_POW_PROFIT

	//ECDHTuple & sender pk is not available for miner tx
	if bl.MinerTx.ParseExtra() {
		// store public key if present
		if _, ok := bl.MinerTx.ExtraMap[transaction.TX_PUBLIC_KEY]; ok {
			o.Tx_Public_Key = bl.MinerTx.ExtraMap[transaction.TX_PUBLIC_KEY].(crypto.Key)
		}
	}
	//replace public key if sub publickey present
	if v, ok := bl.MinerTx.Vout[0].Target.(transaction.TxoutToSubAddress); ok {
		o.Tx_Public_Key = v.PubKey
	}

	serialized, err := msgpack.Marshal(&o)
	if err != nil {
		panic(err)
	}

	// store the index and relevant keys together in compact form
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
	*index_start++

	return true
}

func (chain *Blockchain) writeIssueTokenTx(dbtx storage.DBTX, bl *block.Block, index_start *int64, height int64, topoHeight int64, omniToken *transaction.OmniToken, tokenTx *transaction.Transaction, indexWithinTx uint64) (result bool) {
	blockId := bl.GetHash()

	// load all tx one by one
	tokenTxHash := tokenTx.GetHash()

	if !chain.IsTokenValid(dbtx, blockId, tokenTxHash) { // skip invalid TX
		rlog.Tracef(1, "bl %s tx %s ignored while building outputs index as per client protocol", blockId, tokenTxHash)
		return false
	}
	rlog.Tracef(1, "bl %s tx %s is being used while building outputs index as per client protocol", blockId, tokenTxHash)

	reward := omniToken.Amount / 100 * config.ISSUE_TOKEN_Alphabet[indexWithinTx]

	// ads miner tx separately as a special case
	var tokenOutputData globals.TokenOutputData

	tokenOutputData.BLID = blockId // store block id
	tokenOutputData.TXID = tokenTx.GetHash()

	tokenOutputData.Target = tokenTx.Vout[indexWithinTx].Target
	tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToKey).Key

	tokenOutputData.InKey.Mask = ringct.ZeroCommitmentFromAmount(reward)
	tokenOutputData.Height = uint64(height)
	tokenOutputData.Unlock_Height = 0 // miner tx cannot be locked
	tokenOutputData.Index_within_tx = indexWithinTx
	tokenOutputData.Index_Global = uint64(*index_start)
	tokenOutputData.Amount = reward
	tokenOutputData.SigType = 0
	tokenOutputData.Block_Time = bl.Timestamp
	tokenOutputData.TopoHeight = topoHeight
	tokenOutputData.TxType = globals.TX_TYPE_ISSUE_TOKEN
	tokenOutputData.TokenId = omniToken.Id.String()
	tokenOutputData.TokenSymbol = omniToken.Symbol

	//ECDHTuple & sender pk is not available for miner tx
	if tokenTx.ParseExtra() {
		// store public key if present
		if _, ok := tokenTx.ExtraMap[transaction.TX_PUBLIC_KEY]; ok {
			tokenOutputData.Tx_Public_Key = tokenTx.ExtraMap[transaction.TX_PUBLIC_KEY].(crypto.Key)
		}
	}

	serialized, err := msgpack.Marshal(&tokenOutputData)
	if err != nil {
		panic(err)
	}

	//TOKEN_OUTPUT_INDEX := []byte(tokenName)
	//
	//if bytes.Compare(GALAXY_OUTPUT_INDEX, TOKEN_OUTPUT_INDEX) == 0 {
	//  return false
	//}

	// store the index and relevant keys together in compact form
	//dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_TOKEN_OUTPUT_INDEX, itob(uint64(token_index_start)), serialized)
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)

	*index_start++

	return true
}

func (chain *Blockchain) writeTransferTokenTx(dbtx storage.DBTX, blockId crypto.Hash, bl *block.Block, token_index_start *int64, hard_fork_version_current int64, height int64, topoHeight int64, omniToken *transaction.OmniToken, tokenTx *transaction.Transaction, indexWithinTx uint64) (result bool) {
	// load all tx one by one
	tokenTxHash := tokenTx.GetHash()

	if !chain.IsTokenValid(dbtx, blockId, tokenTxHash) { // skip invalid TX
		rlog.Tracef(1, "bl %s tx %s ignored while building outputs index as per client protocol", blockId, tokenTxHash)
		return false
	}
	rlog.Tracef(1, "bl %s tx %s is being used while building outputs index as per client protocol", blockId, tokenTxHash)

	//tx, err := chain.LoadTxFromId(dbtx, tokenTxHash)
	//if err != nil {
	//  panic(fmt.Errorf("Cannot load  tx for %x err %s", tokenTxHash, err))
	//}
	//tx := tokenTx

	//indexWithinTx := uint64(0)
	var tokenOutputData globals.TokenOutputData

	tokenOutputData.BLID = blockId // store block id
	tokenOutputData.TXID = tokenTxHash
	tokenOutputData.Height = uint64(height)
	tokenOutputData.SigType = uint64(tokenTx.RctSignature.GetSigType())
	tokenOutputData.Block_Time = bl.Timestamp
	tokenOutputData.TopoHeight = topoHeight
	tokenOutputData.TxType = globals.TX_TYPE_TRANSFER_TOKEN
	//tokenOutputData.TokenName = omniToken.Name
	tokenOutputData.TokenId = omniToken.Id.String()
	tokenOutputData.TokenSymbol = omniToken.Symbol

	// TODO unlock specific outputs on specific height
	tokenOutputData.Unlock_Height = uint64(height) + config.NORMAL_TX_AMOUNT_UNLOCK

	// build the key image list and pack it
	for j := 0; j < len(tokenTx.Vin); j++ {
		kImages := crypto.Key(tokenTx.Vin[j].(transaction.TxinToKey).K_image)
		tokenOutputData.Key_Images = append(tokenOutputData.Key_Images, crypto.Key(kImages))
	}

	// zero out fields between tx
	tokenOutputData.Tx_Public_Key = crypto.Key(ZERO_HASH)
	tokenOutputData.PaymentID = tokenOutputData.PaymentID[:0]

	extraParsed := tokenTx.ParseExtra()

	locked := tokenTx.IsLocked()

	j := indexWithinTx

	// tx has been loaded, now lets get the vout
	//for j := uint64(0); j < uint64(len(tx.Vout)); j++ {

	//tokenOutputData.Target = tokenTx.Vout[j].Target

	//tokenOutputData.TxType = globals.TX_TYPE_NORMAL

	tokenOutputData.Target = tokenTx.Vout[j].Target

	//var amount uint64
	//switch tokenOutputData.Target.(type) {
	//case transaction.TxoutToBuyShare:
	//tokenOutputData.TxType = globals.TX_TYPE_BUY_SHARE
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToBuyShare).Key
	//case transaction.TxoutToRepoShare:
	//tokenOutputData.TxType = globals.TX_TYPE_REPO_SHARE
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToRepoShare).Key
	//case transaction.TxoutToRegisterPool:
	//tokenOutputData.TxType = globals.TX_TYPE_REGISTER_POOL
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToRegisterPool).Key
	//case transaction.TxoutToClosePool:
	//tokenOutputData.TxType = globals.TX_TYPE_CLOSE_POOL
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToClosePool).Key
	//case transaction.TxoutToKey:
	//	tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToKey).Key
	//	tokenOutputData.TxType = globals.TX_TYPE_NORMAL
	//case transaction.TxoutToIssueToken:
	//amount = tokenOutputData.Target.(transaction.TxoutToIssueToken).Value
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToIssueToken).Key
	//tokenOutputData.TxType = globals.TX_TYPE_ISSUE_TOKEN
	//case transaction.TxoutToTransferToken:
	//tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToTransferToken).Key
	//tokenOutputData.TxType = globals.TX_TYPE_TRANSFER_TOKEN
	//default:
	//	panic(fmt.Errorf("invalid vout"))
	//}

	tokenOutputData.InKey.Mask = crypto.Key(tokenTx.RctSignature.OutPk[j].Mask)

	tokenOutputData.ECDHTuple = tokenTx.RctSignature.ECdhInfo[j]

	tokenOutputData.Index_within_tx = indexWithinTx
	tokenOutputData.Index_Global = uint64(*token_index_start)
	tokenOutputData.Amount = tokenTx.Vout[j].Amount
	tokenOutputData.Unlock_Height = 0
	//tokenOutputData.TxFee = tx.RctSignature.GetTXFee()

	if j == 0 && tokenTx.UnlockTime != 0 { // only first output of a TX can be locked
		tokenOutputData.Unlock_Height = tokenTx.UnlockTime
	}

	//if hard_fork_version_current >= 3 && tokenOutputData.Unlock_Height != 0 {
	if hard_fork_version_current >= 3 && tokenOutputData.Unlock_Height != 0 {
		if tokenOutputData.Unlock_Height < config.CRYPTONOTE_MAX_BLOCK_NUMBER {
			if tokenOutputData.Unlock_Height < (tokenOutputData.Height + 1000) {
				tokenOutputData.Unlock_Height = tokenOutputData.Height + 1000
			}
		} else {
			if tokenOutputData.Unlock_Height < (tokenOutputData.Block_Time + 12000) {
				tokenOutputData.Unlock_Height = tokenOutputData.Block_Time + 12000
			}
		}
	}

	if locked == true && j == 0 {
		rlog.Infof("-------->Set Unlock IndexGlobal %d txid %s", config.MAX_TX_AMOUNT_UNLOCK, tokenTx.GetHash().String())
		tokenOutputData.Unlock_Height = config.MAX_TX_AMOUNT_UNLOCK
	}

	// include the key image list in the first output itself
	// rest all the outputs donot contain the keyimage
	if j != 0 && len(tokenOutputData.Key_Images) > 0 {
		tokenOutputData.Key_Images = tokenOutputData.Key_Images[:0]
	}

	//var tokenName string
	//var tokenTx transaction.Transaction

	if extraParsed {
		// store public key if present
		if _, ok := tokenTx.ExtraMap[transaction.TX_PUBLIC_KEY]; ok {
			tokenOutputData.Tx_Public_Key = tokenTx.ExtraMap[transaction.TX_PUBLIC_KEY].(crypto.Key)
		}

		// store private key if present
		//if _, ok := tx.ExtraMap[transaction.TX_PRIVATE_KEY]; ok {
		//  tokenOutputData.Tx_Private_Key = tx.ExtraMap[transaction.TX_PRIVATE_KEY].(crypto.Key)
		//}

		//if _, ok := tx.ExtraMap[transaction.TOKEN_ID]; ok {
		//  tokenName = tx.ExtraMap[transaction.TOKEN_ID].(string)
		//}

		//if _, ok := tx.ExtraMap[transaction.TOKEN_TX]; ok {
		//tokenTx = tx.ExtraMap[transaction.TOKEN_TX].(transaction.Transaction)
		//}

		// store payment IDs if present
		if _, ok := tokenTx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID]; ok {
			tokenOutputData.PaymentID = tokenTx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID].([]byte)
		} else if _, ok := tokenTx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID]; ok {
			tokenOutputData.PaymentID = tokenTx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID].([]byte)
		}
	}

	if voutsub, ok := tokenOutputData.Target.(transaction.TxoutToSubAddress); ok {
		tokenOutputData.Tx_Public_Key = voutsub.PubKey
		tokenOutputData.InKey.Destination = voutsub.Key
	} else {
		//tokenOutputData.Tx_Public_Key = txPublicKey
		tokenOutputData.InKey.Destination = tokenOutputData.Target.(transaction.TxoutToKey).Key
	}

	serialized, err := msgpack.Marshal(&tokenOutputData)
	if err != nil {
		panic(err)
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*token_index_start)), serialized)

	//switch tokenOutputData.Target.(type) {
	//case transaction.TxoutToIssueToken:
	//  chain.writeIssueTokenTx(amount, dbtx, bl, 0, height, topoHeight, tokenName, tokenTx)
	//case transaction.TxoutToTransferToken:
	//  chain.writeTransferTokenTx(dbtx, blockId, bl, token_index_start, height, topoHeight, tokenName, tokenTx)
	//default:
	//  ...
	//}

	*token_index_start++
	//indexWithinTx++
	//}

	return true
}

func (chain *Blockchain) writeTokenTx(
	dbtx storage.DBTX,
	txid crypto.Hash,
	blid crypto.Hash,
	indexGlobal *int64,
	timestamp uint64,
	height, topoHeight int64) {
	transfers, err := chain.loadErc20Transfers(dbtx, txid)
	if err != nil {
		rlog.Warnf("failed loading token transfers, tx %s, err %s", txid, err)
		return
	}

	for k, v := range transfers {
		var o globals.TXOutputData
		o.BLID = blid
		o.TXID = txid
		o.Height = uint64(height)
		o.TopoHeight = topoHeight
		o.Block_Time = timestamp

		// fixme: tx.RctSignature.GetTXFee()
		o.TxFee = 0
		o.Index_within_tx = uint64(k)
		o.Index_Global = uint64(*indexGlobal)
		o.Erc20Transfer = &v

		serialized, err := msgpack.Marshal(&o)
		if err != nil {
			panic(err)
		}

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*indexGlobal)), serialized)
		*indexGlobal++

		rlog.Debugf("topoheight %d, tx %s, token amount %s", topoHeight, o.TXID, v.Amount)
	}
}

func (chain *Blockchain) writeStakeTx2(splitReward uint64, dbtx storage.DBTX, bl *block.Block, index_start *int64, height int64, topoHeight int64) (result bool) {
	blid := bl.GetHash()

	// make a tx key
	var txSecretKey, txPublicKey crypto.Key
	hash := crypto.Keccak256(blid[:], []byte("staketx"))
	copy(txSecretKey[:], hash[:])
	crypto.ScReduce32(&txSecretKey)
	txPublicKey = *txSecretKey.PublicKey()

	// make a tx
	var tx transaction.Transaction
	tx.Version = 2
	tx.UnlockTime = uint64(height) + config.STAKE_TX_AMOUNT_UNLOCK
	tx.Vin = append(tx.Vin, transaction.Txin_gen{Height: uint64(height)})
	tx.ExtraMap = make(map[transaction.EXTRA_TAG]interface{})
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = txPublicKey
	tx.Extra = tx.SerializeExtra()
	tx.RctSignature = &ringct.RctSig{}
	i := 0

	if len(bl.Vote) == 0 {
		return true
	}

	for _, v := range bl.Vote {
		share, _ := chain.LoadStakeShare(dbtx, v.ShareId)
		pool, _ := chain.LoadStakePool(dbtx, share.PoolId)
		var derivation crypto.Key

		// for pool
		derivation = crypto.KeyDerivation(&pool.Reward.ViewKey, &txSecretKey)
		ephemeralPublicKey := derivation.KeyDerivationToPublicKey(uint64(i), pool.Reward.SpendKey)
		if pool.Reward.IsSubAddress() {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(&txSecretKey, &pool.Reward.SpendKey)}
			target.Key = ephemeralPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ephemeralPublicKey}})
		}
		i++

		// for share
		derivation = crypto.KeyDerivation(&share.Reward.ViewKey, &txSecretKey)
		ephemeralPublicKey = derivation.KeyDerivationToPublicKey(uint64(i), share.Reward.SpendKey)
		if share.Reward.IsSubAddress() {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(&txSecretKey, &share.Reward.SpendKey)}
			target.Key = ephemeralPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ephemeralPublicKey}})
		}
		i++
	}
	txid := tx.GetHash()

	// client protocol
	rlog.Debugf("running client protocol for %s staketx %s topo %d", blid, txid, topoHeight)
	chain.StoreTX(dbtx, &tx)
	chain.StoreTxHeight(dbtx, txid, topoHeight)
	chain.store_TX_in_Block(dbtx, blid, txid)
	chain.storeStakeTxInBlock(dbtx, blid, txid)
	chain.markTX(dbtx, blid, txid, true) // NOTE: for mainnet before 160000, tx was not revoked in clientProtocolReverse

	poolTotalReward := splitReward * config.RATE_POS_POOL
	shareTotalReward := splitReward * config.RATE_POS_SHARE

	poolAvg := poolTotalReward / uint64(len(bl.Vote))
	shareAvg := shareTotalReward / uint64(len(bl.Vote))

	tmpPoolReward := uint64(0)
	tmpShareReward := uint64(0)

	indexWithinTx := uint64(0)
	for k, v := range bl.Vote {
		share, _ := chain.LoadStakeShare(dbtx, v.ShareId)
		pool, _ := chain.LoadStakePool(dbtx, share.PoolId)

		if poolAvg > 0 {
			poolReward := uint64(0)

			if k+1 == len(bl.Vote) {
				poolReward = poolTotalReward - tmpPoolReward
			} else {
				poolReward = poolAvg
				tmpPoolReward += poolReward
			}

			var o globals.TXOutputData
			o.BLID = blid // store block id
			o.TXID = txid
			o.Height = uint64(height)
			o.SigType = 0
			o.Block_Time = bl.Timestamp
			o.TopoHeight = topoHeight

			o.Unlock_Height = 0 // sigtype 0 implies unlock height 0, see input maturity

			o.Target = tx.Vout[2*k].Target
			if voutsub, ok := o.Target.(transaction.TxoutToSubAddress); ok {
				o.Tx_Public_Key = voutsub.PubKey
				o.InKey.Destination = voutsub.Key
			} else {
				o.Tx_Public_Key = txPublicKey
				o.InKey.Destination = o.Target.(transaction.TxoutToKey).Key
			}

			o.InKey.Mask = ringct.ZeroCommitmentFromAmount(poolReward)

			o.Index_within_tx = indexWithinTx
			o.Index_Global = uint64(*index_start)
			o.Amount = poolReward
			o.TxType = globals.TX_TYPE_POOL_PROFIT
			o.RewardPoolId = share.PoolId // pool to which this reward would go

			serialized, err := msgpack.Marshal(&o)
			if err != nil {
				panic(err)
			}
			// store the index and relevant keys together in compact form
			dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
			*index_start++
			indexWithinTx++

			pool.LastPayTime = int64(bl.Timestamp)
			pool.Profit += poolReward
			chain.storeStakePool(dbtx, pool)
		}

		if shareAvg > 0 {
			shareReward := uint64(0)

			if k+1 == len(bl.Vote) {
				shareReward = shareTotalReward - tmpShareReward
			} else {
				shareReward = shareAvg
				tmpShareReward += shareReward
			}

			var o globals.TXOutputData
			o.BLID = blid // store block id
			o.TXID = txid
			o.Height = uint64(height)
			o.SigType = 0
			o.Block_Time = bl.Timestamp
			o.TopoHeight = topoHeight

			o.Unlock_Height = 0 // sigtype 0 implies unlock height 0, see input maturity

			o.Target = tx.Vout[2*k+1].Target
			if voutsub, ok := o.Target.(transaction.TxoutToSubAddress); ok {
				o.Tx_Public_Key = voutsub.PubKey
				o.InKey.Destination = voutsub.Key
			} else {
				o.Tx_Public_Key = txPublicKey
				o.InKey.Destination = o.Target.(transaction.TxoutToKey).Key
			}

			o.InKey.Mask = ringct.ZeroCommitmentFromAmount(shareReward)

			o.Index_within_tx = indexWithinTx
			o.Index_Global = uint64(*index_start)
			o.Amount = shareReward
			o.TxType = globals.TX_TYPE_SHARE_PROFIT
			o.RewardPoolId = share.PoolId // pool to which this share belongs
			o.RewardShareId = v.ShareId   // share to which this reward would go

			serialized, err := msgpack.Marshal(&o)
			if err != nil {
				panic(err)
			}
			// store the index and relevant keys together in compact form
			dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
			*index_start++
			indexWithinTx++

			share.LastPayTime = int64(bl.Timestamp)
			share.Profit += shareReward
			chain.storeStakeShare(dbtx, share)
		}
	}

	return true
}

func (chain *Blockchain) writePoolStakeTx(dbtx storage.DBTX, bl *block.Block, index_start *int64, height int64, topoHeight int64, stakeRewards []StakeReward) (total uint64) {
	if len(stakeRewards) == 0 {
		logger.Warnf("no pool rewards, height %d/%d", height, topoHeight)
		return
	}

	blid := bl.GetHash()

	// make a tx key
	var txSecretKey, txPublicKey crypto.Key
	hash := crypto.Keccak256(blid[:], []byte("poolstaketx"))
	copy(txSecretKey[:], hash[:])
	crypto.ScReduce32(&txSecretKey)
	txPublicKey = *txSecretKey.PublicKey()

	// make a tx
	var tx transaction.Transaction
	tx.Version = 2
	tx.UnlockTime = uint64(height) + config.STAKE_TX_AMOUNT_UNLOCK
	tx.Vin = append(tx.Vin, transaction.Txin_gen{Height: uint64(height)})
	tx.ExtraMap = make(map[transaction.EXTRA_TAG]interface{})
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = txPublicKey
	tx.Extra = tx.SerializeExtra()
	tx.RctSignature = &ringct.RctSig{}

	for i, v := range stakeRewards {
		total += v.Amount
		pool, _ := chain.LoadStakePool(dbtx, v.Id)
		var derivation crypto.Key

		// for pool
		derivation = crypto.KeyDerivation(&pool.Reward.ViewKey, &txSecretKey)
		ephemeralPublicKey := derivation.KeyDerivationToPublicKey(uint64(i), pool.Reward.SpendKey)
		if pool.Reward.IsSubAddress() {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(&txSecretKey, &pool.Reward.SpendKey)}
			target.Key = ephemeralPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ephemeralPublicKey}})
		}
	}
	txid := tx.GetHash()

	// client protocol
	rlog.Debugf("running client protocol for %s pool staketx %s topo %d", blid, txid, topoHeight)
	chain.StoreTX(dbtx, &tx)
	chain.StoreTxHeight(dbtx, txid, topoHeight)
	chain.store_TX_in_Block(dbtx, blid, txid)
	chain.storePoolStakeTxInBlock(dbtx, blid, txid)
	chain.markTX(dbtx, blid, txid, true) // NOTE: for mainnet before 160000, tx was not revoked in clientProtocolReverse

	for k, v := range stakeRewards {
		pool, _ := chain.LoadStakePool(dbtx, v.Id)

		poolReward := v.Amount

		var o globals.TXOutputData
		o.BLID = blid // store block id
		o.TXID = txid
		o.Height = uint64(height)
		o.SigType = 0
		o.Block_Time = bl.Timestamp
		o.TopoHeight = topoHeight

		o.Unlock_Height = 0 // sigtype 0 implies unlock height 0, see input maturity

		o.Target = tx.Vout[k].Target
		if voutsub, ok := o.Target.(transaction.TxoutToSubAddress); ok {
			o.Tx_Public_Key = voutsub.PubKey
			o.InKey.Destination = voutsub.Key
		} else {
			o.Tx_Public_Key = txPublicKey
			o.InKey.Destination = o.Target.(transaction.TxoutToKey).Key
		}

		o.InKey.Mask = ringct.ZeroCommitmentFromAmount(poolReward)

		o.Index_within_tx = uint64(k)
		o.Index_Global = uint64(*index_start)
		o.Amount = poolReward
		o.TxType = globals.TX_TYPE_POOL_PROFIT
		o.RewardPoolId = v.Id // pool to which this reward would go

		serialized, err := msgpack.Marshal(&o)
		if err != nil {
			panic(err)
		}
		// store the index and relevant keys together in compact form
		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
		*index_start++

		pool.LastPayTime = int64(bl.Timestamp)
		pool.Profit += poolReward
		chain.storeStakePool(dbtx, pool)
	}

	return
}

func (chain *Blockchain) writeShareStakeTx(dbtx storage.DBTX, bl *block.Block, index_start *int64, height int64, topoHeight int64, stakeRewards []StakeReward) (total uint64) {
	if len(stakeRewards) == 0 {
		logger.Warnf("no share rewards, height %d/%d", height, topoHeight)
		return
	}

	blid := bl.GetHash()

	// make a tx key
	var txSecretKey, txPublicKey crypto.Key
	hash := crypto.Keccak256(blid[:], []byte("sharestaketx"))
	copy(txSecretKey[:], hash[:])
	crypto.ScReduce32(&txSecretKey)
	txPublicKey = *txSecretKey.PublicKey()

	// make a tx
	var tx transaction.Transaction
	tx.Version = 2
	tx.UnlockTime = uint64(height) + config.STAKE_TX_AMOUNT_UNLOCK
	tx.Vin = append(tx.Vin, transaction.Txin_gen{Height: uint64(height)})
	tx.ExtraMap = make(map[transaction.EXTRA_TAG]interface{})
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = txPublicKey
	tx.Extra = tx.SerializeExtra()
	tx.RctSignature = &ringct.RctSig{}

	for i, v := range stakeRewards {
		total += v.Amount
		share, _ := chain.LoadStakeShare(dbtx, v.Id)
		var derivation crypto.Key

		// for share
		derivation = crypto.KeyDerivation(&share.Reward.ViewKey, &txSecretKey)
		ephemeralPublicKey := derivation.KeyDerivationToPublicKey(uint64(i), share.Reward.SpendKey)
		if share.Reward.IsSubAddress() {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(&txSecretKey, &share.Reward.SpendKey)}
			target.Key = ephemeralPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ephemeralPublicKey}})
		}
	}
	txid := tx.GetHash()

	// client protocol
	rlog.Debugf("running client protocol for %s share staketx %s topo %d", blid, txid, topoHeight)
	chain.StoreTX(dbtx, &tx)
	chain.StoreTxHeight(dbtx, txid, topoHeight)
	chain.store_TX_in_Block(dbtx, blid, txid)
	chain.storeShareStakeTxInBlock(dbtx, blid, txid)
	chain.markTX(dbtx, blid, txid, true) // NOTE: for mainnet before 160000, tx was not revoked in clientProtocolReverse

	for k, v := range stakeRewards {
		share, _ := chain.LoadStakeShare(dbtx, v.Id)

		shareReward := v.Amount

		var o globals.TXOutputData
		o.BLID = blid // store block id
		o.TXID = txid
		o.Height = uint64(height)
		o.SigType = 0
		o.Block_Time = bl.Timestamp
		o.TopoHeight = topoHeight

		o.Unlock_Height = 0 // sigtype 0 implies unlock height 0, see input maturity

		o.Target = tx.Vout[k].Target
		if voutsub, ok := o.Target.(transaction.TxoutToSubAddress); ok {
			o.Tx_Public_Key = voutsub.PubKey
			o.InKey.Destination = voutsub.Key
		} else {
			o.Tx_Public_Key = txPublicKey
			o.InKey.Destination = o.Target.(transaction.TxoutToKey).Key
		}

		o.InKey.Mask = ringct.ZeroCommitmentFromAmount(shareReward)

		o.Index_within_tx = uint64(k)
		o.Index_Global = uint64(*index_start)
		o.Amount = shareReward
		o.TxType = globals.TX_TYPE_SHARE_PROFIT
		o.RewardPoolId = share.PoolId // pool to which this share belongs
		o.RewardShareId = v.Id        // share to which this reward would go

		serialized, err := msgpack.Marshal(&o)
		if err != nil {
			panic(err)
		}
		// store the index and relevant keys together in compact form
		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
		*index_start++
		//indexWithinTx++

		share.LastPayTime = int64(bl.Timestamp)
		share.Profit += shareReward
		chain.storeStakeShare(dbtx, share)
	}

	return
}

func (chain *Blockchain) writeContractTx(
	dbtx storage.DBTX,
	txid crypto.Hash,
	blid crypto.Hash,
	indexWithinTx *uint64,
	indexGlobal *int64,
	timestamp uint64,
	height int64,
	topoHeight int64) (result bool) {

	sctxData, err := chain.loadContractTransfer(dbtx, txid)
	if err != nil {
		logger.Debugf("No contract tx in tx %s", txid)
		return
	}

	for _, v := range sctxData.TransferE {
		var o globals.TXOutputData
		addr := common.HexToAddress(v.Address)
		var pubkey crypto.Key
		copy(pubkey[:], addr[:])
		if err != nil || !pubkey.Public_Key_Valid() {
			logger.Warnf("invalid output address %s, tx %s, err %s", v.Address, txid, err)
			continue
		}

		o.BLID = blid // store block id
		o.TXID = txid
		o.Height = uint64(height)
		o.TopoHeight = topoHeight
		o.SigType = 0
		o.Unlock_Height = 0
		o.Block_Time = timestamp
		o.Index_within_tx = *indexWithinTx
		o.Key_Images = o.Key_Images[:0]

		// generate one time keys
		o.Tx_Public_Key, o.InKey.Destination = chain.getContractEphermalKey(txid, *indexWithinTx, pubkey)
		o.Target = transaction.TxoutToKey{Key: o.InKey.Destination}
		o.Amount = v.Amount
		o.InKey.Mask = ringct.ZeroCommitmentFromAmount(v.Amount)
		o.Index_Global = uint64(*indexGlobal)
		o.TxType = globals.TX_TYPE_CONTRACT_TX

		rlog.Debugf("writing SC output height %d, address %s, amount %s\n", height, v.Address, globals.FormatMoney(v.Amount))

		serialized, err := msgpack.Marshal(&o)
		if err != nil {
			panic(err)
		}

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*indexGlobal)), serialized)
		*indexGlobal++
		*indexWithinTx++
	}
	return true
}

func (chain *Blockchain) writePoolBonusTx(dbtx storage.DBTX,
	blid crypto.Hash,
	timestamp uint64,
	index_start *int64,
	height int64,
	topoHeight int64) (result bool, destroyed uint64) {
	currentBonusHeight := height - globals.GetBonusDelayHeight()

	coinsTotal := chain.GetAlreadyGeneratedCoinsBetween(dbtx, currentBonusHeight-globals.GetBonusLifetimeHeight(), currentBonusHeight)
	bonusTotal := coinsTotal * config.RATE_BONUS / 100

	bonusPoolShareInfoRanks, err := chain.GetPoolBonusStats(dbtx, height)
	if err != nil {
		panic(fmt.Errorf("invalid pool bonus stats of height %d, err %s", height, err))
		return
	}

	checkTotal := uint64(0)
	bonusRewards := chain.CalcBonusReward(height, bonusTotal, bonusPoolShareInfoRanks)
	if len(bonusRewards) == 0 {
		logger.Warnf("height %d block %s has no bonus reward", height, blid)
		result = true
		return
	}

	// create tx keys
	var txSecretKey, txPublicKey crypto.Key
	hash := crypto.Keccak256(blid[:], []byte("bonustx"))
	copy(txSecretKey[:], hash[:])
	crypto.ScReduce32(&txSecretKey)
	txPublicKey = *txSecretKey.PublicKey()

	// create tx
	var tx transaction.Transaction
	tx.Version = 2
	tx.UnlockTime = uint64(height) + config.STAKE_TX_AMOUNT_UNLOCK
	tx.Vin = append(tx.Vin, transaction.Txin_gen{Height: uint64(height)})
	tx.ExtraMap = make(map[transaction.EXTRA_TAG]interface{})
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = txPublicKey
	tx.Extra = tx.SerializeExtra()
	tx.RctSignature = &ringct.RctSig{}
	for k, v := range bonusRewards {
		pool, _ := chain.LoadStakePool(dbtx, v.PoolId)
		//TODO: continue, if pool closed when bonusHeight.
		derivation := crypto.KeyDerivation(&pool.Reward.ViewKey, &txSecretKey)
		ephemeralKey := derivation.KeyDerivationToPublicKey(uint64(k), pool.Reward.SpendKey)
		if pool.Reward.IsSubAddress() {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(&txSecretKey, &pool.Reward.SpendKey)}
			target.Key = ephemeralKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ephemeralKey}})
		}
	}
	txid := tx.GetHash()

	rlog.Debugf("running client protocol for %s bonustx %s topo %d", blid, txid, topoHeight)
	chain.StoreTX(dbtx, &tx)
	chain.StoreTxHeight(dbtx, txid, topoHeight)
	chain.store_TX_in_Block(dbtx, blid, txid)
	chain.storeBonusTxInBlock(dbtx, blid, txid)
	chain.markTX(dbtx, blid, txid, true)

	for k, v := range bonusRewards {
		pool, _ := chain.LoadStakePool(dbtx, v.PoolId)

		//TODO: continue, if pool closed when bonusHeight.
		var o globals.TXOutputData
		o.BLID = blid // store block id
		o.TXID = txid
		o.Height = uint64(height)
		o.SigType = 0
		o.Block_Time = timestamp
		o.TopoHeight = topoHeight

		o.Unlock_Height = 0 // sigtype 0 implies unlock height 0, see input maturity

		o.Target = tx.Vout[k].Target
		if voutsub, ok := o.Target.(transaction.TxoutToSubAddress); ok {
			o.Tx_Public_Key = voutsub.PubKey
			o.InKey.Destination = voutsub.Key
		} else {
			o.Tx_Public_Key = txPublicKey
			o.InKey.Destination = o.Target.(transaction.TxoutToKey).Key
		}

		o.InKey.Mask = ringct.ZeroCommitmentFromAmount(uint64(v.Amount))

		o.Index_within_tx = uint64(k)
		o.Index_Global = uint64(*index_start)
		o.Amount = uint64(v.Amount)
		o.TxType = globals.TX_TYPE_BONUS_PROFIT
		o.RewardPoolId = v.PoolId // pool to which this reward would go

		serialized, err := msgpack.Marshal(&o)
		if err != nil {
			panic(err)
		}

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(*index_start)), serialized)
		*index_start++

		pool.LastPayTime = int64(timestamp)
		pool.Profit += uint64(v.Amount)
		chain.storeStakePool(dbtx, pool)
		checkTotal += uint64(v.Amount)
	}

	logger.Debugf("height %d, bonusTotal %d and checkTotal %d", height, bonusTotal, checkTotal)

	if bonusTotal < checkTotal {
		panic(fmt.Errorf("bonus error bonusTotal %d Less than checkTotal %d", bonusTotal, checkTotal))
	}

	return true, bonusTotal - checkTotal
}

func (chain *Blockchain) CalcBonusReward(height int64, bonusTotal uint64, infoRanks []*PoolShareInfoRank) (rewards []*BonusReward) {
	stakePoolRankStats := GetStakePoolRankStats(infoRanks)

	var goldBonusCount int
	var silverBonusCount int
	var bronzeBonusCount int
	var silverBonusAmount uint32
	var bronzeBonusAmount uint32

	if stakePoolRankStats[RankTypeGold] != nil {
		goldBonusCount = stakePoolRankStats[RankTypeGold].TotalCount
	}
	if stakePoolRankStats[RankTypeSilver] != nil {
		silverBonusCount = stakePoolRankStats[RankTypeSilver].TotalCount
	}
	if stakePoolRankStats[RankTypeBronze] != nil {
		bronzeBonusCount = stakePoolRankStats[RankTypeBronze].TotalCount
	}

	if stakePoolRankStats[RankTypeSilver] != nil {
		silverBonusAmount = stakePoolRankStats[RankTypeSilver].TotalVoteNum
	}
	if stakePoolRankStats[RankTypeBronze] != nil {
		bronzeBonusAmount = stakePoolRankStats[RankTypeBronze].TotalVoteNum
	}

	var goldBonusTotal int64
	if silverBonusCount == 0 && bronzeBonusCount == 0 {
		goldBonusTotal = int64(bonusTotal)
	} else {
		goldBonusTotal = int64(bonusTotal * 5 / 10)
	}

	silverBonusTotal := int64(bonusTotal) * 5 / 10

	goldBonusAvg := int64(0)
	silverBonusAvg := int64(0)
	bronzeBonusTotal := int64(0)
	const rateMin = -2500 // -25%
	const rateMax = -0500 // -5%

	for k, v := range infoRanks {
		if 0 <= k && k < GoldPoolLimit {
			if goldBonusCount <= 0 {
				panic(fmt.Errorf("gold pool count is 0"))
			}

			goldBonusAvg = goldBonusTotal / int64(goldBonusCount) // total = 30%; half of total, avg
			if goldBonusAvg > 0 {
				expectBonusReward := goldBonusAvg
				actualBonusReward := int64(0)

				rate := int64(10000)
				if v.PreVoteNum > 0 {
					rate = 10000 * (int64(v.VoteNum) - int64(v.PreVoteNum)) / int64(v.PreVoteNum)
				}

				if rate < rateMin {
					actualBonusReward = expectBonusReward / 2
					bronzeBonusTotal += expectBonusReward - actualBonusReward
				} else if rateMin <= rate && rate <= rateMax {
					actualBonusReward = (expectBonusReward*10000 + expectBonusReward*rate*2) / 10000
					bronzeBonusTotal += expectBonusReward - actualBonusReward
				} else {
					actualBonusReward = expectBonusReward
				}

				if actualBonusReward <= 0 {
					logger.Errorf("no gold bonus for %s", v.PoolId)
					return
				}

				rewards = append(rewards, &BonusReward{
					PoolId: v.PoolId,
					Amount: uint64(actualBonusReward),
				})
			}
		}

		if GoldPoolLimit <= k && k < SilverPoolLimit {
			if silverBonusAmount <= 0 {
				panic(fmt.Errorf("total silver vote num is 0"))
			}

			silverBonusAvg = silverBonusTotal / int64(silverBonusAmount) // total = 30%; half of total, avg
			if silverBonusAvg > 0 {
				expectBonusReward := silverBonusAvg * int64(v.VoteNum)
				actualBonusReward := int64(0)

				rate := int64(10000)
				if v.PreVoteNum > 0 {
					rate = 10000 * (int64(v.VoteNum) - int64(v.PreVoteNum)) / int64(v.PreVoteNum)
				}

				if rate < rateMin {
					actualBonusReward = expectBonusReward / 2
					bronzeBonusTotal += expectBonusReward - actualBonusReward
				} else if rateMin <= rate && rate <= rateMax {
					actualBonusReward = (expectBonusReward*10000 + expectBonusReward*rate*2) / 10000
					bronzeBonusTotal += expectBonusReward - actualBonusReward
				} else {
					actualBonusReward = expectBonusReward
				}

				if actualBonusReward <= 0 {
					logger.Errorf("no silver bonus for %s", v.PoolId)
					return
				}

				rewards = append(rewards, &BonusReward{
					PoolId: v.PoolId,
					Amount: uint64(actualBonusReward),
				})
			}
		}

		if SilverPoolLimit <= k && k < BronzePoolLimit {
			if bronzeBonusAmount <= 0 {
				panic(fmt.Errorf("total bronze vote num is 0"))
			}

			bronzeBonusAvg := bronzeBonusTotal / int64(bronzeBonusAmount)

			if bronzeBonusAvg > 0 {
				expectBonusReward := bronzeBonusAvg * int64(v.VoteNum)
				actualBonusReward := expectBonusReward

				rewards = append(rewards, &BonusReward{
					PoolId: v.PoolId,
					Amount: uint64(actualBonusReward),
				})
			}
		}
	}

	return
}

func (chain *Blockchain) calcReward(dbtx storage.DBTX,
	totalReward uint64,
	baseReward uint64,
	height int64,
	bl *block.Block,
	sideblock bool) (powReward, posReward, destroyed uint64) {
	if height < globals.GetVotingStartHeight() {
		powReward = totalReward
		posReward = 0
		return
	}

	// reserved for PoS season bonus
	reserved := baseReward / 100 * config.RATE_BONUS

	if !globals.IsMainnet() || height >= config.FIX_SIDEBLOCK_REWARD { // version 4: reward fees to side blocks with votes
		if !sideblock {
			if len(bl.Vote) != 0 { // 5% + 65%
				powReward = totalReward / 100 * config.RATE_POW
				posReward = totalReward / 100 * (config.RATE_POS_POOL + config.RATE_POS_SHARE)
			} else {
				powReward = 0
				posReward = 0
			}
		} else {
			if len(bl.Vote) != 0 {
				powReward = totalReward - baseReward // reward just fees
			} else {
				powReward = 0
			}
			posReward = 0
		}
	} else if height >= config.FIX_ZERO_REWARD { // version 3: NO reward for side blocks or block without votes
		if !sideblock {
			if len(bl.Vote) != 0 { // 5% + 65%
				powReward = totalReward / 100 * config.RATE_POW
				posReward = totalReward / 100 * (config.RATE_POS_POOL + config.RATE_POS_SHARE)
			} else {
				powReward = 0
				posReward = 0
			}
		} else {
			powReward = 0
			posReward = 0
		}
	} else if height >= config.FIXED_REWARD_HEIGHT { // version 2: fix too little reward for side block
		if !sideblock {
			if len(bl.Vote) != 0 { // 5% + 65%
				powReward = totalReward / 100 * config.RATE_POW
				posReward = totalReward / 100 * (config.RATE_POS_POOL + config.RATE_POS_SHARE)
			} else {
				powReward = totalReward / 100 * config.RATE_POW / 10
				posReward = 0
			}
		} else {
			powReward = totalReward * 2 / 10
			posReward = 0
		}
	} else { // version 1
		if !sideblock && len(bl.Vote) != 0 { // 5% + 65%
			powReward = totalReward / 100 * config.RATE_POW
			posReward = totalReward / 100 * (config.RATE_POS_POOL + config.RATE_POS_SHARE)
		} else {
			powReward = totalReward / 100 / 10
			posReward = 0
		}
	}

	if totalReward < powReward+posReward+reserved {
		panic(fmt.Errorf("height %d(sideblock %t), total %d, pow %d, pos %d, reserved %d", height, sideblock, totalReward, powReward, posReward, reserved))
	}

	destroyed = totalReward - powReward - posReward - reserved

	return
}

// this function writes or overwrites the data related to outputs
// the following data is collected from each output
// the secret key,
// the commitment  ( for miner tx the commitment is created from scratch
// 8 bytes blockheight to which this output belongs
// this function should always succeed or panic showing something is not correct
// NOTE: this function should only be called after all the tx and the block has been stored to DB
func (chain *Blockchain) writeOutputIndex(dbtx storage.DBTX, blockId crypto.Hash, index_start int64, hard_fork_version_current int64, sideBlock bool) (result bool) {
	// load the block
	bl, err := chain.LoadBlFromId(dbtx, blockId)
	if err != nil {
		logger.Warnf("No such block %s for writing output index", blockId)
		return
	}

	// load topo height
	height := chain.LoadHeightForBlId(dbtx, blockId)
	topoHeight := chain.LoadBlockTopologicalOrder(dbtx, blockId)

	rlog.Debugf("Writing Output Index for block %s height %d output index %d", blockId, height, index_start)

	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX, uint64(index_start))

	// calculate the rewards
	totalReward, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_MINERTX_REWARD)
	if err != nil {
		logger.Errorf("No reward of block %s", blockId)
		return
	}

	baseReward, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_BASEREWARD)
	if err != nil {
		logger.Errorf("No base of block %s", blockId)
		return
	}

	var currentShareCycleIndex int64
	var currentPoolCycleIndex int64
	cycleStartHeight := globals.GetCycleStartHeight()

	if height-cycleStartHeight >= 0 {
		currentShareCycleIndex = ((height - cycleStartHeight) / config.HOUR_CYCLE) + 1
		currentPoolCycleIndex = ((height - cycleStartHeight) / config.DAY_CYCLE) + 1
	}

	powReward, posReward, destroyed := chain.calcReward(dbtx, totalReward, baseReward, height, bl, sideBlock)
	chain.writeMinerTx(powReward, dbtx, bl, &index_start, height, topoHeight)
	if posReward > 0 {
		if height-cycleStartHeight < 0 {
			// note pass reward / 100 as param
			chain.writeStakeTx2(totalReward/100, dbtx, bl, &index_start, height, topoHeight)
			dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_BLOCKHAS_REWARD, 1)
		} else {
			posReward = 0
		}
	}

	var shareProfit, poolProfit uint64

	if !sideBlock && (height-cycleStartHeight-config.CYCLE_DELAY_HEIGHT) > 0 && (height-cycleStartHeight-config.CYCLE_DELAY_HEIGHT)%config.HOUR_CYCLE == 0 {
		err, stakeRewards := chain.getShareStakeRewards(dbtx, currentShareCycleIndex)
		if err != nil {
			logger.Errorf("get Share Stake Rewards err, block %s", blockId)
			return
		}

		shareProfit = chain.writeShareStakeTx(dbtx, bl, &index_start, height, topoHeight, stakeRewards)
	}

	if !sideBlock && (height-cycleStartHeight-config.CYCLE_DELAY_HEIGHT) > 0 && (height-cycleStartHeight-config.CYCLE_DELAY_HEIGHT)%config.DAY_CYCLE == 0 {
		err, stakeRewards := chain.getPoolStakeRewards(dbtx, currentPoolCycleIndex)
		if err != nil {
			logger.Errorf("get Pool Stake Rewards err, block %s", blockId)
			return
		}

		poolProfit = chain.writePoolStakeTx(dbtx, bl, &index_start, height, topoHeight, stakeRewards)
	}

	// fixme: real reward include bonus reward ???
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_REAL_MINERTX_REWARD, powReward+posReward+shareProfit+poolProfit)

	//dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_BONUS_POOL_INDEX)
	if globals.IsBonusHeight(height) && !sideBlock {
		err = chain.StoreRichPoolShareInfoRank(dbtx, height)
		if err != nil {
			logger.Errorf("Store stake pool stats err, block %s", blockId)
			return
		}

		currentBonusTxHeight := height - globals.GetVotingStartHeight() - globals.GetBonusDelayHeight()
		if currentBonusTxHeight > 0 && currentBonusTxHeight/globals.GetBonusLifetimeHeight() == 1 { //NOTE: ONLY FIRST/ONE SEASON
			//TODO: Closed pool bonus should be destroyed.
			_, destroyedBonus := chain.writePoolBonusTx(dbtx, blockId, bl.Timestamp, &index_start, height, topoHeight)
			destroyed += destroyedBonus
		}
	}

	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_BLOCK_DESTROYED_COINS, destroyed)
	alreadyDestroyed := chain.LoadAlreadyDestroyedCoinsForTopoIndex(dbtx, topoHeight-1)
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_ALREADY_DESTROYED_COINS, alreadyDestroyed+destroyed)

	// now loops through all the transactions, and store there ouutputs also
	// however as per client protocol, only process accepted transactions
	for i := 0; i < len(bl.TxHashes); i++ { // load all tx one by one
		if !chain.IsTxValid(dbtx, blockId, bl.TxHashes[i]) { // skip invalid TX
			rlog.Tracef(1, "bl %s tx %s ignored while building outputs index as per client protocol", blockId, bl.TxHashes[i])
			continue
		}
		rlog.Tracef(1, "bl %s tx %s is being used while building outputs index as per client protocol", blockId, bl.TxHashes[i])

		tx, err := chain.LoadTxFromId(dbtx, bl.TxHashes[i])
		if err != nil {
			panic(fmt.Errorf("Cannot load  tx for %x err %s", bl.TxHashes[i], err))
		}
		if tx.IsContract() {
			rlog.Tracef(1, "bl %s tx %s ignored while building outputs index as all contract transaction", blockId, bl.TxHashes[i])
		}

		indexWithinTx := uint64(0)
		var o globals.TXOutputData

		o.BLID = blockId // store block id
		o.TXID = bl.TxHashes[i]
		o.Height = uint64(height)
		sigType := uint64(tx.RctSignature.GetSigType())
		o.Block_Time = bl.Timestamp
		o.TopoHeight = topoHeight

		// TODO unlock specific outputs on specific height
		o.Unlock_Height = uint64(height) + config.NORMAL_TX_AMOUNT_UNLOCK

		// build the key image list and pack it
		for j := 0; j < len(tx.Vin); j++ {
			kImages := crypto.Key(tx.Vin[j].(transaction.TxinToKey).K_image)
			o.Key_Images = append(o.Key_Images, crypto.Key(kImages))
		}

		// zero out fields between tx
		o.Tx_Public_Key = crypto.Key(ZERO_HASH)
		o.PaymentID = o.PaymentID[:0]

		var tokenTx transaction.Transaction
		omniToken := &transaction.OmniToken{}

		if _, ok := tx.ExtraMap[transaction.TOKEN_TX]; ok {
			tokenTxBytes := tx.ExtraMap[transaction.TOKEN_TX].([]byte)
			tokenTx.DeserializeHeader(tokenTxBytes)
		}

		if _, ok := tx.ExtraMap[transaction.OMNI_TOKEN]; ok {
			omniToken = tx.ExtraMap[transaction.OMNI_TOKEN].(*transaction.OmniToken)
		}

		locked := tx.IsLocked()

		if omniToken.Flag > 0 {
			for k := uint64(0); k < uint64(len(tokenTx.Vout)); k++ {
				switch omniToken.Flag {
				case globals.ISSUE_TOKEN:
					ok := chain.writeIssueTokenTx(dbtx, bl, &index_start, height, topoHeight, omniToken, &tokenTx, k)
					if !ok {
						return
					}
				case globals.TRANSFER_TOKEN:
					ok := chain.writeTransferTokenTx(dbtx, blockId, bl, &index_start, hard_fork_version_current, height, topoHeight, omniToken, &tokenTx, k)
					if !ok {
						return
					}
				default:
					//  ...
				}
			}
		}

		// tx has been loaded, now lets get the vout
		for j := uint64(0); j < uint64(len(tx.Vout)); j++ {
			o.SigType = sigType
			o.Target = tx.Vout[j].Target
			o.InKey.Destination = tx.Vout[j].GetKey()

			o.InKey.Mask = crypto.Key(tx.RctSignature.OutPk[j].Mask)
			o.Amount = tx.Vout[j].Amount

			switch o.Target.(type) {
			case transaction.TxoutToBuyShare:
				o.TxType = globals.TX_TYPE_BUY_SHARE
				//o.InKey.Destination = o.Target.(transaction.TxoutToBuyShare).Key

				lockedAmount := tx.RctSignature.GetTxLockedAmount()
				o.InKey.Mask = ringct.ZeroCommitmentFromAmount(lockedAmount)
				o.Amount = lockedAmount
				o.SigType = 0
			case transaction.TxoutToRepoShare:
				o.TxType = globals.TX_TYPE_REPO_SHARE
			case transaction.TxoutToRegisterPool:
				o.TxType = globals.TX_TYPE_REGISTER_POOL

				lockedAmount := tx.RctSignature.GetTxLockedAmount()
				o.InKey.Mask = ringct.ZeroCommitmentFromAmount(lockedAmount)
				o.Amount = lockedAmount
				o.SigType = 0
			case transaction.TxoutToClosePool:
				o.TxType = globals.TX_TYPE_CLOSE_POOL
			case transaction.TxoutToKey:
				o.TxType = globals.TX_TYPE_NORMAL
			case transaction.TxoutToSubAddress:
				o.TxType = globals.TX_TYPE_NORMAL
			default:
				panic(fmt.Errorf("invalid vout"))
			}

			o.ECDHTuple = tx.RctSignature.ECdhInfo[j]

			o.Index_within_tx = indexWithinTx
			o.Index_Global = uint64(index_start)
			o.Unlock_Height = 0
			o.TxFee = tx.RctSignature.GetTXFee()

			if j == 0 && tx.UnlockTime != 0 { // only first output of a TX can be locked
				o.Unlock_Height = tx.UnlockTime
			}

			//if hard_fork_version_current >= 3 && o.Unlock_Height != 0 {
			if hard_fork_version_current >= 3 && o.Unlock_Height != 0 {
				if o.Unlock_Height < config.CRYPTONOTE_MAX_BLOCK_NUMBER {
					if o.Unlock_Height < (o.Height + 1000) {
						o.Unlock_Height = o.Height + 1000
					}
				} else {
					if o.Unlock_Height < (o.Block_Time + 12000) {
						o.Unlock_Height = o.Block_Time + 12000
					}
				}
			}

			if locked == true && j == 0 {
				rlog.Debugf("-------->Set Unlock Height %d txid %s", config.MAX_TX_AMOUNT_UNLOCK, tx.GetHash().String())
				o.Unlock_Height = config.MAX_TX_AMOUNT_UNLOCK
			}

			switch o.Target.(type) {
			case transaction.TxoutToClosePool:
				o.Unlock_Height = o.Height + config.CLOSE_POOL_TX_AMOUNT_UNLOCK
			default:
				//	...
			}

			// include the key image list in the first output itself
			// rest all the outputs donot contain the keyimage
			if j != 0 && len(o.Key_Images) > 0 {
				o.Key_Images = o.Key_Images[:0]
			}

			// store public key if present
			if _, ok := tx.ExtraMap[transaction.TX_PUBLIC_KEY]; ok {
				o.Tx_Public_Key = tx.ExtraMap[transaction.TX_PUBLIC_KEY].(crypto.Key)
			}

			// store payment IDs if present
			if _, ok := tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID]; ok {
				o.PaymentID = tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID].([]byte)
			} else if _, ok := tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID]; ok {
				o.PaymentID = tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID].([]byte)
			}

			if v, ok := tx.Vout[j].Target.(transaction.TxoutToSubAddress); ok {
				o.Tx_Public_Key = v.PubKey
			}

			serialized, err := msgpack.Marshal(&o)
			if err != nil {
				panic(err)
			}

			dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(uint64(index_start)), serialized)

			index_start++
			indexWithinTx++
		}

		//TODO:
		// lets add sc transactions in the output table
		// these transactions are similiar to  miner transactions, open in amount
		// smart contracts are live HF 4
		if hard_fork_version_current >= config.CONTRACT_FORK_VERSION {
			// token tx is actually a transfer record instead of output
			chain.writeTokenTx(dbtx, bl.TxHashes[i], blockId, &index_start, bl.Timestamp, height, topoHeight)

			fakeIndex := uint64(0)
			chain.writeContractTx(dbtx, bl.TxHashes[i], blockId, &fakeIndex, &index_start, bl.Timestamp, height, topoHeight)
		}
	}

	// store where the look up ends
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX_END, uint64(index_start))

	return true
}

func (chain *Blockchain) getPoolStakeRewards(dbtx storage.DBTX, currentPoolCycleIndex int64) (err error, stakeRewards []StakeReward) {
	poolSerialized, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_STAKE, PLANET_POOL_REWARD_BLOB, itob(uint64(currentPoolCycleIndex-1)))
	if err != nil {
		return
	}

	poolRewardInfos := make(map[int64]RewardInfo)
	err = msgpack.Unmarshal(poolSerialized, &poolRewardInfos)
	if err != nil {
		return
	}

	//stats Map
	poolRewards := make(map[crypto.Hash]uint64)
	for _, info := range poolRewardInfos { // each height
		for _, id := range info.Ids { // each pools
			if _, ok := poolRewards[id]; ok {
				poolRewards[id] += info.Amount
			} else {
				poolRewards[id] = info.Amount
			}
		}
	}

	//toArray
	for id, amount := range poolRewards {
		stakeReward := StakeReward{Id: id, Amount: amount}
		stakeRewards = append(stakeRewards, stakeReward)
	}

	//sort
	sort.Slice(stakeRewards, func(i, j int) bool {
		if stakeRewards[i].Amount != stakeRewards[j].Amount {
			return stakeRewards[i].Amount < stakeRewards[j].Amount
		} else {
			return bytes.Compare(stakeRewards[i].Id[:], stakeRewards[j].Id[:]) < 0
		}
	})

	return nil, stakeRewards
}

func (chain *Blockchain) getShareStakeRewards(dbtx storage.DBTX, currentShareCycleIndex int64) (err error, stakeRewards []StakeReward) {
	shareSerialized, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_STAKE, PLANET_SHARE_REWARD_BLOB, itob(uint64(currentShareCycleIndex-1)))
	if err != nil {
		return
	}

	shareRewardInfos := make(map[int64]RewardInfo)
	err = msgpack.Unmarshal(shareSerialized, &shareRewardInfos)
	if err != nil {
		return
	}

	//stats Map
	shareRewards := make(map[crypto.Hash]uint64)
	for _, info := range shareRewardInfos { // each height
		for _, id := range info.Ids { // each shares
			if _, ok := shareRewards[id]; ok {
				shareRewards[id] += info.Amount
			} else {
				shareRewards[id] = info.Amount
			}
		}
	}

	//toArray
	for id, amount := range shareRewards {
		stakeReward := StakeReward{Id: id, Amount: amount}
		stakeRewards = append(stakeRewards, stakeReward)
	}

	//sort
	sort.Slice(stakeRewards, func(i, j int) bool {
		if stakeRewards[i].Amount != stakeRewards[j].Amount {
			return stakeRewards[i].Amount < stakeRewards[j].Amount
		} else {
			return bytes.Compare(stakeRewards[i].Id[:], stakeRewards[j].Id[:]) < 0
		}
	})

	return nil, stakeRewards
}

// this will load the index  data for specific index
// this should be done while holding the chain lock,
// since during reorganisation we might  give out wrong keys,
// to avoid that pitfall take the chain lock
// NOTE: this function is now for internal use only by the blockchain itself
//
func (chain *Blockchain) loadOutputIndex(dbtx storage.DBTX, index uint64) (idata globals.TXOutputData, success bool) {
	success = false
	dataBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(index))

	if err != nil {
		logger.Warnf("err loadOutputIndex while loading output index data index = %d err %s", index, err)
		success = false
		return
	}

	err = msgpack.Unmarshal(dataBytes, &idata)
	if err != nil {
		rlog.Warnf("err while unmarshallin output index data index = %d  data_len %d err %s", index, len(dataBytes), err)
		success = false
		return
	}

	success = true
	return
}

func (chain *Blockchain) loadTokenOutputIndex(dbtx storage.DBTX, index uint64) (idata globals.TokenOutputData, success bool) {
	success = false
	dataBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(index))

	if err != nil {
		logger.Warnf("err loadTokenOutputIndex while loading output index data index = %d err %s", index, err)
		success = false
		return
	}

	err = msgpack.Unmarshal(dataBytes, &idata)
	if err != nil {
		rlog.Warnf("err while unmarshallin output index data index = %d  data_len %d err %s", index, len(dataBytes), err)
		success = false
		return
	}

	success = true
	return
}

// this will read the output index data but will not deserialize it
// this is exposed for rpcserver giving access to wallet
func (chain *Blockchain) ReadOutputIndex(dbtx storage.DBTX, index uint64) (data_bytes []byte, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			rlog.Warnf("Error obtaining read-only tx. Error opening writable TX, err %s", err)
			return
		}
		defer dbtx.Rollback()
	}

	data_bytes, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(index))
	if err != nil {
		rlog.Warnf("err ReadOutputIndex while loading output index data index = %d err %s", index, err)
		return
	}
	return data_bytes, err
}

func (chain *Blockchain) ReadTokenOutputIndex(dbtx storage.DBTX, index uint64) (data_bytes []byte, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			rlog.Warnf("Error obtaining read-only tx. Error opening writable TX, err %s", err)
			return
		}
		defer dbtx.Rollback()
	}

	data_bytes, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_OUTPUT_INDEX, GALAXY_OUTPUT_INDEX, itob(index))
	if err != nil {
		rlog.Warnf("err ReadTokenOutputIndex while loading output index data index = %d err %s", index, err)
		return
	}
	return data_bytes, err
}

// this function finds output index for the tx
// first find a block index , and get the start offset
// then loop the index till you find the key in the result
// if something is not right, we return 0
func (chain *Blockchain) FindTxOutputIndex(tx_hash crypto.Hash) (offset int64) {
	topoHeight := chain.LoadTxHeight(nil, tx_hash) // get height at which it's mined

	blockIds, err := chain.LoadBlockTopologicalOrderAtIndex(nil, topoHeight)
	if err != nil {
		rlog.Warnf("error while finding tx_output_index %s", tx_hash)
		return 0
	}

	blockIndexStart, _ := chain.GetBlockOutputIndex(nil, blockIds)

	bl, err := chain.LoadBlFromId(nil, blockIds)
	if err != nil {
		rlog.Warnf("Cannot load  block for %s err %s", blockIds, err)
		return
	}

	if tx_hash == bl.MinerTx.GetHash() { // miner tx is the beginning point
		return blockIndexStart
	}

	offset = blockIndexStart + 1            // shift by 1
	for i := 0; i < len(bl.TxHashes); i++ { // load all tx one by one
		// follow client protocol and skip some transactions
		if !chain.IsTxValid(nil, blockIds, bl.TxHashes[i]) { // skip invalid TX
			continue
		}

		if bl.TxHashes[i] == tx_hash {
			return offset
		}
		tx, err := chain.LoadTxFromId(nil, bl.TxHashes[i])
		if err != nil {
			rlog.Warnf("Cannot load  tx for %s err %s", bl.TxHashes[i], err)
		}

		// tx has been loaded, now lets get the vout
		voutCount := int64(len(tx.Vout))
		offset += voutCount
	}

	// we will reach here only if tx is linked to wrong block
	// this may be possible during reorganisation
	return -1
}
