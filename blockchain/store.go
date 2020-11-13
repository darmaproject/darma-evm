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

import (
	"errors"
	"fmt"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/vmihailenco/msgpack"
)
import "sync"
import "math/big"

//import "runtime/debug"
import "encoding/binary"

//import log "github.com/sirupsen/logrus"
import "github.com/golang/groupcache/lru"

import "github.com/darmaproject/darmasuite/storage"
import "github.com/darmaproject/darmasuite/block"
import "github.com/darmaproject/darmasuite/crypto"

//import "github.com/darmaproject/darmasuite/globals"
import "github.com/darmaproject/darmasuite/transaction"

/* this file implements the only interface which translates comands  to/from blockchain to storage layer *
 *
 *
 */

//var TOP_ID = []byte("TOP_ID")  // stores current TOP, only stores single value

//var BLOCK_ID = []byte("BLOCKS") // stores blocks
//var CHAIN = []byte("CHAIN")    // this stores the actual chain, parents keeps child list, starts from genesis block

var BLOCKCHAIN_UNIVERSE = []byte("U") //[]byte("BLOCKCHAIN_UNIVERSE") // all block chain data is store in this BLOCKCHAIN_UNIVERSE

// there are only 8 galaxies
var GALAXY_BLOCK = []byte("GB")
var GALAXY_TRANSACTION = []byte("GTX")           // []byte("TRANSACTION")
var GALAXY_TRANSACTION_VALIDITY = []byte("GTXV") //[]byte("TRANSACTIONVALIDITY")
var GALAXY_VOTE_VALIDITY = []byte("GVV")
var GALAXY_KEYIMAGE = []byte("GKI") //[]byte("KEYIMAGE")
var GALAXY_STAKE = []byte("GSK")
var GALAXY_OMNI_TOKEN = []byte("GOT")

var GALAXY_TOPOLOGICAL_ORDER = []byte("GT")  // []byte("TOPOLOGICAL")       // stores  block to topo index mapping
var GALAXY_TOPOLOGICAL_INDEX = []byte("GTI") //[]byte("TOPOLOGICAL_INDEX") // stores topological index  to block mapping

//2 galaxies store inverse mapping
var GALAXY_HEIGHT = []byte("GH") //[]byte("HEIGHT")             // height to block id mapping
var GALAXY_HEIGHT_EXPIRE_SHARE = []byte("GHES")
var GALAXY_OUTPUT_INDEX = []byte("GOI") //[]byte("OUTPUT_INDEX") // output index to wallet data for blockchain verification and wallets
var GALAXY_BONUS_POOL_STATS = []byte("GBPS")

// these store unstructured data
var GALAXY_KEYVALUE = []byte("GKV") //[]byte("KEYVALUE") // used to store simple data

//contract
var GALAXY_CONTRACT = []byte("GSC")

// the various attributes stored in keyvalue
var TOP_HEIGHT = []byte("TOP_HEIGHT")   // stores current TOP HEIGHT, only stores single value
var TOPO_HEIGHT = []byte("TOPO_HEIGHT") // stores current TOPO HEIGHT, only stores single value
var TIPS = []byte("TIPS")               // this stores tips

// the unique TXID or block ID becomes the solar system , which is common and saves lot of space

// individual attributes becomes the planets
// individual attributes should be max  1 or 2 chars long, as they will be repeated millions of times and storing a static string millions of times shows foolishness
// a block table has the following columns/attributes
// CREATE TABLE IF NOT EXISTS BLOCKS (ID CHAR(64) PRIMARY KEY, SERIALIZED BLOB, height BIGINT  default -1, INDEX heighti (height), past TEXT default "", future TEXT default "", SIZE BIGINT default -1, DIFFICULTY DECIMAL(65,0) default 0, CDIFFICULTY DECIMAL(65,0) default 0, TIMESTAMP BIGINT default 1, OUTPUT_INDEX_START BIGINT default 0, OUTPUT_INDEX_END BIGINT default 0, CCOINS DECIMAL(32,0) default 0, BASEREWARD DECIMAL(32,0) default 0, TXFEES DECIMAL(32,0) default 0, TX_COUNT int default 0);

// A TOPO ordering has the following
//CREATE TABLE IF NOT EXISTS TOPO (ID CHAR(64) PRIMARY KEY,topoheight BIGINT  default -1, INDEX topoheighti (topoheight));

// KEYVALUE table has the followiing
//CREATE TABLE IF NOT EXISTS KEYVALUE (ID VARCHAR(1024),value BLOB ));

var PLANET_BLOB = []byte("BLOB")     //it shows serialised block
var PLANET_HEIGHT = []byte("HEIGHT") // contains height
var PLANET_PARENT = []byte("PARENT") // parent of block
var PLANET_PAST = []byte("PAST")     // past of block
var PLANET_FUTURE = []byte("FUTURE") // future of block only 1 level

//var PLANET_HEIGHT_BUCKET = []byte("H")     // contains all blocks of same height
//var PLANET_SETTLE_STATUS = []byte("S")     // contains whether the block is settled

var PLANET_SIZE = []byte("SIZE")                      // sum of block + all txs
var PLANET_ALREADY_GENERATED_COINS = []byte("CCOINS") // all coins generated till this block
var PLANET_OUTPUT_INDEX = []byte("OINDEX")            // tx outputs indexing starts from here for this block
var PLANET_OUTPUT_INDEX_END = []byte("OINDEXEND")     // tx outputs indexing ends here ( this is excluded )
var PLANET_CUMULATIVE_DIFFICULTY = []byte("CDIFF")    //[]byte("CDIFFICULTY") // Cumulative difficulty
var PLANET_DIFFICULTY = []byte("DIFF")                //[]byte("DIFFICULTY")             // difficulty
var PLANET_BASEREWARD = []byte("BREWARD")             // base reward of a block  ( new coins generated)
var PLANET_MINERTX_REWARD = []byte("REWARD")          //reward for minertx is stored here ( base reward + collected fees after client protocol run)
var PLANET_REAL_MINERTX_REWARD = []byte("RREWARD")
var PLANET_BLOCK_DESTROYED_COINS = []byte("DESTROYED")
var PLANET_ALREADY_DESTROYED_COINS = []byte("CDESTROYED")
var PLANET_BLOCKHAS_REWARD = []byte("BLHSRW") //1=true,0=false
var PLANET_POOL_CLOSEDHEIGHT = []byte("PLCHT")

//var PLANET_BONUS_INDEX = []byte("BONUS") 			// each block retain for bonus
//var PLANET_BONUS_POOL_INDEX = []byte("BP") 			// bonus pool total
var PLANET_POOLSTATS_BLOB = []byte("POOLSTATS")

