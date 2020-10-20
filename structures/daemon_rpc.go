// Copyright 2018-2020 Darma Project. All rights reserved.

// this package contains only struct definitions
// in order to avoid the dependency on block chain by any package requiring access to rpc
// and other structures
// having the structures was causing the build times of explorer/wallet to be more than couple of secs
// so separated the logic from the structures

package structures

import (
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/stake"
	"net"
)

//import "github.com/darmaproject/darmasuite/structures"

type (
	GetBlockHeaderByHeightParams struct {
		Height int64 `json:"height"`
	} // no params
	GetBlockHeaderByHeightResult struct {
		Block_Header []BlockHeader_Print `json:"block_headers"`
		Status       string              `json:"status"`
	}
)

type (
	GetBlockHeaderByTopoHeightParams struct {
		TopoHeight uint64 `json:"topoheight"`
	} // no params
	GetBlockHeaderByTopoHeightResult struct {
		BlockHeader BlockHeader_Print `json:"block_header"`
		Status      string            `json:"status"`
	}
)

type (
	GetBlocksHashByTopoHeightParams struct {
		From uint64 `json:"from"`
		To   uint64 `json:"to"`
	} // no params
	GetBlocksHashByTopoHeightResult struct {
		Hash   []crypto.Hash `json:"hash"`
		Status string        `json:"status"`
	}
)

