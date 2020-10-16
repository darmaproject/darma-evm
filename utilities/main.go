// Copyright 2018-2020 Darma Project. All rights reserved.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/darmaproject/darmasuite/stake"
	"io"
	"math"
	"math/big"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"runtime/pprof"

	"github.com/chzyer/readline"
	"github.com/docopt/docopt-go"
	"github.com/romana/rlog"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"

	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/block"
	"github.com/darmaproject/darmasuite/blockchain"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/p2p"
	"github.com/darmaproject/darmasuite/transaction"

	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/cryptonight"

	"github.com/darmaproject/darmasuite/rpcserver"
	"github.com/darmaproject/darmasuite/walletapi"
)

const commandLine = `darmad
Darma: A secure, private blockchain with smart-contracts 

Usage:
  darmad [--help] [--version] [--testNet] [--sync-node] [--boltdb | --badgerdb] [--disable-checkpoints] [--netEnv=<netEnv>] [--socks-proxy=<socks_ip:port>] [--data-dir=<directory>] [--p2p-bind=<0.0.0.0:53803>] [--add-exclusive-node=<ip:port>]... [--add-priority-node=<ip:port>]... 	[--min-peers=<11>] [--rpc-bind=<127.0.0.1:53804>] [--lowcpuram] [--mining-address=<wallet_address>] [--mining-threads=<cpu_num>] [--node-tag=<unique name>] [--vote-rpc-address=<127.0.0.1:53805>] [--pool-id=<xxxx>] [--log-level=<info>]
  darmad -h | --help
  darmad -v | --version

Options:
  --help                               Show usage.
  --version                            Show version.
  --disable-checkpoints                Disable checkpoints, work in truly async, slow mode 1 block at a time
  --testNet  	                       Run in testNet mode.
  --boltdb                             Use boltdb as backend (default on 64 bit systems)
  --badgerdb                           Use Badgerdb as backend (default on 32 bit systems)
  --socks-proxy=<socks_ip:port>        Use a proxy to connect to network.
  --data-dir=<directory>               Store blockchain data at this location
  --rpc-bind=<127.0.0.1:53804>         RPC listens on this ip:port
  --p2p-bind=<0.0.0.0:53803>           P2P server listens on this ip:port, specify port 0 to disable listening server
  --add-exclusive-node=<ip:port>       Connect to specific peer only 
  --add-priority-node=<ip:port>	       Maintain persistant connection to specified peer
  --sync-node                          Sync node automatically with the seeds nodes. This option is for rare use.
  --min-peers=<11>                     Number of connections the daemon tries to maintain  
  --lowcpuram                          Disables some RAM consuming sections (deactivates mining/ultra compact protocol etc).
  --mining-address=<wallet_address>    This address is rewarded when a block is mined sucessfully
  --mining-threads=<cpu_num>           Number of CPU threads for mining
  --node-tag=<unique name>             Unique name of node, visible to everyone
  --vote-rpc-address=<127.0.0.1:53805> RPC address of vote wallet
  --pool-id=<xxxx>                     Stake pool id
  --log-level=<info>                   Log level(trace, debug, info, warn, error), defaults to info
`

var ExitInProgress = make(chan bool)
var needUpdatePrompt bool

func tokenList(chain *blockchain.Blockchain, filter string, l *readline.Instance) {
	datas, err := chain.LoadAllOmniToken(nil)
	if err != nil {
		globals.Logger.Errorf("Err %s", err)
		return
	}

	sort.Slice(datas, func(i, j int) bool {
		return datas[i].TopoHeight < datas[j].TopoHeight
	})

	for _, data := range datas {
		fmt.Printf("%-32s: %s\n", "Symbol", data.Symbol)
		fmt.Printf("%-32s: %s\n", "ID", data.Id)
		fmt.Printf("%-32s: %s\n", "Total Amount", globals.FormatMoney(data.Amount))
		fmt.Printf("%-32s: %s\n", "Destroy DMCH", globals.FormatMoney(data.Destroy))
		fmt.Printf("%-32s: %d\n", "Timestamp", data.Timestamp)
		fmt.Printf("%-32s: %d\n", "Issue Height", data.Height)
		fmt.Printf("%-32s: %d\n", "Issue Topo Height", data.TopoHeight)
		fmt.Printf("%-32s: %s\n", "Name", data.Name)
		fmt.Printf("%-32s: %s\n", "Tx ID", data.TxId)
		fmt.Println()
	}
}

func showStakeShares(chain *blockchain.Blockchain, filter string, l *readline.Instance) {
	shares, err := chain.LoadAllShares(nil)
	if err != nil {
		globals.Logger.Errorf("Err %s", err)
		return
	}

	sort.Slice(shares, func(i, j int) bool {
		return shares[i].TopoHeight < shares[j].TopoHeight
	})

	height := chain.GetHeight()
	for _, share := range shares {
		shareId := share.Id()

		if filter != "" && shareId.String() != filter {
			continue
		}

		status, err := chain.GetShareStatusV2(nil, share, height)
		if err != nil {
			globals.Logger.Warnf("Failed get share status, id %s, err %s", shareId, err)
			continue
		}

		record, err := chain.LoadVoteRecord(nil, shareId, share.Index)
		if err != nil {
			globals.Logger.Warnf("Failed get share record, id %s, err %s", shareId, err)
			continue
		}

		fmt.Printf("%-32s: %s\n", "Pool", share.PoolId)
		fmt.Printf("%-32s: %s\n", "ID", shareId)
		fmt.Printf("%-32s: %s\n", "Reward Address", share.Reward)
		fmt.Printf("%-32s: %s\n", "Tx Hash", share.TxHash)
		fmt.Printf("%-32s: %d\n", "Buy Height", share.Height)
		fmt.Printf("%-32s: %d\n", "Buy Topo Height", share.TopoHeight)
		fmt.Printf("%-32s: %s\n", "Locked Amount", globals.FormatMoney(share.Amount))
		fmt.Printf("%-32s: %d\n", "Total  Num", share.InitNum)
		fmt.Printf("%-32s: %d\n", "Remaining Num", share.WillVoteNum)
		fmt.Printf("%-32s: %s\n", "Profit", globals.FormatMoney(share.Profit))
		fmt.Printf("%-32s: %d\n", "Status", status)
		fmt.Printf("%-32s: %d\n", "Mature Height", chain.LoadHeightForBlId(nil, record.BeginBlock)+record.MaturityTime)
		if share.LastPayTime > 0 {
			fmt.Printf("%-32s: %s\n", "Last Pay Time", time.Unix(share.LastPayTime, 0))
		} else {
			fmt.Printf("%-32s:\n", "Last Pay Time")
		}
		if share.LastVoteTime > 0 {
			fmt.Printf("%-32s: %s\n", "Lasy Vote Time", time.Unix(share.LastVoteTime, 0))
		} else {
			fmt.Printf("%-32s:\n", "Lasy Vote Time")
		}
		fmt.Printf("%-32s: %t\n", "Is Closed", share.Closed)
		fmt.Println()
	}
}