//TODO: should remove this
//dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, itob(uint64(heightCurrent)), PLANET_POOLSTATS_BLOB, byteData)

var PLANET_TIMESTAMP = []byte("TS") // []byte("TIMESTAMP")

// the TX has the following attributes
var PLANET_TX_BLOB = []byte("BLOB")          // contains serialised  TX , this attribute is also found in BLOCK where
var PLANET_TX_MINED_IN_BLOCK = []byte("MBL") //[]byte("MINERBLOCK") // which blocks mined this tx, stores topo height of mined block
var PLANET_TX_MINED = []byte("MIN")          // all blocks where tx is mined in in
var PLANET_TX_SIZE = []byte("SIZE")
var PLANET_STAKE_TX = []byte("STAKETX")
var PLANET_POOLSTAKE_TX = []byte("PLSTAKETX")
var PLANET_SHARESTAKE_TX = []byte("SRSTAKETX")
var PLANET_BONUS_TX = []byte("BONUSTX")

var PLANET_REWARD_FIX = []byte("REWARDFIX")
var PLANET_PROFIT_FIX = []byte("PROFIT")
var PLANET_EXPIRE_FIX = []byte("EXPIREFIX")
var PLANET_CLOSE_HEIGHT_FIX = []byte("CLOSEHEIGHTFIX")

var PLANET_STAKEPOOL_BLOB = []byte("POOL")
var PLANET_STAKEPOOL_NAME = []byte("PNAME")
var PLANET_STAKESHARE_BLOB = []byte("SHARE")
var PLANET_STAKESHARE_ROUND = []byte("ROUND")
var PLANET_TX2SHARE = []byte("TX2SHARE")
var PLANET_STAKEPOOL_STATS = []byte("PSPSS")

var PLANET_ISSUE_OMNI_TOKEN_BLOB = []byte("PIOTB")

var PLANET_CONTRACT_RESULT = []byte("SCRET")
var PLANET_CONTRACT_TX_RECEIPT = []byte("SCRECEIPT")
var PLANET_CONTRACT_ORIGIN = []byte("SCORIGIN")
var PLANET_CONTRACT_ADDR = []byte("SCID")
var PLANET_CONTRACT_TXID = []byte("SCTX")
var PLANET_CONTRACT_TRANSFER_BLOB = []byte("SCTB")
var PLANET_TOKEN_TRANSFER_BLOB = []byte("SCTTB")
var PLANET_STATEROOT_BLOB = []byte("SCST")
var PLANET_CONTRACT_REFUNDGAS_BLOB = []byte("SCGAS")

var PLANET_POOL_REWARD_BLOB = []byte("POLRWD")
var PLANET_SHARE_REWARD_BLOB = []byte("SHRRWD")

// the universe concept is there, as we bring in smart contracts, we will give each of them a universe to play within
// while communicating with external universe

/*
func (chain *Blockchain) Store_Main_Chain(parent_id  crypto.Hash, child_id crypto.Hash){
   err := chain.store.StoreObject(BLOCKCHAIN_UNIVERSE,GALAXY_BLOCK,parent_id[:],PLANET_CHILD, child_id[:] )
     _ = err
 }

func (chain *Blockchain) Load_Main_Chain(parent_id  crypto.Hash) (child_id crypto.Hash ){
	var err error
    // store OO to TXID automatically
     object_data,err = chain.store.LoadObject(BLOCKCHAIN_UNIVERSE,GALAXY_BLOCK,parent_id[:],PLANET_CHILD )

    if err != nil {
    	return child_id,err
    }

    if len(object_data) == 0 {
    	return child_id, fmt.Errorf("No Block at such Height %d", Height)
    }

    if len(object_data) != 32 {
    	panic("Database corruption, invalid block hash ")
    }

    copy(child_id[:],object_data[:32])

_ = err

  return child_id

}

*/

/*
// check whether the block has a child
func (chain *Blockchain) Does_Block_Have_Child(block_id crypto.Hash) bool {
	var err error
	object_data, err := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, block_id[:], PLANET_CHILD)

	if err != nil || len(object_data) == 0 {
		return false
	}
	if len(object_data) != 32 {
		panic("Database corruption, invalid block hash ")
	}
	return true
}

// load the main child
func (chain *Blockchain) Load_Block_Child(parent_id crypto.Hash) (child_id crypto.Hash) {
	if !chain.Does_Block_Have_Child(parent_id) {
		panic("Block does not have a child")
	}
	object_data, _ := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, parent_id[:], PLANET_CHILD)

	copy(child_id[:], object_data)
	return
}

*/

// stores a blocks topological order
func (chain *Blockchain) StoreBlockTopologicalOrder(dbtx storage.DBTX, blid crypto.Hash, index_pos int64) {
	if dbtx == nil {
		panic("TX cannot be nil")
	}

	// logger.Warnf("Storing topo order %s  %d", blid,index_pos)

	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TOPOLOGICAL_ORDER, GALAXY_TOPOLOGICAL_ORDER, blid[:], uint64(index_pos))
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TOPOLOGICAL_INDEX, GALAXY_TOPOLOGICAL_INDEX, itob(uint64(index_pos)), blid[:])
}

// since topological order might mutate, instead of doing cleanup, we double check the pointers
func (chain *Blockchain) Is_Block_Topological_order(dbtx storage.DBTX, blid crypto.Hash) bool {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return false
		}

		defer dbtx.Rollback()

	}

	index_pos, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TOPOLOGICAL_ORDER, GALAXY_TOPOLOGICAL_ORDER, blid[:])
	if err != nil || index_pos >= 0x7fffffffffffffff {
		return false
	}
	blid_at_pos, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, int64(index_pos))

	if err != nil {
		return false
	}

	if blid == blid_at_pos {
		return true
	}
	return false
}

func (chain *Blockchain) LoadBlockTopologicalOrder(dbtx storage.DBTX, blid crypto.Hash) int64 {
	/* if !chain.Is_Block_Topological_order(blid) {  // removed as optimisation
	       return 0xffffffffffffffff
	   }
	*/
	// give highest possible topo order for a block, if it does NOT exist

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf(" Error opening writable TX, err %s", err)
			return 0x7fffffffffffffff
		}

		defer dbtx.Rollback()

	}

	index_pos, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TOPOLOGICAL_ORDER, GALAXY_TOPOLOGICAL_ORDER, blid[:])
	if err != nil {
		logger.Warnf("%s DOES  NOT HAVE base order stored", blid)
		return 0x7fffffffffffffff
	}
	return int64(index_pos)
}

