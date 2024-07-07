package replica

import (
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"go.uber.org/atomic"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/hotstuff"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/mempool"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

type Replica struct {
	node.Node
	Safety
	election.Election
	pd              *mempool.Producer
	pm              *pacemaker.Pacemaker
	start           chan bool // signal to start the node
	isStarted       atomic.Bool
	isByz           bool
	timer           *time.Timer // timeout for each view
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	eventChan       chan interface{}

	/* for monitoring node statistics */
	thrus                string
	lastViewTime         time.Time
	startTime            time.Time
	tmpTime              time.Time
	voteStart            time.Time
	totalCreateDuration  time.Duration
	totalProcessDuration time.Duration
	totalProposeDuration time.Duration
	totalDelay           time.Duration
	totalRoundTime       time.Duration
	totalVoteTime        time.Duration
	totalBlockSize       int
	receivedNo           int
	roundNo              int
	voteNo               int
	totalCommittedTx     int
	latencyNo            int
	proposedNo           int
	processedNo          int
	committedNo          int
	forkedNo             int
	maliNo               int
	kindNo               int
	totolCommitTimes     int
	lastComittedView     types.View
}

// NewReplica creates a new replica instance
func NewReplica(id identity.NodeID, alg string, isByz bool) *Replica {
	r := new(Replica)
	r.Node = node.NewNode(id, isByz)
	if isByz {
		log.Infof("[%v] is Byzantine", r.ID())
	}
	if config.GetConfig().Master == "0" {
		r.Election = election.NewRotation(config.GetConfig().N())
	} else {
		r.Election = election.NewStatic(config.GetConfig().Master)
	}
	r.isByz = isByz
	r.pd = mempool.NewProducer()
	r.pm = pacemaker.NewPacemaker(config.GetConfig().N())
	r.start = make(chan bool)
	r.eventChan = make(chan interface{})
	r.committedBlocks = make(chan *blockchain.Block, 100)
	r.forkedBlocks = make(chan *blockchain.Block, 100)
	r.Register(blockchain.Block{}, r.HandleBlock)
	r.Register(blockchain.Vote{}, r.HandleVote)
	r.Register(pacemaker.TMO{}, r.HandleTmo)
	r.Register(message.Transaction{}, r.handleTxn)
	r.Register(message.Query{}, r.handleQuery)
	r.Register(pacemaker.VMO{}, r.HandleVmo)
	gob.Register(blockchain.Block{})
	gob.Register(blockchain.Vote{})
	gob.Register(pacemaker.TC{})
	gob.Register(pacemaker.TMO{})
	gob.Register(pacemaker.VC{})
	gob.Register(pacemaker.VMO{})

	// Is there a better way to reduce the number of parameters?
	switch config.GetConfig().Algorithm {
	case "hotstuff":
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	// case "tchs":
	// 	r.Safety = tchs.NewTchs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	// case "streamlet":
	// 	r.Safety = streamlet.NewStreamlet(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	// case "lbft":
	// 	r.Safety = lbft.NewLbft(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	// case "fasthotstuff":
	// 	r.Safety = fhs.NewFhs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	default:
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	}
	return r
}

/* Message Handlers */

func (r *Replica) HandleBlock(block blockchain.Block) {
	r.receivedNo++
	r.startSignal()
	r.eventChan <- block
}

func (r *Replica) HandleVote(vote blockchain.Vote) {
	if config.GetConfig().Algorithm != "streamlet" {
		if vote.View < r.pm.GetCurView() {
			return
		}
	}

	r.startSignal()
	log.Debugf("[%v] received a vote frm %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	r.eventChan <- vote
}

func (r *Replica) HandleTmo(tmo pacemaker.TMO) {
	if tmo.View < r.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] received a timeout from %v for view %v", r.ID(), tmo.NodeID, tmo.View)
	r.eventChan <- tmo
}

func (r *Replica) HandleVmo(vmo pacemaker.VMO) {
	if vmo.View < r.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] received a vote from %v for view %v", r.ID(), vmo.NodeID, vmo.View)
	r.eventChan <- vmo
}