func showStakePools(chain *blockchain.Blockchain, filter string, l *readline.Instance) {
	pools, err := chain.LoadAllStakePool(nil)
	if err != nil {
		globals.Logger.Errorf("Err %s", err)
		return
	}

	height := chain.GetHeight()

	for _, pool := range pools {
		if filter != "" && filter != pool.Id.String() && filter != pool.Name {
			continue
		}

		if filter == "" && pool.Closed {
			closedHeight := chain.LoadPoolClosedHeight(nil, pool.Id)

			if height > int64(closedHeight+config.CLOSE_POOL_TX_AMOUNT_UNLOCK) {
				continue
			}
		}

		shares, _ := chain.LoadSharesByPoolId(nil, pool.Id)
		var validNum, totalNum uint32
		for _, share := range shares {
			if share.Closed {
				continue
			}
			status, _ := chain.GetShareStatusV2(nil, share, height)
			if status == stake.STATUS_VALID {
				validNum += share.WillVoteNum
			}
			totalNum += share.InitNum
		}
		fmt.Printf("  %-32s: %s\n", "Name", pool.Name)
		fmt.Printf("  %-32s: %s\n", "ID", pool.Id)
		//fmt.Printf("  %-32s: %s\n", "Register Address", pool.From)
		fmt.Printf("  %-32s: %s\n", "Vote Address", pool.Vote)
		fmt.Printf("  %-32s: %s\n", "Reward Address", pool.Reward)
		fmt.Printf("  %-32s: %s\n", "Tx Hash", pool.TxHash)
		fmt.Printf("  %-32s: %d\n", "Register Height", pool.Height)
		fmt.Printf("  %-32s: %d\n", "Register Topo Height", pool.TopoHeight)
		fmt.Printf("  %-32s: %s\n", "Locked Amount", globals.FormatMoney(pool.Amount))
		fmt.Printf("  %-32s: %d\n", "Immature Vote Num", totalNum-validNum)
		fmt.Printf("  %-32s: %d\n", "Mature Vote Num", validNum)
		fmt.Printf("  %-32s: %d\n", "Total Vote Num", totalNum)
		fmt.Printf("  %-32s: %d\n", "Chosen Vote Num", pool.ChosenNum)
		fmt.Printf("  %-32s: %s\n", "Profit", globals.FormatMoney(pool.Profit))
		if pool.LastPayTime > 0 {
			fmt.Printf("  %-32s: %s\n", "Last Pay Time", time.Unix(pool.LastPayTime, 0))
		} else {
			fmt.Printf("  %-32s:\n", "Last Pay Time")
		}
		if pool.LastVoteTime > 0 {
			fmt.Printf("  %-32s: %s\n", "Lasy Vote Time", time.Unix(pool.LastVoteTime, 0))
		} else {
			fmt.Printf("  %-32s:\n", "Lasy Vote Time")
		}
		fmt.Printf("  %-32s: %t\n", "Is Closed", pool.Closed)
		fmt.Println()
	}
}