func (chain *Blockchain) LoadBlockTopologicalOrderAtIndex(dbtx storage.DBTX, index_pos int64) (hash crypto.Hash, err error) {

	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()

	}

	objectData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TOPOLOGICAL_INDEX, GALAXY_TOPOLOGICAL_INDEX, itob(uint64(index_pos)))
	if err != nil {
		return hash, err
	}

	if len(objectData) == 0 {
		return hash, fmt.Errorf("No Block at such topo index %d", index_pos)
	}

	if len(objectData) != 32 {
		panic("Database corruption, invalid block hash ")
	}

	copy(hash[:], objectData[:32])
	return hash, nil
}

/*

// changes  or set child block of a parent
// there can be only 1 child, rest all are alternatives and stored as
func (chain *Blockchain) Store_Block_Child(parent_id crypto.Hash, child_id crypto.Hash) {
	err := chain.store.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, parent_id[:], PLANET_CHILD, child_id[:])

	// load block children
	_ = err
}

// while store children
func (chain *Blockchain) Store_Block_Children(parent_id crypto.Hash, children []crypto.Hash, exclude_child crypto.Hash) {
	var children_bytes []byte
	for i := range children {
		if children[i] != exclude_child { // exclude main child
			children_bytes = append(children_bytes, children[i][:]...)
		}
	}
	err := chain.store.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, parent_id[:], PLANET_CHILDREN, children_bytes)
	_ = err
}

func (chain *Blockchain) Load_Block_Children(parent_id crypto.Hash) (children []crypto.Hash) {
	var child_hash crypto.Hash
	if !chain.Does_Block_Have_Child(parent_id) { // block doesnot have a child, so it cannot have children
		return
	}
	// we are here means parent does have child
	children = append(children, chain.Load_Block_Child(parent_id))

	// check for children
	children_bytes, _ := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, parent_id[:], PLANET_CHILDREN)

	if len(children_bytes)%32 != 0 {
		panic(fmt.Sprintf("parent does not have child hash in multiples of 32, block_hash %s", parent_id))
	}
	for i := 0; i < len(children_bytes); i = i + 32 {
		copy(child_hash[:], children_bytes[i:i+32])
		children = append(children, child_hash)
	}
	return children
}
*/

// store a tx
// this only occurs when a tx has been mined or a a reorganisation is in progress
// stores a height to show at what height it has been mined
func (chain *Blockchain) StoreTX(dbtx storage.DBTX, tx *transaction.Transaction) {
	if dbtx == nil {
		panic(fmt.Sprintf("Could NOT add TX to chain. Error opening writable TX, err "))
	}

	hash := tx.GetHash()
	serialized := tx.Serialize()
	err := dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, hash[:], PLANET_TX_BLOB, serialized)
	// store size of tx
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, hash[:], PLANET_TX_SIZE, uint64(len(serialized)))
	//dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, hash[:], PLANET_TX_MINED_IN_BLOCK, uint64(TopoHeight))

	_ = err
}

func (chain *Blockchain) StoreTxHeight(dbtx storage.DBTX, txhash crypto.Hash, TopoHeight int64) {
	if dbtx == nil {
		panic(fmt.Sprintf("Could NOT add TX to chain. Error opening writable TX"))

	}
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txhash[:], PLANET_TX_MINED_IN_BLOCK, uint64(TopoHeight))
}

/*

func (chain *Blockchain) Store_TX_Miner(txhash crypto.Hash, block_id crypto.Hash) {
	// store block id  which mined this tx
	err := chain.store.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txhash[:], PLANET_TX_MINED_IN_BLOCK, block_id[:])
	_ = err
}
*/

func (chain *Blockchain) LoadTxSize(dbtx storage.DBTX, txhash crypto.Hash) uint64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}
		defer dbtx.Rollback()
	}

	// store block id  which mined this tx
	size, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txhash[:], PLANET_TX_SIZE)
	if err != nil {
		logger.Warnf("Size not stored for tx %s", txhash)
	}

	return size
}

// load height at which a specific tx was mined
func (chain *Blockchain) LoadTxHeight(dbtx storage.DBTX, txhash crypto.Hash) int64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return -1
		}

		defer dbtx.Rollback()
	}

	height, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txhash[:], PLANET_TX_MINED_IN_BLOCK)
	if err != nil {
		logger.Warnf("Error while querying height for tx %s", txhash)
	}
	return int64(height)
}

// BUG we should be able to delete any arbitrary key
// since a tx mined by one block, can be back in pool after chain reorganises