// handleQuery replies a query with the statistics of the node
func (r *Replica) handleQuery(m message.Query) {
	//realAveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.processedNo)
	//aveProcessTime := float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	//aveVoteProcessTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.roundNo)
	//aveBlockSize := float64(r.totalBlockSize) / float64(r.proposedNo)
	//requestRate := float64(r.pd.TotalReceivedTxNo()) / time.Now().Sub(r.startTime).Seconds()
	//committedRate := float64(r.committedNo) / time.Now().Sub(r.startTime).Seconds()
	//aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	//aveProposeTime := aveRoundTime - aveProcessTime - aveVoteProcessTime
	latency := float64(r.totalDelay.Milliseconds()) / float64(r.latencyNo)
	bsize := config.GetConfig().BSize
	// r.thrus += fmt.Sprintf("Time: %v s. Throughput: %v txs/s.\n", time.Now().Sub(r.startTime).Seconds(), float64(r.totalCommittedTx-r.maliNo)/time.Now().Sub(r.tmpTime).Seconds())
	r.thrus += fmt.Sprintf("Time: %v s. Throughput: %v txs/s.\n", time.Now().Sub(r.startTime).Seconds(), float64(bsize*(r.kindNo-r.forkedNo*config.GetConfig().ByzNo))/time.Now().Sub(r.startTime).Seconds())
	r.thrus += fmt.Sprintf("maliBlock: %v. forkedBlock: %v. totalNo: %v. kindNo: %v.\n", r.maliNo, r.forkedNo*config.GetConfig().ByzNo, r.committedNo, r.kindNo)
	r.thrus += fmt.Sprintf("chainQuality: %v.\n", float64(r.kindNo)/float64(r.committedNo))
	r.thrus += fmt.Sprintf("censorship: %v.\n", float64(r.kindNo)/float64(r.kindNo+r.forkedNo*config.GetConfig().ByzNo))
	r.thrus += fmt.Sprintf("latency: %v.\n", float64(r.totolCommitTimes)/float64(r.pm.GetCurView()))
	r.thrus += fmt.Sprintf("chain growth: %v.\n", float64(r.kindNo)/float64(r.pm.GetCurView()))
	r.thrus += fmt.Sprintf("commitTimes: %v .\n", r.totolCommitTimes)
	r.thrus += fmt.Sprintf("view: %v .\n", r.pm.GetCurView())
	r.thrus += fmt.Sprintf("\n")
	r.totalCommittedTx = 0
	// r.tmpTime = time.Now()
	status := fmt.Sprintf("Latency: %v\n%s", latency, r.thrus)
	//status := fmt.Sprintf("chain status is: %s\nCommitted rate is %v.\nAve. block size is %v.\nAve. trans. delay is %v ms.\nAve. creation time is %f ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nRequest rate is %f txs/s.\nAve. round time is %f ms.\nLatency is %f ms.\nThroughput is %f txs/s.\n", r.Safety.GetChainStatus(), committedRate, aveBlockSize, aveTransDelay, aveCreateDuration, aveProcessTime, aveVoteProcessTime, requestRate, aveRoundTime, latency, throughput)
	//status := fmt.Sprintf("Ave. actual proposing time is %v ms.\nAve. proposing time is %v ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nAve. block size is %v.\nAve. round time is %v ms.\nLatency is %v ms.\n", realAveProposeTime, aveProposeTime, aveProcessTime, aveVoteProcessTime, aveBlockSize, aveRoundTime, latency)
	m.Reply(message.QueryReply{Info: status})
}

func (r *Replica) handleTxn(m message.Transaction) {
	r.pd.AddTxn(&m)
	r.startSignal()
	// the first leader kicks off the protocol
	if r.pm.GetCurView() == 0 && r.IsLeader(r.ID(), 1) {
		log.Debugf("[%v] is going to kick off the protocol", r.ID())
		r.pm.AdvanceView(0)
	}
}

/* Processors */