func main() {
	var err error
	globals.Arguments, err = docopt.Parse(commandLine, nil, true, config.Version.String(), false)

	if err != nil {
		log.Fatalf("Error while parsing options err: %s\n", err)
	}
	// We need to initialize readline first, so it changes stderr to ansi processor on windows

	l, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[92mDarma:\033[32m>>>\033[0m ",
		HistoryFile:     filepath.Join(os.TempDir(), "darmad_readline.tmp"),
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})

	if err != nil {
		panic(err)
	}
	defer l.Close()

	// parse arguments and setup testNet mainNet
	globals.Initialize() // setup network and proxy
	fmt.Print(strings.TrimPrefix(config.LOGO, "\n"))
	globals.Logger.Infof("") // a dummy write is required to fully activate logrus

	// all screen output must go through the readline
	globals.Logger.Out = l.Stdout()

	rlog.Infof("Arguments %+v", globals.Arguments)
	globals.Logger.Infof("Network: %s", globals.Config.Name)
	globals.Logger.Infof("Version: %s", config.Version.String())
	globals.Logger.Infof("Copyright 2018-2020 Darma Project. All rights reserved.")
	globals.Logger.Infof("OS:%s ARCH:%s GOMAXPROCS:%d", runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0))
	globals.Logger.Infof("Daemon data directory %s", globals.GetDataDirectory())

	go checkUpdateLoop()

	params := map[string]interface{}{}

	//params["--disable-checkpoints"] = globals.Arguments["--disable-checkpoints"].(bool)
	chain, err := blockchain.BlockchainStart(params)
	if err != nil {
		globals.Logger.Warnf("Error starting blockchain err '%s'", err)
		return
	}

	params["chain"] = chain

	if cryptonight.HardwareAES {
		rlog.Infof("Hardware AES detected")
	}

	p2p.P2pInit(params)

	rpc, _ := rpcserver.RpcServerStart(params)

	// setup function pointers
	// these pointers need to fixed
	chain.Mempool.P2pTxRelayer = func(tx *transaction.Transaction, peerid uint64) (count int) {
		count += p2p.BroadcastTx(tx, peerid)
		return
	}

	chain.P2pBlockRelayer = func(cbl *block.CompleteBlock, peerid uint64) {
		p2p.BroadcastBlock(cbl, peerid)
	}
	chain.P2pVoteRelayer = func(v *stake.Vote, peerid uint64) {
		p2p.BroadcastVote(v, peerid)
	}
	chain.P2pLotteryRelayer = func(lottery *stake.Lottery, peerid uint64) {
		p2p.BroadcastLottery(lottery, peerid)
	}

	chain.P2pNetStatus = func() (Count uint64, Good_count, Poor_count, Fair_count, Status int) {
		peerCount, peerStatus := p2p.PeerStatus()

		Count, Good_count, Fair_count, Poor_count = peerCount, peerStatus.Good_count, peerStatus.Fair_count, peerStatus.Poor_count
		Status = int(peerStatus.Status)

		return
	}
	chain.P2pNodeTag = func() string {
		return p2p.NodeTag()
	}
	chain.BestPeerHeight = func() (best_height, best_topo_height int64) {
		return p2p.BestPeerHeight()
	}

	if globals.Arguments["--lowcpuram"].(bool) == false && globals.Arguments["--sync-node"].(bool) == false { // enable v1 of protocol only if requested

		// if an address has been provided, verify that it satisfies //mainNet/testNet criteria
		if globals.Arguments["--mining-address"] != nil {

			addr, err := globals.ParseValidateAddress(globals.Arguments["--mining-address"].(string))
			if err != nil {
				globals.Logger.Fatalf("Mining address is invalid: err %s", err)
			}
			params["mining-address"] = addr

			//log.Debugf("Setting up proxy using %s", Arguments["--socks-proxy"].(string))
		}

		if globals.Arguments["--mining-threads"] != nil {
			threadCount := 0
			if s, err := strconv.Atoi(globals.Arguments["--mining-threads"].(string)); err == nil {
				//fmt.Printf("%T, %v", s, s)
				threadCount = s
			} else {
				globals.Logger.Fatalf("Mining threads argument cannot be parsed: err %s", err)
			}

			if threadCount > runtime.GOMAXPROCS(0) {
				globals.Logger.Fatalf("Mining threads (%d) is more than available CPUs (%d). This is NOT optimal", threadCount, runtime.GOMAXPROCS(0))
			}

			params["mining-threads"] = threadCount

			if _, ok := params["mining-address"]; !ok {
				globals.Logger.Fatalf("Mining threads require a valid wallet address")
			}

			globals.Logger.Infof("System will mine to %s with %d threads. Good Luck!!", globals.Arguments["--mining-address"].(string), threadCount)

			go startMiner(chain, params["mining-address"].(*address.Address), threadCount)
		}
	}

	go timeCheckRoutine() // check whether server time is in sync
	//go healthCheck(chain)

	// This tiny goroutine continuously updates status as required
	go func() {
		lastOurHeight := int64(0)
		lastBestHeight := int64(0)
		lastPeerCount := uint64(0)
		lastTopoHeight := int64(0)
		lastMempoolTxCount := 0
		lastCounter := uint64(0)
		lastCounterTime := time.Now()
		lastMiningState := false

		for {
			select {
			case <-ExitInProgress:
				return
			default:
			}

			ourHeight := chain.GetHeight()
			bestHeight, bestTopoHeight := p2p.BestPeerHeight()
			peerCount, _ := p2p.PeerStatus()
			topoHeight := chain.LoadTopoHeight(nil)

			mempoolTxCount := len(chain.Mempool.MempoolListTx())

			// only update prompt if needed
			if lastMiningState != mining || mining || lastOurHeight != ourHeight || lastBestHeight != bestHeight || lastPeerCount != peerCount || lastTopoHeight != topoHeight || lastMempoolTxCount != mempoolTxCount || needUpdatePrompt {
				if needUpdatePrompt {
					needUpdatePrompt = false
				}
				// choose color based on urgency
				color := "\033[92m" // default is light green color
				if ourHeight < bestHeight {
					color = "\033[32m" // make prompt green
				} else if ourHeight > bestHeight {
					color = "\033[31m" // make prompt red
				}

				miningString := ""

				if mining {
					miningSpeed := float64(counter-lastCounter) / (float64(uint64(time.Since(lastCounterTime))) / 1000000000.0)
					lastCounter = counter
					lastCounterTime = time.Now()
					switch {
					case miningSpeed > 1000000:
						miningString = fmt.Sprintf("MINING %.1f MH/s ", float32(miningSpeed)/1000000.0)
					case miningSpeed > 1000:
						miningString = fmt.Sprintf("MINING %.1f KH/s ", float32(miningSpeed)/1000.0)
					case miningSpeed > 0:
						miningString = fmt.Sprintf("MINING %.0f H/s ", miningSpeed)
					}
				}
				lastMiningState = mining

				stakeString := ""
				enabled, online, validNum, totalNum := chain.LoadStakeInfo()
				if chain.GetCurrentVersionAtHeight(ourHeight) >= config.DPOS_FORK_VERSION && enabled {
					if online {
						stakeString = fmt.Sprintf("PPoS online %d/%d/%d ", totalNum-validNum, validNum, totalNum)
					} else {
						stakeString = fmt.Sprintf("PPoS offline ")
					}
				}

				testnetString := ""
				if !globals.IsMainnet() {
					testnetString = "\033[31m[TESTNET]"
				}

				//l.SetPrompt(fmt.Sprintf("%s\033[1m\033[34mHeight \033[0m%s%d/%d \033[1m\033[34mTopo Height \033[0m%s%d/%d %sP %d TXp %d \033[34mNW %s %s>>>\033[0m ", testnetString, color, ourHeight, bestHeight, color, topoHeight, bestTopoHeight, pcolor, peerCount, mempoolTxCount, hashRateString, miningString))
				percent := 0.0
				left := int64(0)
				if bestHeight > ourHeight {
					left = bestHeight - ourHeight
					percent = 100 * float64(ourHeight) / float64(bestHeight)
				}
				l.SetPrompt(fmt.Sprintf("%s\033[1m\033[34mDarmaV2 Synced \033[0m%s%d/%d | \033[0m%s[%d/%d] \033[94m(%0.2f%%, %d left) %s %s>>>\033[0m ", testnetString, color, ourHeight, topoHeight, color, bestHeight, bestTopoHeight, percent, left, miningString, stakeString))
				l.Refresh()
				lastOurHeight = ourHeight
				lastBestHeight = bestHeight
				lastPeerCount = peerCount
				lastMempoolTxCount = mempoolTxCount
				lastTopoHeight = bestTopoHeight
			}
			time.Sleep(1 * time.Second)
		}
	}()

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})
	l.Refresh() // refresh the prompt

	go func() {
		var gracefulStop = make(chan os.Signal)
		signal.Notify(gracefulStop, os.Interrupt) // listen to all signals
		for {
			sig := <-gracefulStop
			fmt.Printf("received signal %s\n", sig)

			if sig.String() == "interrupt" {
				close(ExitInProgress)
			}
		}
	}()

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				fmt.Print("Ctrl-C received, Exit in progress\n")
				close(ExitInProgress)
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			<-ExitInProgress
			break
		}

		line = strings.TrimSpace(line)
		lineParts := strings.Fields(line)

		command := ""
		if len(lineParts) >= 1 {
			command = strings.ToLower(lineParts[0])
		}

		switch {
		case line == "help":
			usage(l.Stderr())

		case strings.HasPrefix(line, "say"):
			line := strings.TrimSpace(line[3:])
			if len(line) == 0 {
				log.Println("say what?")
				break
			}
		//
		case command == "import_chain": // this migrates existing chain from Darma to Darma atlantis
			f, err := os.Open("/tmp/raw_export.txt")
			if err != nil {
				globals.Logger.Warnf("error opening  file  /tmp/raw_export.txt %s", err)
				continue
			}
			reader := bufio.NewReader(f)

			account, _ := walletapi.Generate_Keys_From_Random() // create a random address

			for {
				line, err = reader.ReadString('\n')

				if err != nil || len(line) < 10 {
					break
				}

				var txs []string

				err = json.Unmarshal([]byte(line), &txs)
				if err != nil {
					fmt.Printf("err while unmarshalling json err %s", err)
					continue
				}

				if len(txs) < 1 {
					panic("TX cannot be zero")
				}

				cbl, bl, _ := chain.CreateNewMinerBlock(account.GetAddress())

				for i := range txs {
					var tx transaction.Transaction

					txBytes, err := hex.DecodeString(txs[i])
					if err != nil {
						globals.Logger.Warnf("TX could not be decoded")
					}

					err = tx.DeserializeHeader(txBytes)
					if err != nil {
						globals.Logger.Warnf("TX could not be Deserialized")
					}

					globals.Logger.Infof(" txhash  %s", tx.GetHash())
					if i == 0 {
						bl.MinerTx = tx
						cbl.Bl.MinerTx = tx

						if bl.MinerTx.GetHash() != tx.GetHash() || cbl.Bl.MinerTx.GetHash() != tx.GetHash() {
							panic("miner TX hash mismatch")
						}
					} else {
						bl.TxHashes = append(bl.TxHashes, tx.GetHash())
						cbl.Bl.TxHashes = append(cbl.Bl.TxHashes, tx.GetHash())
						cbl.Txs = append(cbl.Txs, &tx)
					}
				}

				if err, ok := chain.AddCompleteBlock(cbl); ok {
					globals.Logger.Warnf("Block Successfully accepted by chain at height %d", cbl.Bl.MinerTx.Vin[0].(transaction.Txin_gen).Height)
				} else {
					globals.Logger.Warnf("Block rejected by chain at height %d, please investigate, err %s", cbl.Bl.MinerTx.Vin[0].(transaction.Txin_gen).Height, err)
					globals.Logger.Warnf("Stopping import")
				}
			}

			globals.Logger.Infof("File imported Successfully")
			f.Close()

		case command == "profile": // writes cpu and memory profile
			// TODO enable profile over http rpc to enable better testing/tracking
			cpufile, err := os.Create(filepath.Join(globals.GetDataDirectory(), "cpuprofile.prof"))
			if err != nil {
				globals.Logger.Warnf("Could not start cpu profiling, err %s", err)
				continue
			}
			if err := pprof.StartCPUProfile(cpufile); err != nil {
				globals.Logger.Warnf("could not start CPU profile: ", err)
			}
			globals.Logger.Infof("CPU profiling will be available after program shutsdown")
			defer pprof.StopCPUProfile()

		case command == "print_bc":
			log.Info("printing block chain")
			// first is starting point, second is ending point
			start := int64(0)
			stop := int64(0)

			if len(lineParts) != 3 {
				log.Warnf("This function requires 2 parameters, start and endpoint\n")
				continue
			}
			if s, err := strconv.ParseInt(lineParts[1], 10, 64); err == nil {
				start = s
			} else {
				log.Warnf("Invalid start value err %s", err)
				continue
			}

			if s, err := strconv.ParseInt(lineParts[2], 10, 64); err == nil {
				stop = s
			} else {
				log.Warnf("Invalid stop value err %s", err)
				continue
			}

			if start < 0 || start > int64(chain.LoadTopoHeight(nil)) {
				log.Warnf("Start value should be be between 0 and current height\n")
				continue
			}
			if start > stop || stop > int64(chain.LoadTopoHeight(nil)) {
				log.Warnf("Stop value should be > start and current height\n")
				continue

			}

			log.Infof("Printing block chain from %d to %d\n", start, stop)

			for i := start; i <= stop; i++ {
				// get block id at height
				currentBlockId, err := chain.LoadBlockTopologicalOrderAtIndex(nil, i)
				if err != nil {
					log.Infof("Skipping block at height %d due to error %s\n", i, err)
					continue
				}

				timestamp := chain.LoadBlockTimestamp(nil, currentBlockId)
				cdiff := chain.LoadBlockCumulativeDifficulty(nil, currentBlockId)
				diff := chain.LoadBlockDifficulty(nil, currentBlockId)
				//size := chain.

				log.Infof("topo height: %10d,  height %d, "+
					"timestamp: %10d, difficulty: %s cdiff: %s\n", i,
					chain.LoadHeightForBlId(nil, currentBlockId),
					timestamp, diff.String(), cdiff.String())

				log.Infof("Block Id: %s , \n", currentBlockId)
				log.Infof("")

			}
		case command == "mempool_print":
			chain.Mempool.MempoolPrint()

		case command == "mempool_flush":
			chain.Mempool.MempoolFlush()
		case command == "mempool_delete_tx":
			if len(lineParts) == 2 && len(lineParts[1]) == 64 {
				txid, err := hex.DecodeString(strings.ToLower(lineParts[1]))
				if err != nil {
					fmt.Printf("err while decoding txid err %s\n", err)
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(txid))

				chain.Mempool.MempoolDeleteTx(hash)
			} else {
				fmt.Printf("mempool_delete_tx  needs a single transaction id as arugument\n")
			}
		case command == "version":
			fmt.Printf("Version %s OS:%s ARCH:%s \n", config.Version.String(), runtime.GOOS, runtime.GOARCH)

		case command == "start_mining": // it needs 2 parameters, one dark matter address, second number of threads
			if mining {
				fmt.Printf("Mining is already started\n")
				continue
			}

			if globals.Arguments["--lowcpuram"].(bool) {
				globals.Logger.Warnf("Mining is deactivated since daemon is running in low cpu mode, please check program options.")
				continue
			}

			if globals.Arguments["--sync-node"].(bool) {
				globals.Logger.Warnf("Mining is deactivated since daemon is running with --sync-mode, please check program options.")
				continue
			}

			if len(lineParts) != 3 {
				fmt.Printf("This function requires 2 parameters 1) dark matter address  2) number of threads\n")
				continue
			}

			addr, err := globals.ParseValidateAddress(lineParts[1])
			//a, b := blockchain.CreateStealthAddress(addr.SpendKey, addr.ViewKey)
			if err != nil {
				globals.Logger.Warnf("Mining address is invalid: err %s", err)
				continue
			}

			threadCount := 0
			if s, err := strconv.Atoi(lineParts[2]); err == nil {
				//fmt.Printf("%T, %v", s, s)
				threadCount = s

			} else {
				globals.Logger.Warnf("Mining threads argument cannot be parsed: err %s", err)
				continue
			}

			if threadCount > runtime.GOMAXPROCS(0) {
				globals.Logger.Warnf("Mining threads (%d) is more than available CPUs (%d). This is NOT optimal", threadCount, runtime.GOMAXPROCS(0))

			}

			go startMiner(chain, addr, threadCount)
			fmt.Printf("Mining started for %s on %d threads", addr, threadCount)

		case command == "stop_mining":
			if mining == true {
				fmt.Printf("mining stopped\n")
			}
			mining = false

		case command == "print_tree": // prints entire block chain tree
			//WriteBlockChainTree(chain, "/tmp/graph.dot")

		case command == "print_block":
			fmt.Printf("printing block\n")
			if len(lineParts) == 2 && len(lineParts[1]) == 64 {
				bl_raw, err := hex.DecodeString(strings.ToLower(lineParts[1]))

				if err != nil {
					fmt.Printf("err while decoding txid err %s\n", err)
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(bl_raw))

				bl, err := chain.LoadBlFromId(nil, hash)
				if err == nil {
					fmt.Printf("Block ID : %s\n", hash)
					fmt.Printf("Block : %x\n", bl.Serialize())
					fmt.Printf("difficulty: %s\n", chain.LoadBlockDifficulty(nil, hash).String())
					fmt.Printf("cdifficulty: %s\n", chain.LoadBlockCumulativeDifficulty(nil, hash).String())
					fmt.Printf("PoW: %s\n", bl.GetPoWHash())
					//fmt.Printf("Orphan: %v\n",chain.Is_Block_Orphan(hash))

					json_bytes, err := json.Marshal(bl)

					fmt.Printf("%s  err : %s\n", string(prettyprint_json(json_bytes)), err)
				} else {
					fmt.Printf("Err %s\n", err)
				}
			} else if len(lineParts) == 2 {
				if s, err := strconv.ParseInt(lineParts[1], 10, 64); err == nil {
					_ = s
					// first load block id from topo height

					hash, err := chain.LoadBlockTopologicalOrderAtIndex(nil, s)
					if err != nil {
						fmt.Printf("Skipping block at topo height %d due to error %s\n", s, err)
						continue
					}
					bl, err := chain.LoadBlFromId(nil, hash)
					if err == nil {
						fmt.Printf("Block ID : %s\n", hash)
						fmt.Printf("Block : %x\n", bl.Serialize())
						fmt.Printf("difficulty: %s\n", chain.LoadBlockDifficulty(nil, hash).String())
						fmt.Printf("cdifficulty: %s\n", chain.LoadBlockCumulativeDifficulty(nil, hash).String())
						fmt.Printf("Height: %d\n", chain.LoadHeightForBlId(nil, hash))
						fmt.Printf("TopoHeight: %d\n", s)

						fmt.Printf("PoW: %s\n", bl.GetPoWHash())
						//fmt.Printf("Orphan: %v\n",chain.Is_Block_Orphan(hash))

						json_bytes, err := json.Marshal(bl)

						fmt.Printf("%s  err : %s\n", string(prettyprint_json(json_bytes)), err)
					} else {
						fmt.Printf("Err %s\n", err)
					}

				} else {
					fmt.Printf("print_block  needs a single transaction id as arugument\n")
				}
			}

		// can be used to debug/deserialize blocks
		// it can be used for blocks not in chain
		case command == "parse_block":
			if len(lineParts) != 2 {
				globals.Logger.Warnf("parse_block needs a block in hex format")
				continue
			}

			block_raw, err := hex.DecodeString(strings.ToLower(lineParts[1]))
			if err != nil {
				fmt.Printf("err while hex decoding block err %s\n", err)
				continue
			}

			var bl block.Block
			err = bl.Deserialize(block_raw)
			if err != nil {
				globals.Logger.Warnf("Error deserializing block err %s", err)
				continue
			}

			// decode and print block as much as possible
			fmt.Printf("Block ID : %s\n", bl.GetHash())
			fmt.Printf("PoW: %s\n", bl.GetPoWHash()) // block height
			fmt.Printf("Height: %d\n", bl.MinerTx.Vin[0].(transaction.Txin_gen).Height)
			tipsFound := true
			for i := range bl.Tips {
				_, err := chain.LoadBlFromId(nil, bl.Tips[i])
				if err != nil {
					fmt.Printf("Tips %s not in our DB", bl.Tips[i])
					tipsFound = false
					break
				}
			}
			fmt.Printf("Tips: %d %+v\n", len(bl.Tips), bl.Tips)        // block height
			fmt.Printf("Txs: %d %+v\n", len(bl.TxHashes), bl.TxHashes) // block height
			expectedDifficulty := new(big.Int).SetUint64(0)
			if tipsFound { // we can solve diffculty
				expectedDifficulty = chain.GetDifficultyAtTips(nil, bl.Tips)
				fmt.Printf("Difficulty:  %s\n", expectedDifficulty.String())

				powSuccess := chain.VerifyPoW(nil, &bl)
				fmt.Printf("PoW verification %+v\n", powSuccess)

				PoW := bl.GetPoWHash()
				for i := expectedDifficulty.Uint64(); i >= 1; i-- {
					if blockchain.CheckPowHashBig(PoW, new(big.Int).SetUint64(i)) == true {
						fmt.Printf("Block actually has max Difficulty:  %d\n", i)
						break
					}
				}

			} else { // difficulty cann not solved

			}

		case command == "print_tx":
			if len(lineParts) == 2 && len(lineParts[1]) == 64 {
				txid, err := hex.DecodeString(strings.ToLower(lineParts[1]))

				if err != nil {
					fmt.Printf("err while decoding txid err %s\n", err)
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(txid))

				tx, err := chain.LoadTxFromId(nil, hash)
				if err == nil {
					jsonBytes, err := json.MarshalIndent(tx, "", "    ")
					_ = err
					fmt.Printf("%s\n", string(jsonBytes))
				} else {
					tx = chain.Mempool.MempoolGetTx(hash)
					if tx != nil {
						jsonBytes, _ := json.MarshalIndent(tx, "", "    ")
						fmt.Printf("%s\n", string(jsonBytes))
					} else {
						fmt.Printf("Err %s\n", err)
					}
				}
			} else {
				fmt.Printf("print_tx  needs a single transaction id as arugument\n")
			}
		case command == "dev_verify_pool": // verifies and discards any tx which cannot be verified
			txLists := chain.Mempool.MempoolListTx()
			for i := range txLists { // check tx for nay double spend
				tx := chain.Mempool.MempoolGetTx(txLists[i])
				if tx != nil {
					if !chain.VerifyTransactionNonCoinbaseDoubleSpendCheck(nil, tx) {
						fmt.Printf("TX %s is double spended, this TX should not be in pool", txLists[i])
						chain.Mempool.MempoolDeleteTx(txLists[i])
					}
				}
			}

		case strings.ToLower(line) == "diff":
			fmt.Printf("Network %s BH %d, Diff %d, NW Hashrate %0.03f MH/sec  TH %s\n", globals.Config.Name, chain.GetHeight(), chain.GetDifficulty(), float64(chain.GetNetworkHashRate())/1000000.0, chain.GetTopId())

		case strings.ToLower(line) == "status":
			in, out := p2p.PeerDirectionCount()

			supply := chain.LoadAlreadyGeneratedCoinsForTopoIndex(nil, chain.LoadTopoHeight(nil))

			fmt.Printf("Network %s Height %d Network Hashrate %0.03f MH/sec TopoHash %s\n", globals.Config.Name, chain.GetHeight(), float64(chain.GetNetworkHashRate())/1000000.0, chain.GetTopId())
			fmt.Printf("P2P session %d in, %d out MEMPOOL size %d Total Supply %s DMCH %s\n", in, out, len(chain.Mempool.MempoolListTx()), globals.FormatMoney(supply),
				fmt.Sprintf("Version %s OS:%s ARCH:%s", config.Version.String(), runtime.GOOS, runtime.GOARCH))

		case strings.ToLower(line) == "node_list": // print peer node list

			p2p.PeerList_Print()

		case strings.ToLower(line) == "p2p_session": // print active connections
			p2p.Connection_Print(l)
		case command == "list_stake_pool":
			var filter string
			if len(lineParts) >= 2 {
				filter = strings.TrimSpace(lineParts[1])
			}
			showStakePools(chain, filter, l)

		case command == "list_share":
			var filter string
			if len(lineParts) >= 2 {
				filter = strings.TrimSpace(lineParts[1])
			}
			showStakeShares(chain, filter, l)

		case command == "token_list":
			var filter string
			if len(lineParts) >= 2 {
				filter = strings.TrimSpace(lineParts[1])
			}
			tokenList(chain, filter, l)

		case strings.ToLower(line) == "bye":
			fallthrough
		case strings.ToLower(line) == "exit":
			fallthrough
		case strings.ToLower(line) == "quit":
			close(ExitInProgress)
			goto exit

		case command == "graph":
			if len(lineParts) < 2 {
				globals.Logger.Errorf("graph requires start topoheight")
				break
			}
			height, err := strconv.ParseInt(lineParts[1], 10, 64)
			if err != nil {
				globals.Logger.Errorf("invalid start topoheight %s", lineParts[1])
				break
			}

			f, err := os.Create("graph.dot")
			if err != nil {
				break
			}
			chain.WriteBlockChainTree(f, height)
			f.Close()

		case command == "popto":
			if len(lineParts) < 2 {
				globals.Logger.Errorf("popto requires a height argument")
				break
			}
			height, err := strconv.ParseInt(lineParts[1], 10, 64)
			if err != nil {
				globals.Logger.Errorf("invalid target height %s", lineParts[1])
				break
			}

			chainHeight := chain.LoadTopHeight(nil)
			if chainHeight <= height || chainHeight-height > math.MaxInt32 {
				globals.Logger.Warnf("invalid target height %d", height)
				break
			}

			chain.RewindChain(int(chainHeight - height))

		case command == "pop":

			switch len(lineParts) {
			case 1:
				chain.RewindChain(1)
			case 2:
				pop_count := 0
				if s, err := strconv.Atoi(lineParts[1]); err == nil {
					//fmt.Printf("%T, %v", s, s)
					pop_count = s

					if chain.RewindChain(int(pop_count)) {
						globals.Logger.Infof("Rewind successful")
					} else {
						globals.Logger.Infof("Rewind failed")
					}

				} else {
					fmt.Printf("POP needs argument n to pop this many blocks from the top\n")
				}

			default:
				fmt.Printf("POP needs argument n to pop this many blocks from the top\n")
			}

		case command == "ban":
			if len(lineParts) >= 4 || len(lineParts) == 1 {
				fmt.Printf("IP address required to ban\n")
				break
			}

			if len(lineParts) == 3 { // process ban time if provided
				// if user provided a time, apply ban for specific time
				if s, err := strconv.ParseInt(lineParts[2], 10, 64); err == nil && s >= 0 {
					p2p.Ban_Address(lineParts[1], uint64(s))
					break
				} else {
					fmt.Printf("err parsing ban time (only positive number) %s", err)
					break
				}
			}

			err := p2p.Ban_Address(lineParts[1], 10*60) // default ban is 10 minutes
			if err != nil {
				fmt.Printf("err parsing address %s", err)
				break
			}

		case command == "unban":
			if len(lineParts) >= 3 || len(lineParts) == 1 {
				fmt.Printf("IP address required to unban\n")
				break
			}

			err := p2p.UnBan_Address(lineParts[1])
			if err != nil {
				fmt.Printf("err unbanning %s, err = %s", lineParts[1], err)
			} else {
				fmt.Printf("unbann %s successful", lineParts[1])
			}
		case command == "bans":
			p2p.BanList_Print() // print ban list

		case strings.ToLower(line) == "checkpoints": // save all knowns block id

			var blockId crypto.Hash
			checksums := "mainnet_checksums.dat"
			if !globals.IsMainnet() {
				checksums = "testnet_checksums.dat"
			}

			filenameChecksums := filepath.Join(os.TempDir(), checksums)

			fchecksum, err := os.Create(filenameChecksums)
			if err != nil {
				globals.Logger.Warnf("error creating new file %s", err)
				continue
			}

			wchecksums := bufio.NewWriter(fchecksum)

			chain.Lock() // we do not want any reorgs during this op
			height := chain.LoadTopoHeight(nil)
			for i := int64(0); i <= height; i++ {

				blockId, err = chain.LoadBlockTopologicalOrderAtIndex(nil, i)
				if err != nil {
					break
				}

				// calculate sha1 of file
				h := sha3.New256()
				bl, err := chain.LoadBlFromId(nil, blockId)
				if err == nil {
					h.Write(bl.Serialize()) // write serialized block
				} else {
					break
				}
				for j := range bl.TxHashes {
					tx, err := chain.LoadTxFromId(nil, bl.TxHashes[j])
					if err == nil {
						h.Write(tx.Serialize()) // write serialized transaction
					} else {
						break
					}
				}
				if err != nil {
					break
				}

				wchecksums.Write(h.Sum(nil)) // write sha3 256 sum

			}
			if err != nil {
				globals.Logger.Warnf("error writing checkpoints err: %s", err)
			} else {
				globals.Logger.Infof("Successfully wrote %d checksums to file %s", height, filenameChecksums)
			}

			wchecksums.Flush()
			fchecksum.Close()

			chain.Unlock()
		case line == "sleep":
			log.Println("sleep 4 second")
			time.Sleep(4 * time.Second)
		case line == "":
			ourHeight := chain.GetHeight()
			hashRateString := ""
			hashRate := chain.GetNetworkHashRate()
			switch {
			case hashRate > 1000000000000:
				hashRateString = fmt.Sprintf("\033[1m\033[34mNetwork Hashrate \033[0m\033[32m%.1f TH/s", float64(hashRate)/1000000000000.0)
			case hashRate > 1000000000:
				hashRateString = fmt.Sprintf("\033[1m\033[34mNetwork Hashrate \033[0m\033[32m%.1f GH/s", float64(hashRate)/1000000000.0)
			case hashRate > 1000000:
				hashRateString = fmt.Sprintf("\033[1m\033[34mNetwork Hashrate \033[0m\033[32m%.1f MH/s", float64(hashRate)/1000000.0)
			case hashRate > 1000:
				hashRateString = fmt.Sprintf("\033[1m\033[34mNetwork Hashrate \033[0m\033[32m%.1f KH/s", float64(hashRate)/1000.0)
			case hashRate > 0:
				hashRateString = fmt.Sprintf("\033[1m\033[34mNetwork Hashrate \033[0m\033[32m%d H/s", hashRate)
			}

			stakeString := ""
			if chain.GetCurrentVersionAtHeight(ourHeight) >= config.DPOS_FORK_VERSION {
				canVoteNum, totalNum := chain.CalcShareNumber(nil)
				cannotVoteNum := totalNum - canVoteNum

				stakeString = fmt.Sprintf("\033[1m\033[34mNetwork PPoS \033[0m\033[32m%d/%d/%d", cannotVoteNum, canVoteNum, totalNum)
			}

			p2pCount, p2pStatus := p2p.PeerStatus()
			var netStatus string

			switch p2pStatus.Status {
			case p2p.NetGood:
				netStatus = fmt.Sprintf("\033[1;32m%s\033[0m", "Good")
			case p2p.NetFair:
				netStatus = fmt.Sprintf("\033[1;33m%s\033[0m", "Fair")
			case p2p.NetPoor:
				netStatus = fmt.Sprintf("\033[1;31m%s\033[0m", "Poor")
			default:
				netStatus = fmt.Sprintf("\033[1;33m%s\033[0m", "Unknown")
			}

			chainHeightString := fmt.Sprintf("\033[1m\033[34mChain Height \033[0m\033[32m%d", ourHeight)
			p2pString := fmt.Sprintf("\033[1m\033[34mP2P session \033[0m\033[32m%d \033[1m\033[34mStatus", p2pCount)
			netStatusNum := fmt.Sprintf("(%d/%d/%d)", p2pStatus.Good_count, p2pStatus.Fair_count, p2pStatus.Poor_count)
			txPoolString := fmt.Sprintf("\033[1m\033[34mTXpool \033[0m\033[32m%d", len(chain.Mempool.MempoolListTx()))
			l.SetPrompt(fmt.Sprintf("%s %s %s %s %s %s %s\033[0m\n",
				chainHeightString,
				p2pString,
				netStatus,
				netStatusNum,
				txPoolString,
				hashRateString,
				stakeString))
			l.Refresh()
			needUpdatePrompt = true

			//log.Infof("Chain Height %d P2P session %d TXpool %d Network Hashrate %s",
			//	chain.GetHeight(),
			//	p2p.PeerStatus(),
			//	len(chain.Mempool.MempoolListTx()),
			//	hashRateString)
		default:
			log.Println("you said:", strconv.Quote(line))
		}
	}