// TODO the miner tx should be extracted ands stored from somewhere else
// NOTE: before storing a block, its transactions must be stored
func (chain *Blockchain) StoreBL(dbtx storage.DBTX, bl *block.Block) {
	if dbtx == nil {
		panic(fmt.Sprintf("Could NOT store block to DB.  nil dbtx"))
	}
	// store block height BHID automatically
	hash := bl.GetHash()

	// we should deserialize the block here
	serializedBytes := bl.Serialize() // we are storing the miner transactions within
	err := dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_BLOB, serializedBytes)

	height := chain.CalculateHeightAtTips(dbtx, bl.Tips)

	// store new height
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_HEIGHT, uint64(height))

	// this will be ignored by SQL backend as it can be recreated later on
	//dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, append(itob(uint64(height)),PLANET_HEIGHT_BUCKET...) , hash[:], []byte(""))

	blocks_at_height := chain.GetBlocksAtHeight(dbtx, height)
	blocks_at_height = append(blocks_at_height, hash)

	blocks_at_height_bytes := make([]byte, 0, len(blocks_at_height)*32)
	for j := range blocks_at_height {
		blocks_at_height_bytes = append(blocks_at_height_bytes, blocks_at_height[j][:]...)
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_HEIGHT, PLANET_HEIGHT, itob(uint64(height)), blocks_at_height_bytes)

	// store timestamp separatly is NOT necessary
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_TIMESTAMP, bl.Timestamp)

	// create empty past and future buckets
	// past will be empty for genesis block
	// future may be empty in case of discarded blocks
	//dbtx.CreateBucket(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, append(hash[:],PLANET_PAST...))
	//dbtx.CreateBucket(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, append(hash[:],PLANET_FUTURE...))

	// store Past tips into a separate bucket
	// information is stored within the buckets keys itself
	past_bytes := make([]byte, 0, len(bl.Tips)*32)
	for i := range bl.Tips {
		past_bytes = append(past_bytes, bl.Tips[i][:]...)
	}
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_PAST, past_bytes)

	// store future mixed for easier processing later on
	for i := range bl.Tips {
		future := chain.Get_Block_Future(dbtx, bl.Tips[i])
		future = append(future, hash)

		future_bytes := make([]byte, 0, len(future)*32)
		for j := range future {
			future_bytes = append(future_bytes, future[j][:]...)
		}

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, bl.Tips[i][:], PLANET_FUTURE, future_bytes)
	}

	// calculate cumulative difficulty at last block

	if len(bl.Tips) == 0 { // genesis block has no parent
		//cumulative_difficulty = 1
		//difficulty_of_current_block = 1

		difficulty_of_current_block := new(big.Int).SetUint64(1) // this is never used, as genesis block is a sync block, only its cumulative difficulty is used
		cumulative_difficulty := new(big.Int).SetUint64(1)       // genesis block cumulative difficulty is 1

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY, cumulative_difficulty.Bytes())

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_DIFFICULTY, difficulty_of_current_block.Bytes())

		// chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_DIFFICULTY, difficulty_of_current_block)

	} else {
		difficulty_of_current_block := chain.GetDifficultyAtTips(dbtx, bl.Tips)
		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_DIFFICULTY, difficulty_of_current_block.Bytes())

		// NOTE: difficulty must be stored before cumulative difficulty calculation, since it is used while calculating Cdiff

		base, base_height := chain.findCommonBase(dbtx, bl.Tips)
		work_map, cumulative_difficulty := chain.FindTipWorkScore(dbtx, hash, base, base_height)

		_ = work_map
		/*logger.Infof("workmap base ")
		  for k,v := range work_map{
		   logger.Infof("%s %d",k,v)
		  }*/

		dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY, cumulative_difficulty.Bytes())

		// chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY, cumulative_difficulty)

		/*gbl:=GenerateGenesisBlock()

		     // TODO BUG BUG BUG  cumulative_difficulty neeeds to calculated against a previous sync point , otherise
		     // we are DOSing  ourselves
		   work_map_gbl, cumulative_difficulty_gbl  := chain.FindTipWorkScore(hash, gbl.GetHash(),0)

		   if cumulative_difficulty != cumulative_difficulty_gbl {

		    logger.Warnf("DIFFICULTY mismatch for %s hash   from base %s  %d from genesis %d", base,cumulative_difficulty,cumulative_difficulty_gbl)

		    logger.Infof("workmap base ")
		    for k,v := range work_map{
		     logger.Infof("%s %d",k,v)
		    }

		    logger.Infof("workmap genesis base ")
		    for k,v := range work_map_gbl{
		     logger.Infof("%s %d",k,v)
		    }

		}*/

	}

	// the cumulative difficulty  includes  self difficulty
	// total_difficulty = cumulative_difficulty //+ difficulty_of_current_block

	//chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY, total_difficulty)
	//chain.store.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY, hash[:])

	// cdifficulty_bytes, err := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY)

	// logger.Infof("cumulative difficulty of %s is %d", hash, total_difficulty)

	/*
			// total size of block = size of miner_tx + size of all transactions in block ( excludind miner tx)

			size_of_block := uint64(0) //len(bl.Miner_tx.Serialize()))
			for i := 0; i < len(bl.TxHashes); i++ {
				size_of_tx := chain.LoadTxSize(bl.TxHashes[i])
				size_of_block += size_of_tx
			}
			chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_SIZE, size_of_block)

			// calculated position of vouts in global indexs
			index_pos := uint64(0)
			if hash != globals.Config.GenesisBlockHash {
				// load index pos from last block + add count of vouts from last block
				indexPos = chain.Get_BlockOutputIndex(bl.PrevHash)
				voutCountPrevBlock := chain.BlockCountVout(bl.PrevHash)
				indexPos += voutCountPrevBlock
			}
			chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_OUTPUT_INDEX, index_pos)
			//logger.Debugf("height %d   output index %d",height, index_pos)

			total_fees := uint64(0)
			for i := 0; i < len(bl.TxHashes); i++ {
				tx, _ := chain.LoadTxFromId(bl.TxHashes[i])
				total_fees += tx.RctSignature.Get_TX_Fee()
			}

			total_reward :=  uint64(0) //bl.Miner_tx.Vout[0].Amount
			base_reward := total_reward - total_fees
			chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_BASEREWARD, base_reward)

			already_generated_coins := uint64(0)
			if hash != globals.Config.GenesisBlockHash { // genesis block has no parent
				already_generated_coins = chain.LoadAlreadyGeneratedCoinsForBlId(bl.Prev_Hash)
			} else {
				base_reward = config.COIN_UNIT // trigger the bug to fix coin calculation, see comments in emission
			}
			already_generated_coins += base_reward
			chain.store.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_ALREADY_GENERATED_COINS, already_generated_coins)

			// also extract and store the miner tx separetly, fr direct querying purpose
		        //TODO miner TX should be created using deterministic random number and saved
			//chain.StoreTX(&bl.Miner_tx, height)

	*/
	_ = err
}

var pastCache = lru.New(10240)
var pastCacheLock sync.Mutex

// all the immediate past of a block
func (chain *Blockchain) GetBlockPast(dbtx storage.DBTX, hash crypto.Hash) (blocks []crypto.Hash) {
	pastCacheLock.Lock()
	defer pastCacheLock.Unlock()

	if keysi, ok := pastCache.Get(hash); ok {
		keys := keysi.([]crypto.Hash)
		blocks = make([]crypto.Hash, len(keys))
		for i := range keys {
			copy(blocks[i][:], keys[i][:])
		}
		return
	}

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	// serve from store
	pastBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_PAST)
	if err != nil {
		return
	}
	blocks = make([]crypto.Hash, len(pastBytes)/32, len(pastBytes)/32)

	for i := 0; i < len(pastBytes)/32; i++ {
		copy(blocks[i][:], pastBytes[i*32:(i*32)+32])
	}

	cacheCopy := make([]crypto.Hash, len(blocks), len(blocks))
	for i := range blocks {
		cacheCopy[i] = blocks[i]
	}

	//set in cache
	pastCache.Add(hash, cacheCopy)

	return
}

// a block withput a future is called tip
func (chain *Blockchain) Get_Block_Future(dbtx storage.DBTX, hash crypto.Hash) (blocks []crypto.Hash) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	// deserialize future
	futureBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_FUTURE)
	if err != nil {
		return
	}
	blocks = make([]crypto.Hash, len(futureBytes)/32, len(futureBytes)/32)

	for i := 0; i < len(futureBytes)/32; i++ {
		copy(blocks[i][:], futureBytes[i*32:(i*32)+32])
	}

	return
}

func (chain *Blockchain) LoadTxFromId(dbtx storage.DBTX, hash crypto.Hash) (*transaction.Transaction, error) {
	var tx transaction.Transaction

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return nil, err
		}

		defer dbtx.Rollback()

	}

	txData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, hash[:], PLANET_TX_BLOB)
	if err != nil {
		return nil, err
	}

	// we should deserialize the block here
	err = tx.DeserializeHeader(txData)
	if err != nil {
		logger.Printf("fError deserialiing tx, block id %s len(data) %d data %x", hash[:], len(txData), txData)
		return nil, err
	}

	return &tx, nil
}