func (r *Replica) processCommittedBlock(block *blockchain.Block) {
	if block.CommitFromThis {
		r.totolCommitTimes++
	}

	if block.Proposer == r.ID() {
		for _, txn := range block.Payload {
			// only record the delay of transactions from the local memory pool
			delay := time.Now().Sub(txn.Timestamp)
			r.totalDelay += delay
			r.latencyNo++
		}
	}
	r.committedNo++
	r.totalCommittedTx += len(block.Payload)

	if r.committedNo%1000 == 0 && r.ID().Node() == 1 && false {
		// 打开文件，如果文件不存在则创建
		indexFile, err := os.OpenFile(config.Configuration.Algorithm+"_index.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("open file error: %v", err)
		}

		chainQualityFile, _ := os.OpenFile(config.Configuration.Algorithm+"_chainQuality.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		censorship, _ := os.OpenFile(config.Configuration.Algorithm+"_censorship.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		maliBlock, _ := os.OpenFile(config.Configuration.Algorithm+"_maliBlock.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		forkedBlock, _ := os.OpenFile(config.Configuration.Algorithm+"_forkedBlock.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		totalBlock, _ := os.OpenFile(config.Configuration.Algorithm+"_totalBlock.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		kindBlock, _ := os.OpenFile(config.Configuration.Algorithm+"_kindBlock.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		// defer语句会在函数执行到最后执行，而不是在调用defer语句的时候执行

		defer chainQualityFile.Close()
		defer indexFile.Close()
		defer censorship.Close()
		defer maliBlock.Close()
		defer forkedBlock.Close()
		defer totalBlock.Close()
		defer kindBlock.Close()
		// 写入内容
		indexFile.WriteString(fmt.Sprintf("%d,", r.committedNo))
		chainQualityFile.WriteString(fmt.Sprintf("%f,", float64(r.kindNo)/float64(r.committedNo)))
		censorship.WriteString(fmt.Sprintf("%f,", float64(r.kindNo)/float64(r.kindNo+r.forkedNo*config.GetConfig().ByzNo)))
		maliBlock.WriteString(fmt.Sprintf("%d,", r.maliNo))
		forkedBlock.WriteString(fmt.Sprintf("%d,", r.forkedNo*config.GetConfig().ByzNo))
		totalBlock.WriteString(fmt.Sprintf("%d,", r.committedNo))
		kindBlock.WriteString(fmt.Sprintf("%d,", r.kindNo))
	}

	// count the number of malicious blocks and the number of forked blocks
	if block.GetMali() {
		r.maliNo++
		r.forkedNo += block.GetForkNum()
	} else {
		r.kindNo++
	}
	log.Infof("[%v] the block is committed, No. of transactions: %v, view: %v, current view: %v, id: %x, forkNum: %v, Byz: %v, commitFromThis %v", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID, block.GetForkNum(), block.GetMali(), block.CommitFromThis)
}

func (r *Replica) processForkedBlock(block *blockchain.Block) {
	if block.Proposer == r.ID() {
		for _, txn := range block.Payload {
			// collect txn back to mem pool
			r.pd.CollectTxn(txn)
		}
	}
	log.Infof("[%v] the block is forked, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID)
}

func (r *Replica) processNewView(newView types.View) {
	log.Debugf("[%v] is processing new view: %v, leader is %v", r.ID(), newView, r.FindLeaderFor(newView))
	if !r.IsLeader(r.ID(), newView) {
		return
	}

	// if r.isByz {
	// 	return
	// }

	r.proposeBlock(newView)
}

func (r *Replica) proposeBlock(view types.View) {
	createStart := time.Now()
	block := r.Safety.MakeProposal(view, r.pd.GeneratePayload())
	if block == nil {
		return
	}
	r.totalBlockSize += len(block.Payload)
	r.proposedNo++
	createEnd := time.Now()
	createDuration := createEnd.Sub(createStart)
	block.Timestamp = time.Now()
	r.totalCreateDuration += createDuration
	// if the Algorithm is streamlet, the leader will broadcast the block
	if config.GetConfig().Algorithm != "streamlet" {
		r.Broadcast(block)
	}
	_ = r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

// ListenLocalEvent listens new view and timeout events
func (r *Replica) ListenLocalEvent() {
	r.lastViewTime = time.Now()
	r.timer = time.NewTimer(r.pm.GetTimerForView())
	for {
		r.timer.Reset(r.pm.GetTimerForView())
	L:
		for {
			select {
			case view := <-r.pm.EnteringViewEvent():
				if view >= 2 {
					r.totalVoteTime += time.Now().Sub(r.voteStart)
				}
				// measure round time
				now := time.Now()
				lasts := now.Sub(r.lastViewTime)
				r.totalRoundTime += lasts
				r.roundNo++
				r.lastViewTime = now
				r.eventChan <- view
				log.Debugf("[%v] the last view lasts %v milliseconds, current view: %v", r.ID(), lasts.Milliseconds(), view)
				break L
			case <-r.timer.C:
				r.Safety.ProcessLocalTmo(r.pm.GetCurView())
				break L
			}
		}
	}
}

// ListenCommittedBlocks listens committed blocks and forked blocks from the protocols
func (r *Replica) ListenCommittedBlocks() {
	for {
		select {
		case committedBlock := <-r.committedBlocks:
			r.processCommittedBlock(committedBlock)
		case forkedBlock := <-r.forkedBlocks:
			r.processForkedBlock(forkedBlock)
		}
	}
}

func (r *Replica) startSignal() {
	if !r.isStarted.Load() {
		r.startTime = time.Now()
		r.tmpTime = time.Now()
		log.Debugf("[%v] is boosting", r.ID())
		r.isStarted.Store(true)
		r.start <- true
	}
}

// Start starts event loop
func (r *Replica) Start() {
	go r.Run()
	// wait for the start signal
	<-r.start
	go r.ListenLocalEvent()
	go r.ListenCommittedBlocks()
	for r.isStarted.Load() {
		event := <-r.eventChan
		switch v := event.(type) {
		case types.View:
			r.processNewView(v)
		case blockchain.Block:
			startProcessTime := time.Now()
			r.totalProposeDuration += startProcessTime.Sub(v.Timestamp)
			_ = r.Safety.ProcessBlock(&v)
			r.totalProcessDuration += time.Now().Sub(startProcessTime)
			r.voteStart = time.Now()
			r.processedNo++
		case blockchain.Vote:
			startProcessTime := time.Now()
			r.Safety.ProcessVote(&v)
			processingDuration := time.Now().Sub(startProcessTime)
			r.totalVoteTime += processingDuration
			r.voteNo++
		case pacemaker.TMO:
			r.Safety.ProcessRemoteTmo(&v)
		case pacemaker.VMO:
			r.Safety.ProcessRemoteVmo(&v)
		}
	}
}