exit:

	globals.Logger.Infof("Exit in Progress, Please wait")
	time.Sleep(100 * time.Millisecond) // give prompt update time to finish

	rpc.RpcServerStop()
	p2p.P2P_Shutdown() // shutdown p2p subsystem
	chain.Shutdown()   // shutdown chain subsysem

	for globals.SubsystemActive > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

func prettyprint_json(b []byte) []byte {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	_ = err
	return out.Bytes()
}

func usage(w io.Writer) {
	const color = "\033[35m"
	fmt.Fprintf(w, "commands:\n")
	//fmt.Fprintf(w, completer.Tree("    "))
	fmt.Fprintf(w, "\t\033[1m%shelp\033[0m\t\tthis help\n", color)
	fmt.Fprintf(w, "\t\033[1m%sdiff\033[0m\t\tShow difficulty\n", color)
	fmt.Fprintf(w, "\t\033[1m%sprint_bc\033[0m\tPrint blockchain info in a given blocks range, print_bc <begin_height> <end_height>\n", color)
	fmt.Fprintf(w, "\t\033[1m%sprint_block\033[0m\tPrint block, print_block <block_hash> or <block_height>\n", color)
	fmt.Fprintf(w, "\t\033[1m%sprint_height\033[0m\tPrint local blockchain height\n", color)
	fmt.Fprintf(w, "\t\033[1m%sprint_tx\033[0m\tPrint transaction, print_tx <transaction_hash>\n", color)
	fmt.Fprintf(w, "\t\033[1m%sstatus\033[0m\t\tShow general information\n", color)
	fmt.Fprintf(w, "\t\033[1m%sstart_mining\033[0m\tStart mining <dark matter address> <number of threads>\n", color)
	fmt.Fprintf(w, "\t\033[1m%sstop_mining\033[0m\tStop daemon mining\n", color)
	fmt.Fprintf(w, "\t\033[1m%snode_list\033[0m\tPrint peer node list\n", color)
	fmt.Fprintf(w, "\t\033[1m%sp2p_session\033[0m\tPrint information about connected p2p node and their state\n", color)
	fmt.Fprintf(w, "\t\033[1m%sgraph\033[0m\t\tSave block topological order to a graph.dot file\n", color)
	fmt.Fprintf(w, "\t\033[1m%stoken_list\033[0m\tList Omni Token\n", color)
	fmt.Fprintf(w, "\t\033[1m%slist_stake_pool\033[0m\tPrint information about all stake pools\n", color)
	fmt.Fprintf(w, "\t\033[1m%slist_share\033[0m\tPrint information about all shares\n", color)
	fmt.Fprintf(w, "\t\033[1m%sbye\033[0m\t\tQuit the daemon\n", color)
	fmt.Fprintf(w, "\t\033[1m%sban\033[0m\t\tBan specific ip from making any connections\n", color)
	fmt.Fprintf(w, "\t\033[1m%sunban\033[0m\t\tRevoke restrictions on previously banned ips\n", color)
	fmt.Fprintf(w, "\t\033[1m%sbans\033[0m\t\tPrint current ban list\n", color)
	fmt.Fprintf(w, "\t\033[1m%sversion\033[0m\t\tShow version\n", color)
	fmt.Fprintf(w, "\t\033[1m%sexit\033[0m\t\tQuit the daemon\n", color)
	fmt.Fprintf(w, "\t\033[1m%squit\033[0m\t\tQuit the daemon\n", color)

}