func (chain *Blockchain) LoadBlFromId(dbtx storage.DBTX, hash [32]byte) (*block.Block, error) {
	var bl block.Block
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return nil, err
		}

		defer dbtx.Rollback()

	}

	blockData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_BLOB)
	if err != nil {
		return nil, err
	}

	if len(blockData) == 0 {
		return nil, fmt.Errorf("Block not found in DB")
	}

	// we should deserialize the block here
	err = bl.Deserialize(blockData)
	if err != nil {
		logger.Warnf("fError deserialiing block, block id %s len(data) %d data %x", hash[:], len(blockData), blockData)
		return nil, err
	}
	return &bl, nil
}

func (chain *Blockchain) LoadHeightForBlId(dbtx storage.DBTX, hash crypto.Hash) (Height int64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT open readonly TX, err %s", err)
			return -1
		}

		defer dbtx.Rollback()
	}

	if hash == ZERO_HASH { // handle special case for  genesis
		return 0
	}

	objectData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_HEIGHT)
	if err != nil {
		logger.Warnf("Error while querying height for block %s, err %s", hash, err)
		return
	}

	if len(objectData) == 0 {
		//return hash, fmt.Errorf("No Height for block %x", hash[:])
		return
	}

	if len(objectData) != 8 {
		panic("Database corruption, invalid block hash ")
	}

	Height = int64(binary.BigEndian.Uint64(objectData))

	return int64(Height)
}

func (chain *Blockchain) LoadBlockTimestamp(dbtx storage.DBTX, hash crypto.Hash) int64 {

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return -1
		}

		defer dbtx.Rollback()

	}

	timestamp, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_TIMESTAMP)
	if err != nil {
		logger.Warnf("Error while querying timestamp for block %s", hash)
		logger.Panicf("Error while querying timestamp for block %s", hash)

	}

	return int64(timestamp)
}

func (chain *Blockchain) LoadBlockCumulativeDifficulty(dbtx storage.DBTX, hash crypto.Hash) *big.Int {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return new(big.Int).SetInt64(0)
		}

		defer dbtx.Rollback()
	}

	cdifficultyBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY)
	//cdifficulty, err := chain.store.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_CUMULATIVE_DIFFICULTY)
	if err != nil {
		logger.Warnf("Error while querying cumulative difficulty for block %s", hash)
		logger.Panicf("Error while querying cumulative difficulty for block %s", hash)
	}

	return new(big.Int).SetBytes(cdifficultyBytes)
}

func (chain *Blockchain) LoadBlockDifficulty(dbtx storage.DBTX, hash crypto.Hash) *big.Int {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return new(big.Int).SetInt64(0)
		}

		defer dbtx.Rollback()
	}

	//difficulty, err := chain.store.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_DIFFICULTY)
	difficulty_bytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_DIFFICULTY)
	if err != nil {
		logger.Warnf("Error while querying difficulty for block %s", hash)
		logger.Panicf("Error while querying difficulty for block %s", hash)
	}

	//return difficulty
	return new(big.Int).SetBytes(difficulty_bytes)
}

func (chain *Blockchain) LoadBlockBaseReward(dbtx storage.DBTX, hash crypto.Hash) uint64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}

	block_reward, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_BASEREWARD)
	if err != nil {
		logger.Warnf("Error while querying base_reward for block %s", hash)
	}

	return block_reward
}

// inluding reward + fees
func (chain *Blockchain) LoadBlockRealReward(dbtx storage.DBTX, hash crypto.Hash) uint64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}

	block_reward, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_REAL_MINERTX_REWARD)
	if err != nil {
		logger.Warnf("Error while querying real reward for block %s", hash)
	}

	return block_reward
}

func (chain *Blockchain) LoadAlreadyDestroyedCoinsForTopoIndex(dbtx storage.DBTX, index int64) uint64 {
	if index < 0 { // fix up pre-genesis
		return 0
	}

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}
	// first find the block at the topo index
	hash, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, index)
	if err != nil {
		return 0
	}

	alreadyDestroyedCoins, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_ALREADY_DESTROYED_COINS)
	if err != nil {
		logger.Warnf("Error while querying already destroyed coins for block %s", hash)
	}

	return alreadyDestroyedCoins
}

func (chain *Blockchain) LoadCirculationSupply(dbtx storage.DBTX, index int64) uint64 {
	if index < 0 { // fix up pre-genesis
		return 0
	}

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}
	// first find the block at the topo index
	hash, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, index)
	if err != nil {
		return 0
	}

	already_generated_coins, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_ALREADY_GENERATED_COINS)
	if err != nil {
		logger.Warnf("Error while querying already generated coins for block %s", hash)
	}

	height := chain.LoadHeightForBlId(dbtx, hash)
	if height < globals.GetVotingStartHeight() {
		return already_generated_coins
	}

	beforeVoting := chain.LoadAlreadyGeneratedCoinsForTopoIndex(dbtx, 148319) // height 120319
	if already_generated_coins <= beforeVoting {
		panic(fmt.Errorf("block %s, already %d, before voting %d", hash, already_generated_coins, beforeVoting))
	}

	reserved := (already_generated_coins - beforeVoting) / 100 * config.RATE_BONUS

	return already_generated_coins - reserved
}

func (chain *Blockchain) LoadAlreadyGeneratedCoinsForTopoIndex(dbtx storage.DBTX, index int64) uint64 {
	if index < 0 { // fix up pre-genesis
		return 0
	}

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}
	// first find the block at the topo index
	hash, err := chain.LoadBlockTopologicalOrderAtIndex(dbtx, index)
	if err != nil {
		return 0
	}

	already_generated_coins, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_ALREADY_GENERATED_COINS)
	if err != nil {
		logger.Warnf("Error while querying already generated coins for block %s", hash)
	}

	return already_generated_coins
}

func (chain *Blockchain) LoadPoolStatsForHeight(dbtx storage.DBTX, index int64) (PoolVoteInfo, error) {
	if index < 0 { // fix up pre-genesis
		return nil, errors.New("index lt zero")
	}

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return nil, err
		}

		defer dbtx.Rollback()
	}

	marshalData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_STAKE, PLANET_STAKEPOOL_STATS, itob(uint64(index)))
	if err != nil {
		logger.Warnf("Error while load pool stats for block %d", index)
	}

	poolVoteInfo := make(PoolVoteInfo, 0)

	err = msgpack.Unmarshal(marshalData, &poolVoteInfo)
	if len(marshalData) > 0 && err != nil {
		return nil, err
	}

	return poolVoteInfo, nil
}