// GetBlockHeaderByHash
type (
	GetBlockHeaderByHashParams struct {
		Hash string `json:"hash"`
	} // no params
	GetBlockHeaderByHashResult struct {
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

type (
	ProveTxSpentParams struct {
		Input_key  string `json:"tx_key"`
		Input_addr string `json:"address"`
		Input_tx   string `json:"txid"`
	} // no params
	ProveTxSpentResult struct {
		Status string `json:"status"`
		Txinfo Txinfo `json:"tx_info"`
	}

	Txinfo struct {
		Proof_address string `json:"address"`         // address agains which which the proving ran
		Proof_index   int64  `json:"index_within_tx"` // proof satisfied for which index
		Proof_amount  uint64 `json:"amount"`          // decoded amount
		Proof_PayID8  string `json:"paymentid"`       // decrypted 8 byte payment id
		Proof_error   string `json:"err,omitempty"`   // error if any while decoding proof
	}
)

// get block count
type (
	GetBlockCountParams struct {
		// NO params
	}
	GetBlockCountResult struct {
		Count  uint64 `json:"count"`
		Status string `json:"status"`
	}
)

type (
	GetTreeGraphParams struct {
		Height uint64 `json:"height,omitempty"`
	} // no params
	GetTreeGraphResult struct {
		Status  string   `json:"status"`
		Results []Result `json:"results"`
	}

	Result struct {
		Height     int64         `json:"height"`
		TopoHeight int64         `json:"topo_height"`
		Hash       crypto.Hash   `json:"hash"`
		Tips       []crypto.Hash `json:"tips"`
	}
)

type (
	GetDotGraphParams struct {
		StartTopoHeight int64 `json:"start_topo_height"`
		StopTopoHeight  int64 `json:"stop_topo_height"`
	}

	GetDotGraphResult struct {
		Dot string `json:"dot"`
	}
)

// getblock
type (
	GetBlockParams struct {
		Hash   string `json:"hash,omitempty"`   // Monero Daemon breaks if both are provided
		Height uint64 `json:"height,omitempty"` // Monero Daemon breaks if both are provided
	} // no params
	GetBlockResult struct {
		Blob         string            `json:"blob"`
		Json         string            `json:"json"`
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
		BonusTxHex   string            `json:"bonus_tx_hex"`
		StakeTxHex   string            `json:"stake_tx_hex"`
	}
)

type (
	PoolShareInfo struct {
		PoolId    string `json:"pool_id"` // crypto.Hash
		ChosenNum uint32 `json:"chosen_num"`
	}
)

// get block template request response
type (
	GetBlockTemplateParams struct {
		Wallet_Address string `json:"wallet_address"`
		Reserve_size   uint64 `json:"reserve_size"`
	}
	GetBlockTemplateResult struct {
		Blocktemplate_blob string `json:"blocktemplate_blob"`
		Blockhashing_blob  string `json:"blockhashing_blob"`
		Expected_reward    uint64 `json:"expected_reward"`
		Difficulty         uint64 `json:"difficulty"`
		Height             uint64 `json:"height"`
		Prev_Hash          string `json:"prev_hash"`
		Reserved_Offset    uint64 `json:"reserved_offset"`
		Epoch              uint64 `json:"epoch"` // used to expire pool jobs
		Status             string `json:"status"`
	}
)

type (
	GetTokenOutputParams struct {
		IndexGlobal uint64 `json:"index_global,omitempty"`
	}
	GetTokenOutputResult struct {
		Data []byte `json:"data,omitempty"`
	}
)
type (
	IsTokenIdSpentParams struct {
		TokenIds []string `json:"token_ids"`
	} // no params
	IsTokenIdSpentResult struct {
		SpentStatuses []int  `json:"spent_status"` // 0 if okay, 1 spent in block chain, 2 spent in pool
		Status        string `json:"status"`
	}
)

type ( // array without name containing block template in hex
	SubmitBlockParams struct {
		X []string
	}
	SubmitBlockResult struct {
		BLID   string `json:"blid"`
		Status string `json:"status"`
	}
)

type (
	GetLastBlockHeaderParams struct{} // no params
	GetLastBlockHeaderResult struct {
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

type (
	GetTxPoolParams struct{} // no params
	GetTxPoolResult struct {
		Tx_list []string `json:"txs,omitempty"`
		Status  string   `json:"status"`
	}
)

type (
	GetPoolVoteStatsParams struct {
		Height int64 `json:"height,omitempty"`
	} // no params
	GetPoolVoteStatsResult struct {
		Status         string           `json:"status"`
		PoolShareInfos []*PoolShareInfo `json:"pool_share_infos"`
	}

	GetPoolBonusStatsParams struct {
		Height int64 `json:"height,omitempty"`
	} // no params
	GetPoolBonusStatsResult struct {
		Status string               `json:"status"`
		Stats  []*PoolShareInfoRank `json:"stats"`
	}

	PoolShareInfoRank struct {
		PoolId          string `json:"pool_id"`
		PoolName        string `json:"pool_name"`
		ChosenVoteNum   uint32 `json:"chosen_vote_num"`
		PreChoseVoteNum uint32 `json:"pre_chose_vote_num"`
		Newcomer        bool   `json:"newcomer"`
		PoolType        int    `json:"pool_type"`
		VoteNum         uint32 `json:"vote_num"`
		PreVoteNum      uint32 `json:"pre_vote_num"`
		Reward          uint64 `json:"reward"`
		CurrentVoteNum  uint32 `json:"current_vote_num"`
	}
)

type (
	GetP2pSessionParams struct{} // no params
	GetP2pSessionResult struct {
		Connections []P2pConnection `json:"connections,omitempty"`
		Status      string          `json:"status"`
	}

	P2pConnection struct {
		Height           int64  `json:"height"`             // last height sent by peer  ( first member alignments issues)
		StableHeight     int64  `json:"stable_height"`      // last stable height
		StableTopoHeight int64  `json:"stable_topo_height"` // last stable topo height
		StableHash       string `json:"stable_hash"`        // last stable height
		TopoHeight       int64  `json:"topo_height"`        // topo height, current topo height, this is the only thing we require for syncing
		StableStatus     int32  `json:"stable_status"`

		LastObjectRequestTime int64  `json:"last_object_request_time"` // when was the last item placed in object list
		BytesIn               uint64 `json:"bytes_in"`                 // total bytes in
		BytesOut              uint64 `json:"bytes_out"`                // total bytes out
		Latency               int64  `json:"latency"`                  // time.Duration            // latency to this node when sending timed sync

		Incoming    bool         `json:"incoming"`    // is connection incoming or outgoing
		Addr        *net.TCPAddr `json:"addr"`        // endpoint on the other end
		Port        uint32       `json:"port"`        // port advertised by other end as its server,if it's 0 server cannot accept connections
		Peer_ID     uint64       `json:"peer_id"`     // Remote peer id
		Lowcpuram   bool         `json:"low_cpu_ram"` // whether the peer has low cpu ram
		SyncNode    bool         `json:"sync_node"`   // whether the peer has been added to command line as sync node
		Top_Version uint64       `json:"top_version"` // current hard fork version supported by peer
		////TXpool_cache      map[uint64]uint32 // used for ultra blocks in miner mode,cache where we keep TX which have been broadcasted to this peer
		////TXpool_cache_lock sync.RWMutex
		ProtocolVersion string `json:"protocol_version"`
		Tag             string `json:"tag"` // tag for the other end
		DaemonVersion   string `json:"daemon_version"`
		//Exit                  chan bool   // Exit marker that connection needs to be killed
	}
)

// GetOutputs
type (
	GetOutputsParams struct {
		Height uint64 `json:"height"`
		Hash   string `json:"hash,omitempty"` // Monero Daemon breaks if both are provided
	}

	GetOutputsResult struct {
		Outputs []globals.TXOutputData `json:"outputs"`
	}
)

// get height http response as json
type (
	Daemon_GetHeight_Result struct {
		Height       uint64 `json:"height"`
		StableHeight int64  `json:"stableheight"`
		TopoHeight   int64  `json:"topoheight"`

		Status string `json:"status"`
	}
)

type (
	OnGetBlockHashParams struct {
		X [1]uint64 `json:"x"`
	}
	OnGetBlockHashResult struct {
		Hash string `json:"hash"`
	}
)

type (
	GetTransactionParams struct {
		TxHashes []string `json:"txs_hashes"`
		Decode   uint64   `json:"decode_as_json,omitempty"` // Monero Daemon breaks if this sent
	} // no params
	GetTransactionResult struct {
		TxsAsHexes []string        `json:"txs_as_hex"`
		TxsAsJsons []string        `json:"txs_as_json"`
		Txs        []TxRelatedInfo `json:"txs"`
		Status     string          `json:"status"`
	}

	TxRelatedInfo struct {
		AsHex         string                   `json:"as_hex"`
		AsJson        string                   `json:"as_json"`
		BlockHeight   int64                    `json:"block_height"`
		Reward        []uint64                 `json:"reward"`  // miner tx rewards are decided by the protocol during execution
		Ignored       bool                     `json:"ignored"` // tell whether this tx is okau as per client protocol or bein ignored
		InPool        bool                     `json:"in_pool"`
		OutputIndices []uint64                 `json:"output_indices"`
		TxHash        string                   `json:"tx_hash"`
		ValidBlock    string                   `json:"valid_block"`   // TX is valid in this block
		InvalidBlock  []string                 `json:"invalid_block"` // TX is invalid in this block,  0 or more
		Ring          [][]globals.TXOutputData `json:"ring"`
	}
)

type (
	IsKeyImageSpentParams struct {
		KeyImages []string `json:"key_images"`
	} // no params
	IsKeyImageSpentResult struct {
		SpentStatuses []int  `json:"spent_status"` // 0 if okay, 1 spent in block chain, 2 spent in pool
		Status        string `json:"status"`
	}
)

type (
	SendRawTransactionParams struct {
		TxAsHex string `json:"tx_as_hex"`
	}
	SendRawTransactionResult struct {
		Status        string `json:"status"`
		DoubleSpend   bool   `json:"double_spend"`
		FeeTooLow     bool   `json:"fee_too_low"`
		InvalidInput  bool   `json:"invalid_input"`
		InvalidOutput bool   `json:"invalid_output"`
		LowMixin      bool   `json:"low_mixin"`
		NonRct        bool   `json:"not_rct"`
		NotRelayed    bool   `json:"not_relayed"`
		Overspend     bool   `json:"overspend"`
		TooBig        bool   `json:"too_big"`
		Reason        string `json:"string"`
	}
)

/*
{
  	"id": "0",
  	"jsonrpc": "2.0",
  	"result": {
    	"alt_blocks_count": 5,
    	"difficulty": 972165250,
    	"grey_peerlist_size": 2280,
    	"height": 993145,
    	"incoming_connections_count": 0,
    	"outgoing_connections_count": 8,
    	"status": "OK",
    	"target": 60,
    	"target_height": 993137,
    	"testNet": false,
    	"top_block_hash": "",
    	"tx_count": 564287,
    	"tx_pool_size": 45,
    	"white_peerlist_size": 529
  	}
}
*/

type (
	DebugStableParams struct {
		StartHeight int64 `json:"start_height"`
		EndHeight   int64 `json:"end_height"`
	}
	DebugStableResult struct {
		Status string `json:"status"`

		StartHeight      int64       `json:"start_height,omitempty"`
		EndHeight        int64       `json:"end_height,omitempty"`
		StableHeight     int64       `json:"stable_height,omitempty"`
		StableTopoHeight int64       `json:"stable_topo_height,omitempty"`
		StableHash       crypto.Hash `json:"stable_hash,omitempty"`
		ChainHeight      int64       `json:"chain_height,omitempty"`
		Distance         int64       `json:"distance,omitempty"`

		Local  []BlocksBetween `json:"data"`
		Remote []BlocksBetween `json:"remote,omitempty"` // now unSupport
	}

	BlocksBetween struct {
		Height     int64       `json:"height"`
		BlockItems []BlockItem `json:"blocks"`
	}

	BlockItem struct {
		Id         crypto.Hash   `json:"blid"`
		TopoHeight int64         `json:"topo_height"`
		Tips       []crypto.Hash `json:"tips"`
	}
)

type (
	GetInfoParams struct{} // no params
	GetInfoResult struct {
		AltBlocksCount           uint64  `json:"alt_blocks_count"`
		Difficulty               uint64  `json:"difficulty"`
		GreyPeerListSize         uint64  `json:"grey_peerlist_size"`
		Height                   int64   `json:"height"`
		StableHeight             int64   `json:"stableheight"`
		TopoHeight               int64   `json:"topoheight"`
		AverageBlockTime50       float32 `json:"averageblocktime50"`
		IncomingConnectionsCount uint64  `json:"incoming_connections_count"`
		OutgoingConnectionsCount uint64  `json:"outgoing_connections_count"`
		Target                   uint64  `json:"target"`
		TargetHeight             uint64  `json:"target_height"`
		TestNet                  bool    `json:"testNet"`
		TopBlockHash             string  `json:"top_block_hash"`
		TxCount                  uint64  `json:"tx_count"`
		TxPoolSize               uint64  `json:"tx_pool_size"`
		DynamicFeePerKb          uint64  `json:"dynamic_fee_per_kb"` // our addition
		TotalSupply              uint64  `json:"total_supply"`       // our addition
		CirculationSupply        uint64  `json:"circulation_supply"`
		MedianBlockSize          uint64  `json:"median_block_size"` // our addition
		WhitePeerlistSize        uint64  `json:"white_peerlist_size"`
		Version                  string  `json:"version"`
		ShareTotalNum            uint32  `json:"share_total_num"`
		CanVoteNum               uint32  `json:"can_vote_num"`
		CannotVoteNum            uint32  `json:"cannot_vote_num"`
		NetStatus                `json:"net_status"`
		NodeTag                  string `json:"node_tag,omitempty"`

		Status string `json:"status"`
	}

	NetStatus struct {
		Status string `json:"status"`
		Good   int    `json:"good"`
		Poor   int    `json:"poor"`
		Fair   int    `json:"fair"`
	}
)

// listStakePool
type (
	ListStakePoolParams struct {
		From  int `json:"from"`
		Limit int `json:"limit"`
	}
	ListStakePoolResult struct {
		Pools []*GetStakePoolResult `json:"pools"`
		Count int                   `json:"count"`
	}
)

type (
	ListShareParams struct {
		IncludeVoteRecords bool         `json:"include_vote_records"`
		PoolId             string       `json:"pool_id"`
		PoolName           string       `json:"pool_name"`
		Status             stake.Status `json:"status"`
		From               int          `json:"from"`
		Limit              int          `json:"limit"`
	}
	ListShareResult struct {
		Shares []GetShareResult `json:"shares"`
		Count  int              `json:"count"`
	}
)

type (
	GetDaemonStakeInfoParams struct {
	}
	GetDaemonStakeInfoResult struct {
		TotalPoolNum       int    `json:"total_pool_num"`
		TotalPoolProfit    uint64 `json:"total_pool_profit"`
		CurrentBonusProfit uint64 `json:"current_bonus_profit"`
		TotalShareNum      uint64 `json:"total_share_num"`
		TotalShareProfit   uint64 `json:"total_share_profit"`
		TotalVoteNum       uint32 `json:"total_vote_num"`
		ImmatureVoteNum    uint32 `json:"immature_vote_num"`
		MatureVoteNum      uint32 `json:"mature_vote_num"`
		LastVotePoolId     string `json:"last_vote_pool_id"`
		LastVotePoolName   string `json:"last_vote_pool_name"`
		LastVoteShareId    string `json:"last_vote_share_id"`
	}
)

// getStakePool
type (
	GetStakePoolParams struct {
		PoolId string `json:"pool_id"`
	}
	GetStakePoolResult struct {
		Name            string `json:"name"`
		PoolId          string `json:"pool_id"`
		Vote            string `json:"vote"`
		Reward          string `json:"reward"`
		TxHash          string `json:"tx_id"`
		Height          int64  `json:"height"`
		TopoHeight      int64  `json:"topo_height"`
		Amount          uint64 `json:"amount"`
		TotalNum        uint32 `json:"total_num"`
		ImmatureVoteNum uint32 `json:"immature_vote_num"`
		MatureVoteNum   uint32 `json:"mature_vote_num"`
		WishVoteNum     uint32 `json:"wish_vote_num"`
		ChosenNum       uint32 `json:"chosen_vote_num"`
		Profit          uint64 `json:"profit"`
		Fee             int32  `json:"fee"`
		LastPayTime     int64  `json:"last_pay_time"`
		LastVoteTime    int64  `json:"last_vote_time"`
		Closed          bool   `json:"closed"`
		ClosedHeight    int64  `json:"closed_height"`
		ProfitRate      string `json:"profit_rate"`
	}
)

// GetBlockHeaderByHash
type (
	GetContractAddressByTxHashParams struct {
		Hash string `json:"hash"`
	} // no params
	GetContractAddressByTxHashResult struct {
		Address string `json:"address"`
	}
)

type (
	CallContractParams struct {
		WalletCallContractParams
		From string `json:"from"`
	}
	CallContractResult struct {
		Data string `json:"data"`
	}
)

type (
	GetContractResultParams struct {
		TXHash string `json:"tx_hash"`
	}
	GetContractResultResult struct {
		Data string `json:"data"`
	}
)

type (
	GetDifficultyParams struct {
		Height    int64 `json:"height"`
		Interval  int64 `json:"interval"`
		StartTime int64 `json:"start_time"`
	}
	GetDifficultyResult []Difficulty
)

type Difficulty struct {
	Difficulty uint64 `json:"difficulty"`
	HashRate   string `json:"hash_rate"`
	Timestamp  int64  `json:"timestamp"`
}

type (
	GetBlockListParams struct {
		StartTopoHeight uint64 `json:"starttopoheight"`
		SizeInBlocks    uint64 `json:"sizeinblock"`
	}
	GetBlockListResult []BlockInfo
)

type BlockInfo struct {
	MajorVersion uint64
	MinorVersion uint64
	Height       int64
	TopoHeight   int64
	Depth        int64
	Timestamp    uint64
	Hash         string
	Tips         []string
	Votes        []string
	Pools        []string
	Nonce        uint64
	Fees         string
	Reward       string
	Size         string
	Age          string //  time diff from current time
	BlockTime    string // UTC time from block header
	AverageAge   uint64 // UTC time from prev block
	Epoch        uint64 // Epoch time
	Outputs      string
	Mtx          TxInfo
	Stx          *TxInfo
	Sstx         *TxInfo
	Pstx         *TxInfo
	Btx          *TxInfo
	Txs          []TxInfo
	OrphanStatus bool
	SyncBlock    bool // whether the block is sync block
	TxCount      int
}

type TxInfo struct {
	Hex          string // raw tx
	Height       string // height at which tx was mined
	Depth        int64
	Timestamp    uint64 // timestamp
	Age          string //  time diff from current time
	BlockTime    string // UTC time from block header
	Epoch        uint64 // Epoch time
	In_Pool      bool   // whether tx was in pool
	Hash         string // hash for hash
	PrefixHash   string // prefix hash
	Version      int    // version of tx
	Size         string // size of tx in KB
	Sizeuint64   uint64 // size of tx in bytes
	Fee          string // fee in TX
	Feeuint64    uint64 // fee in atomic units
	In           int    // inputs counts
	Out          int    // outputs counts
	Amount       []string
	CoinBase     bool     // is tx coin base
	Extra        string   // extra within tx
	Keyimages    []string // key images within tx
	OutAddress   []string // contains output secret key
	OutOffset    []uint64 // contains index offsets
	Type         string   // ringct or ruffct ( bulletproof)
	ValidBlock   string   // the tx is valid in which block
	InvalidBlock []string // the tx is invalid in which block
	Skipped      bool     // this is only valid, when a block is being listed
	Ring_size    int
	Ring         [][]globals.TXOutputData

	TXpublickey string
	PayID32     string // 32 byte payment ID
	PayID8      string // 8 byte encrypted payment ID

	Proof_address string // address agains which which the proving ran
	Proof_index   int64  // proof satisfied for which index
	Proof_amount  string // decoded amount
	Proof_PayID8  string // decrypted 8 byte payment id
	Proof_error   string // error if any while decoding proof

}

type (
	GetBalanceOfContractAccountParams struct {
		Address string `json:"address"`
	}
	GetBalanceOfContractAccountResult struct {
		Balance string `json:"balance"`
	}
)