var completer = readline.NewPrefixCompleter(
	/*	readline.PcItem("mode",
			readline.PcItem("vi"),
			readline.PcItem("emacs"),
		),
		readline.PcItem("login"),
		readline.PcItem("say",
			readline.PcItem("hello"),
			readline.PcItem("bye"),
		),
		readline.PcItem("setprompt"),
		readline.PcItem("setpassword"),
		readline.PcItem("bye"),
	*/
	readline.PcItem("help"),
	/*	readline.PcItem("go",
			readline.PcItem("build", readline.PcItem("-o"), readline.PcItem("-v")),
			readline.PcItem("install",
				readline.PcItem("-v"),
				readline.PcItem("-vv"),
				readline.PcItem("-vvv"),
			),
			readline.PcItem("test"),
		),
		readline.PcItem("sleep"),
	*/
	readline.PcItem("diff"),
	readline.PcItem("dev_verify_pool"),
	readline.PcItem("dev_verify_chain_doublespend"),
	readline.PcItem("mempool_flush"),
	readline.PcItem("mempool_delete_tx"),
	readline.PcItem("mempool_print"),
	readline.PcItem("node_list"),
	readline.PcItem("print_bc"),
	readline.PcItem("print_block"),
	readline.PcItem("print_height"),
	readline.PcItem("print_tx"),
	readline.PcItem("status"),
	readline.PcItem("start_mining"),
	readline.PcItem("stop_mining"),
	readline.PcItem("p2p_session"),
	readline.PcItem("graph"),
	readline.PcItem("token_list"),
	readline.PcItem("list_stake_pool"),
	readline.PcItem("list_share"),
	readline.PcItem("version"),
	readline.PcItem("bye"),
	readline.PcItem("exit"),
	readline.PcItem("quit"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}