func (chain *Blockchain) GetAlreadyGeneratedCoinsBetween(dbtx storage.DBTX, beginHeight int64, endHeight int64) uint64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}

	latestBlocks := chain.GetBlocksAtHeight(dbtx, beginHeight)
	var latestMainBlock crypto.Hash
	var beginTopoHeight int64
	for _, latestBlock := range latestBlocks {
		if chain.Is_Block_Topological_order(dbtx, latestBlock) {
			beginTopoHeight = chain.LoadBlockTopologicalOrder(dbtx, latestBlock)
			if !chain.isblockSideBlock(dbtx, latestBlock, beginTopoHeight) {
				latestMainBlock = latestBlock
				break
			}
		}
	}
	if latestMainBlock == crypto.ZeroHash {
		panic(fmt.Errorf("No main block at height %d", beginHeight))
	}

	beginGeneratedCoins := chain.LoadAlreadyGeneratedCoinsForTopoIndex(dbtx, beginTopoHeight)
	beginBlockReward := chain.LoadBlockBaseReward(dbtx, latestMainBlock)

	currentBlocks := chain.GetBlocksAtHeight(dbtx, endHeight)
	var currentMainBlock crypto.Hash
	var endTopoHeight int64
	for _, currentBlock := range currentBlocks {
		if chain.Is_Block_Topological_order(dbtx, currentBlock) {
			endTopoHeight = chain.LoadBlockTopologicalOrder(dbtx, currentBlock)
			if !chain.isblockSideBlock(dbtx, currentBlock, endTopoHeight) {
				currentMainBlock = currentBlock
				break
			}
		}
	}
	if currentMainBlock == crypto.ZeroHash {
		panic(fmt.Errorf("No main block at height %d", endHeight))
	}

	endGeneratedCoins := chain.LoadAlreadyGeneratedCoinsForTopoIndex(dbtx, endTopoHeight)
	endBlockReward := chain.LoadBlockBaseReward(dbtx, currentMainBlock)

	return (endGeneratedCoins - endBlockReward) - (beginGeneratedCoins - beginBlockReward)
}

func (chain *Blockchain) Load_Block_Size(dbtx storage.DBTX, hash crypto.Hash) uint64 {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0
		}

		defer dbtx.Rollback()
	}

	size, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_SIZE)
	if err != nil {
		logger.Warnf("Error while querying size for block %s", hash)
	}

	return size
}

/*
func (chain *Blockchain) Load_Block_Parent_ID(hash crypto.Hash) crypto.Hash {
	var parent_id crypto.Hash
	object_data, err := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, hash[:], PLANET_PARENT)

	if err != nil || len(object_data) != 32 {
		logger.Warnf("Error while querying parent id for block %s", hash)
	}
	copy(parent_id[:], object_data)

	return parent_id
}
*/

// store current top id
/*
func (chain *Blockchain) Store_TOP_ID(hash crypto.Hash) {
	chain.store.StoreObject(BLOCKCHAIN_UNIVERSE, TOP_ID, TOP_ID, TOP_ID, hash[:])
}

// crash if something is not correct
func (chain *Blockchain) Load_TOP_ID() (hash crypto.Hash) {
	object_data, err := chain.store.LoadObject(BLOCKCHAIN_UNIVERSE, TOP_ID, TOP_ID, TOP_ID)

	if err != nil {
		panic("Backend failure")
	}

	if len(object_data) == 0 {
		panic(fmt.Errorf("most probably Database corruption, No TOP_ID stored "))
	}

	if len(object_data) != 32 {
		panic("Database corruption, invalid block hash ")
	}
	copy(hash[:], object_data[:32])
	return hash
}

*/

// store current  highest topo id
func (chain *Blockchain) StoreTopoHeight(dbtx storage.DBTX, height int64) {
	if dbtx == nil {
		panic("Could NOT change TOP height to chain. Error opening writable TX, err ")
	}
	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TOPO_HEIGHT, TOPO_HEIGHT, uint64(height))
}

// faster bootstrap
func (chain *Blockchain) LoadTopoHeight(dbtx storage.DBTX) (height int64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	heightx, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TOPO_HEIGHT, TOPO_HEIGHT)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load  TOPO height for chain err %s", err)
		return 0
	}
	return int64(heightx)
}

// store current top id  // store top height known
func (chain *Blockchain) StoreTopHeight(dbtx storage.DBTX, height int64) {
	if dbtx == nil {
		panic("Could NOT change TOP height to chain. Error opening writable TX, ")

	}

	dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TOP_HEIGHT, TOP_HEIGHT, uint64(height))
}

// faster bootstrap
func (chain *Blockchain) LoadTopHeight(dbtx storage.DBTX) (height int64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}
		defer dbtx.Rollback()
	}

	heightx, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TOP_HEIGHT, TOP_HEIGHT)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load  TOP height for chain err %s", err)
		return 0
	}
	return int64(heightx)
}

func (chain *Blockchain) LoadPoolClosedHeight(dbtx storage.DBTX, poolId crypto.Hash) (height uint64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT load pool closed height. Error opening writable TX, err %s", err)
			return
		}
		defer dbtx.Rollback()
	}

	height, err = dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, poolId[:], PLANET_POOL_CLOSEDHEIGHT)
	if err != nil {
		return 0
	}

	return height
}

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func (chain *Blockchain) GetBlockOutputs(dbtx storage.DBTX, blid crypto.Hash) (outputs []globals.TXOutputData, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT get block outputs. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	start, end := chain.GetBlockOutputIndex(dbtx, blid)
	var data []byte
	for i := start; i < end; i++ {
		data, err = chain.ReadOutputIndex(dbtx, uint64(i))
		if err != nil {
			return
		}

		var output globals.TXOutputData
		if err = msgpack.Unmarshal(data, &output); err != nil {
			return
		}

		outputs = append(outputs, output)
	}

	return
}

// get the position from where indexing must start for this block
// indexing mean vout based index
// cryptonote works by giving each vout a unique index number
func (chain *Blockchain) GetBlockOutputIndex(dbtx storage.DBTX, blockId crypto.Hash) (int64, int64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0, 0
		}

		defer dbtx.Rollback()
	}
	// first gets the  topo index of this block

	index, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load output index for %s err %s", blockId, err)
		return 0, 0
	}

	indexEnd, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX_END)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load output index for %s err %s", blockId, err)
		return int64(index), 0
	}

	return int64(index), int64(indexEnd)
}

func (chain *Blockchain) GetTokenOutputIndex(dbtx storage.DBTX, blockId crypto.Hash) (int64, int64) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return 0, 0
		}

		defer dbtx.Rollback()
	}
	// first gets the  topo index of this block

	index, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load output index for %s err %s", blockId, err)
		return 0, 0
	}

	indexEnd, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blockId[:], PLANET_OUTPUT_INDEX_END)
	if err != nil {
		// TODO  this panic must be enabled to catch some bugs
		logger.Warnf("Cannot load output index for %s err %s", blockId, err)
		return int64(index), 0
	}

	return int64(index), int64(indexEnd)
}

func (chain *Blockchain) GetBlocksAtHeight(dbtx storage.DBTX, height int64) (blocks []crypto.Hash) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	// deserialize height
	heightBytes, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_HEIGHT, PLANET_HEIGHT, itob(uint64(height)))
	if err != nil {
		return
	}
	blocks = make([]crypto.Hash, len(heightBytes)/32, len(heightBytes)/32)

	for i := 0; i < len(heightBytes)/32; i++ {
		copy(blocks[i][:], heightBytes[i*32:(i*32)+32])
	}

	return
}

// TODO: votes per share > 1 ???
func (chain *Blockchain) isVoteValid(dbtx storage.DBTX, blid crypto.Hash, shareId crypto.Hash) bool {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT check vote validity. Error opening readable TX, err %s", err)
			return false
		}

		defer dbtx.Rollback()
	}

	object_data, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_VOTE_VALIDITY, GALAXY_VOTE_VALIDITY, append(blid[:], shareId[:]...))

	if err != nil {
		return false
	}

	if len(object_data) == 0 {
		return false
	}

	if len(object_data) != 1 {
		panic(fmt.Errorf("probably Database corruption, Wrong data stored in tx validity, expected size 1, actual size %d", len(object_data)))
	}

	if object_data[0] == 1 {
		return true
	}

	// anything other than value 1 is considered wrong
	return false
}

// mark the vote as valid/invalid for apply/revoke vote
func (chain *Blockchain) markVote(dbtx storage.DBTX, blid crypto.Hash, shareId crypto.Hash, valid bool) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := byte(0)
	if valid {
		store_value = byte(1)
	}
	keyValue := append(blid[:], shareId[:]...)
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_VOTE_VALIDITY, GALAXY_VOTE_VALIDITY, keyValue, []byte{store_value})
}

// this will mark a block, tx combination as valid/invalid
func (chain *Blockchain) markTX(dbtx storage.DBTX, blid crypto.Hash, txhash crypto.Hash, valid bool) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := byte(0)
	if valid {
		store_value = byte(1)
	}
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION_VALIDITY, GALAXY_TRANSACTION_VALIDITY, append(blid[:], txhash[:]...), []byte{store_value})
}

func (chain *Blockchain) markTokenTx(dbtx storage.DBTX, blid crypto.Hash, txhash crypto.Hash, valid bool) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := byte(0)
	if valid {
		store_value = byte(1)
	}
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION_VALIDITY, GALAXY_TOKEN_VALIDITY, append(blid[:], txhash[:]...), []byte{store_value})
}

// this will return the tx combination as valid/invalid
func (chain *Blockchain) IsTxValid(dbtx storage.DBTX, blid crypto.Hash, txhash crypto.Hash) bool {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return false
		}

		defer dbtx.Rollback()
	}

	object_data, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION_VALIDITY, GALAXY_TRANSACTION_VALIDITY, append(blid[:], txhash[:]...))

	if err != nil {
		return false
	}

	if len(object_data) == 0 {
		return false
	}

	if len(object_data) != 1 {
		panic(fmt.Errorf("probably Database corruption, Wrong data stored in tx validity, expected size 1, actual size %d", len(object_data)))
	}

	if object_data[0] == 1 {
		return true
	}

	// anything other than value 1 is considered wrong tx
	return false
}

func (chain *Blockchain) IsTokenValid(dbtx storage.DBTX, blid crypto.Hash, txhash crypto.Hash) bool {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return false
		}

		defer dbtx.Rollback()
	}

	object_data, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION_VALIDITY, GALAXY_TOKEN_VALIDITY, append(blid[:], txhash[:]...))

	if err != nil {
		return false
	}

	if len(object_data) == 0 {
		return false
	}

	if len(object_data) != 1 {
		panic(fmt.Errorf("probably Database corruption, Wrong data stored in tx validity, expected size 1, actual size %d", len(object_data)))
	}

	if object_data[0] == 1 {
		return true
	}

	// anything other than value 1 is considered wrong tx
	return false
}

// store key image to its own galaxy
// a keyimage stored with value 1  == it has been consumed
// a keyimage stored with value 0  == it has not been consumed
// a key image not found in store == it has NOT been consumed
// TODO this function should NOT be exported
func (chain *Blockchain) StoreKeyImage(dbtx storage.DBTX, hash crypto.Hash, height int64) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := itob(uint64(height))

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_KEYIMAGE, GALAXY_KEYIMAGE, hash[:], store_value)
}

func (chain *Blockchain) StoreTokenKeyImage(dbtx storage.DBTX, hash crypto.Hash, height int64) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := itob(uint64(height))

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TOKEN_KEYIMAGE, GALAXY_TOKEN_KEYIMAGE, hash[:], store_value)
}

func (chain *Blockchain) StoreTokenId(dbtx storage.DBTX, hash crypto.Hash, height int64) {
	if dbtx == nil {
		panic("dbtx cannot be nil")
	}
	store_value := itob(uint64(height))

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TOKEN_ID, GALAXY_TOKEN_ID, hash[:], store_value)
}

// read a key image, whether it's stored with value 1
// a keyimage stored with value 1  == it has been consumed
// a keyimage stored with value 0  == it has not been consumed
// a key image not found in store == it has NOT been consumed
func (chain *Blockchain) Read_KeyImage_Status(dbtx storage.DBTX, hash crypto.Hash) (int64, bool) {

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return -1, false
		}

		defer dbtx.Rollback()
	}

	marker, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_KEYIMAGE, GALAXY_KEYIMAGE, hash[:])
	if err != nil {
		return -1, false
	}

	height_consumed := int64(marker)

	if height_consumed < 0 {
		return -1, false
	} else {
		return height_consumed, true
	}

}

func (chain *Blockchain) ReadTokenKeyImageStatus(dbtx storage.DBTX, hash crypto.Hash) (int64, bool) {

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return -1, false
		}

		defer dbtx.Rollback()
	}

	marker, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TOKEN_KEYIMAGE, GALAXY_TOKEN_KEYIMAGE, hash[:])
	if err != nil {
		return -1, false
	}

	height_consumed := int64(marker)

	if height_consumed < 0 {
		return -1, false
	} else {
		return height_consumed, true
	}

}

func (chain *Blockchain) ReadTokenIdStatus(dbtx storage.DBTX, hash crypto.Hash) (int64, bool) {

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return -1, false
		}

		defer dbtx.Rollback()
	}

	marker, err := dbtx.LoadUint64(BLOCKCHAIN_UNIVERSE, GALAXY_TOKEN_ID, GALAXY_TOKEN_ID, hash[:])
	if err != nil {
		return -1, false
	}

	height_consumed := int64(marker)

	if height_consumed < 0 {
		return -1, false
	} else {
		return height_consumed, true
	}

}

func (chain *Blockchain) storeStakeTxInBlock(dbtx storage.DBTX, blid, txid crypto.Hash) {
	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_STAKE_TX, txid[:])
}

func (chain *Blockchain) storePoolStakeTxInBlock(dbtx storage.DBTX, blid, txid crypto.Hash) {
	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_POOLSTAKE_TX, txid[:])
}

func (chain *Blockchain) storeShareStakeTxInBlock(dbtx storage.DBTX, blid, txid crypto.Hash) {
	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_SHARESTAKE_TX, txid[:])
}

func (chain *Blockchain) LoadStakeTxInBlock(dbtx storage.DBTX, blid crypto.Hash) (txid crypto.Hash, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	var value []byte
	value, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_STAKE_TX)
	if err != nil {
		return
	}
	if len(value) != 32 {
		err = fmt.Errorf("invalid staketx %x in block %s", value, blid)
		return
	}

	copy(txid[:], value)

	if !chain.IsTxValid(dbtx, blid, txid) {
		err = fmt.Errorf("tx %s of bl %s is invalid", txid, blid)
		return
	}

	return
}

func (chain *Blockchain) LoadPoolStakeTxInBlock(dbtx storage.DBTX, blid crypto.Hash) (txid crypto.Hash, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	var value []byte
	value, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_POOLSTAKE_TX)
	if err != nil {
		return
	}
	if len(value) != 32 {
		err = fmt.Errorf("invalid pool staketx %x in block %s", value, blid)
		return
	}

	copy(txid[:], value)

	if !chain.IsTxValid(dbtx, blid, txid) {
		err = fmt.Errorf("tx %s of bl %s is invalid", txid, blid)
		return
	}

	return
}

func (chain *Blockchain) LoadShareStakeTxInBlock(dbtx storage.DBTX, blid crypto.Hash) (txid crypto.Hash, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	var value []byte
	value, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_SHARESTAKE_TX)
	if err != nil {
		return
	}
	if len(value) != 32 {
		err = fmt.Errorf("invalid share staketx %x in block %s", value, blid)
		return
	}

	copy(txid[:], value)

	if !chain.IsTxValid(dbtx, blid, txid) {
		err = fmt.Errorf("tx %s of bl %s is invalid", txid, blid)
		return
	}

	return
}

func (chain *Blockchain) storeBonusTxInBlock(dbtx storage.DBTX, blid, txid crypto.Hash) {
	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_BONUS_TX, txid[:])
}

func (chain *Blockchain) LoadBonusTxInBlock(dbtx storage.DBTX, blid crypto.Hash) (txid crypto.Hash, err error) {
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	var value []byte
	value, err = dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, blid[:], PLANET_BONUS_TX)
	if err != nil {
		return
	}
	if len(value) != 32 {
		err = fmt.Errorf("invalid bonustx %x in block %s", value, blid)
		return
	}

	copy(txid[:], value)

	if !chain.IsTxValid(dbtx, blid, txid) {
		err = fmt.Errorf("tx %s of bl %s is invalid", txid, blid)
		return
	}

	return
}

func (chain *Blockchain) store_TX_in_Block(dbtx storage.DBTX, blid, txid crypto.Hash) {

	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	existing_blocks := chain.Load_TX_blocks(dbtx, txid)

	tx_map := map[crypto.Hash]bool{}

	tx_map[blid] = true
	for i := range existing_blocks {
		tx_map[existing_blocks[i]] = true
	}

	store_value := make([]byte, 0, len(tx_map)*32)
	for k, _ := range tx_map {
		store_value = append(store_value, k[:]...)
	}

	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_TX_MINED, store_value)

}

func (chain *Blockchain) Load_TX_blocks(dbtx storage.DBTX, txid crypto.Hash) (blocks []crypto.Hash) {

	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening readable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()

	}

	object_data, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_TRANSACTION, txid[:], PLANET_TX_MINED)

	if err != nil {
		return
	}

	if len(object_data)%32 != 0 {
		panic(fmt.Errorf("probably Database corruption, no blocks found for tx %s  actual size %d", txid, len(object_data)))
	}

	if len(object_data) >= 32 {
		blocks = make([]crypto.Hash, len(object_data)/32, len(object_data)/32)
		for i := 0; i < len(object_data)/32; i++ {
			copy(blocks[i][:], object_data[i*32:i*32+32])
		}
	}
	return blocks

}

// store settle status within the block
// a blockid with stored with value 1  == it has been settled
// a blockid stored with value 0  == it has not been settled
// a blockid not found in store == it has NOT been settled
func (chain *Blockchain) storeTIPS(dbtx storage.DBTX, tips []crypto.Hash) {

	if dbtx == nil {
		panic("Could NOT add block to chain. Error opening writable TX,")
	}

	storeValue := make([]byte, 0, len(tips)*32)
	for i := range tips {
		storeValue = append(storeValue, tips[i][:]...)
	}
	dbtx.StoreObject(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TIPS, TIPS, storeValue)
}

// this is exported for rpc server
func (chain *Blockchain) LoadTipsAtomic(dbtx storage.DBTX) (tips []crypto.Hash) {
	return chain.loadTIPS(dbtx)
}

func (chain *Blockchain) loadTIPS(dbtx storage.DBTX) (tips []crypto.Hash) {
	var err error
	if dbtx == nil {
		dbtx, err = chain.store.BeginTX(false)
		if err != nil {
			logger.Warnf("Could NOT add block to chain. Error opening writable TX, err %s", err)
			return
		}

		defer dbtx.Rollback()
	}

	objectData, err := dbtx.LoadObject(BLOCKCHAIN_UNIVERSE, GALAXY_KEYVALUE, TIPS, TIPS)
	if err != nil {
		return
	}

	if len(objectData) == 0 || len(objectData)%32 != 0 {
		panic(fmt.Errorf("probably Database corruption, No tips founds or invalid tips, tips  actual size %d", len(objectData)))
	}

	tips = make([]crypto.Hash, len(objectData)/32, len(objectData)/32)
	for i := 0; i < len(objectData)/32; i++ {
		copy(tips[i][:], objectData[i*32:i*32+32])
	}
	return tips
}
