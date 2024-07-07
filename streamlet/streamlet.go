package streamlet

import (
	"fmt"
	"time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

type Streamlet struct {
	node.Node
	election.Election
	pm                     *pacemaker.Pacemaker
	bc                     *blockchain.BlockChain
	notarizedChain         [][]*blockchain.Block
	bufferedBlocks         map[crypto.Identifier]*blockchain.Block
	bufferedQCs            map[crypto.Identifier]*blockchain.QC
	bufferedNotarizedBlock map[crypto.Identifier]*blockchain.QC
	committedBlocks        chan *blockchain.Block
	forkedBlocks           chan *blockchain.Block
	echoedBlock            map[crypto.Identifier]struct{}
	echoedVote             map[crypto.Identifier]struct{}
}

// NewStreamlet creates a new Streamlet instance
func NewStreamlet(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block) *Streamlet {
	sl := new(Streamlet)
	sl.Node = node
	sl.Election = elec
	sl.pm = pm
	sl.committedBlocks = committedBlocks
	sl.forkedBlocks = forkedBlocks
	sl.bc = blockchain.NewBlockchain(config.GetConfig().N())
	sl.bufferedBlocks = make(map[crypto.Identifier]*blockchain.Block)
	sl.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	sl.bufferedNotarizedBlock = make(map[crypto.Identifier]*blockchain.QC)
	sl.notarizedChain = make([][]*blockchain.Block, 0)
	sl.echoedBlock = make(map[crypto.Identifier]struct{})
	sl.echoedVote = make(map[crypto.Identifier]struct{})
	sl.pm.AdvanceView(0)
	return sl
}

// ProcessBlock processes an incoming block as follows:
// 1. check if the view of the block matches current view (ignore for now)
// 2. check if the view of the block matches the proposer's view (ignore for now)
// 3. insert the block into the block tree
// 4. if the view of the block is lower than the current view, don't vote
// 5. if the block is extending the longest notarized chain, vote for the block
// 6. if the view of the block is higher than the the current view, buffer the block
// and process it when entering that view
func (sl *Streamlet) ProcessBlock(block *blockchain.Block) error {
	if sl.bc.Exists(block.ID) {
		return nil
	}

	// 当前区块是恶意且上一个也是
	// if block.Mali && block.GetForkNum() == 0 && sl.IsByz() {
	// 	return nil
	// }

	log.Debugf("[%v] is processing block, view: %v, id: %x", sl.ID(), block.View, block.ID)
	curView := sl.pm.GetCurView()
	if block.View < curView {
		return fmt.Errorf("received a stale block")
	}

	_, err := sl.bc.GetBlockByID(block.PrevID)
	if err != nil && block.View > 1 {
		// buffer future blocks
		sl.bufferedBlocks[block.PrevID] = block
		log.Debugf("[%v] buffer the block for future processing, view: %v, id: %x", sl.ID(), block.View, block.ID)
		return nil
	}
	if !sl.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	if block.Proposer != sl.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", sl.ID())
		}
	}
	_, exists := sl.echoedBlock[block.ID]
	if !exists {
		sl.echoedBlock[block.ID] = struct{}{}
		// sl.Broadcast(block)
		if sl.IsByz() && sl.Election.FindLeaderFor(sl.pm.GetCurView()+1).Node() > config.GetConfig().ByzNo {
			sl.MaliBroadcast(block, false)
		} else {
			sl.NoMailBroadcast(block)
		}
	}
	sl.bc.AddBlock(block)
	shouldVote := sl.votingRule(block)
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", sl.ID(), block.ID)
		sl.bufferedBlocks[block.PrevID] = block
		log.Debugf("[%v] buffer the block for future processing, view: %v, id: %x", sl.ID(), block.View, block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, sl.ID(), block.ID)
	// vote to the current leader
	sl.ProcessVote(vote)
	// sl.Broadcast(vote)
	if sl.IsByz() && sl.Election.FindLeaderFor(sl.pm.GetCurView()+1).Node() > config.GetConfig().ByzNo {
		sl.MaliBroadcast(vote, true)
	} else {
		sl.NoMailBroadcast(vote)
	}

	// 添加两个方法，一个给恶意调用，一个给非恶意调用
	// 基本思想是，如果是恶意调用，就将区块广播给除了leader外的f+1个诚实节点和所有恶意节点
	// 并且广播时间保证诚实节点的广播会在超时后才被其他节点看到
	// 如果是非恶意调用，就将区块广播给所有节点，但是有一个恒定的时间延迟

	// process buffers
	qc, ok := sl.bufferedQCs[block.ID]
	if ok {
		sl.processCertificate(qc)
	}
	b, ok := sl.bufferedBlocks[block.ID]
	if ok {
		_ = sl.ProcessBlock(b)
	}
	return nil
}

func (sl *Streamlet) MaliBroadcast(m interface{}, isVote bool) {
	// log.Debugf("[%v] is broadcasting a mali message", sl.ID())
	// 恶意广播
	// 一个时间延迟
	deley := config.GetConfig().Timeout * 5 / 7
	// 找到下一个leader，如果是恶意节点，就广播给所有节点
	nextLeaderId := sl.Election.FindLeaderFor(sl.pm.GetCurView() + 1).Node()
	if nextLeaderId <= config.GetConfig().ByzNo {
		sl.NoMailBroadcast(m)
	} else if isVote {
		nodeId := 1
		deley := config.GetConfig().Timeout * 6 / 7
		// 投票发给除leader外的所有节点. 这里排除掉了leader
		for nodeId <= config.GetConfig().N()-1 {
			if nodeId == nextLeaderId {
				nodeId++
			}
			currentId := nodeId
			delay := time.Duration(deley) * time.Millisecond
			time.AfterFunc(delay, func() {
				// log.Debugf("[%v] is sending mali message to %v", sl.ID(), currentId)
				sl.Send(identity.NewNodeID(currentId), m)
			})
			nodeId++
		}
	} else {
		nodeId := 1
		for nodeId <= config.GetConfig().N() {
			currentId := nodeId
			// currentView := sl.pm.GetCurView()
			delay := time.Duration(deley) * time.Millisecond
			time.AfterFunc(delay, func() {
				// log.Debugf("[%v] is sending message to %v, currentView is %v", sl.ID(), currentId, currentView)
				sl.Send(identity.NewNodeID(currentId), m)
			})
			nodeId++
		}
	}

}

func (sl *Streamlet) NoMailBroadcast(m interface{}) {
	log.Debugf("[%v] is broadcasting a normal message, view is %v", sl.ID(), sl.pm.GetCurView())
	deley := config.GetConfig().Timeout * 3 / 7
	nodeId := 1

	for nodeId <= config.GetConfig().N() {
		currentId := nodeId
		// currentView := sl.pm.GetCurView()
		delay := time.Duration(deley) * time.Millisecond
		time.AfterFunc(delay, func() {
			// log.Debugf("[%v] is sending message to %v, currentView is %v", sl.ID(), currentId, currentView)
			sl.Send(identity.NewNodeID(currentId), m)
		})
		nodeId++
	}
}

func (sl *Streamlet) ProcessVote(vote *blockchain.Vote) {

	log.Debugf("[%v] is processing the vote, block id: %x", sl.ID(), vote.BlockID)
	if vote.Voter != sl.ID() {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Fatalf("[%v] Error in verifying the signature in vote id: %x", sl.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with invalid signature. vote id: %x", sl.ID(), vote.BlockID)
			return
		}
	}
	// echo the message
	_, exists := sl.echoedBlock[vote.BlockID]
	if !exists {
		sl.echoedBlock[vote.BlockID] = struct{}{}
		sl.NoMailBroadcast(vote)
	}
	isBuilt, qc := sl.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] votes are not sufficient to build a qc, view: %v, block id: %x", sl.ID(), vote.View, vote.BlockID)
		return
	}
	// send the QC to the next leader
	log.Debugf("[%v] a qc is built, view: %v, block id: %x", sl.ID(), qc.View, qc.BlockID)
	sl.processCertificate(qc)

	return
}

func (sl *Streamlet) ProcessRemoteTmo(tmo *pacemaker.TMO) {
	log.Debugf("[%v] is processing tmo from %v, current view is %v", sl.ID(), tmo.NodeID, sl.pm.GetCurView())
	isBuilt, tc := sl.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		log.Debugf("[%v] not enough tc for %v", sl.ID(), tmo.View)
		return
	}
	log.Debugf("[%v] a tc is built for view %v", sl.ID(), tc.View)
	sl.processTC(tc)
}

func (sl *Streamlet) ProcessLocalTmo(view types.View) {
	tmo := &pacemaker.TMO{
		View:   view,
		NodeID: sl.ID(),
	}
	sl.Broadcast(tmo)
	sl.ProcessRemoteTmo(tmo)
}

func (sl *Streamlet) MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block {

	prevID, forkNum, silent := sl.forkChoice()
	if silent {
		return nil
	}
	block := blockchain.MakeBlock(view, &blockchain.QC{
		View:      0,
		BlockID:   prevID,
		AggSig:    nil,
		Signature: nil,
	}, prevID, payload, sl.ID(), sl.IsByz(), forkNum, 0)
	return block
}

func (sl *Streamlet) forkChoice() (crypto.Identifier, int, bool) {
	var prevID crypto.Identifier
	var forkNum int
	var silent bool
	// 如果有双链结构
	var height = sl.GetNotarizedHeight()
	if height > 3 && sl.IsByz() {
		lastBlocks := sl.notarizedChain[height-1]
		secondBlocks := sl.notarizedChain[height-2]
		if len(lastBlocks) == 1 && len(secondBlocks) == 1 {
			if lastBlocks[0].View == secondBlocks[0].View+1 && lastBlocks[0].View+1 == sl.pm.GetCurView() {
				silent = true
				return prevID, 0, silent
			}
		}
	}

	if sl.GetNotarizedHeight() == 0 {
		prevID = crypto.MakeID("Genesis block")
	} else {
		tailNotarizedBlock := sl.notarizedChain[sl.GetNotarizedHeight()-1][0]
		prevID = tailNotarizedBlock.ID
	}

	if sl.Election.FindLeaderFor(sl.pm.GetCurView()+1).Node() > config.GetConfig().ByzNo && sl.IsByz() {
		forkNum = 1
	} else {
		forkNum = 0
	}
	silent = false
	return prevID, forkNum, silent
}

func (sl *Streamlet) processTC(tc *pacemaker.TC) {
	if tc.View < sl.pm.GetCurView() {
		return
	}
	go sl.pm.AdvanceView(tc.View)
}

// 1. advance view
// 2. update notarized chain
// 3. check commit rule
// 4. commit blocks
func (sl *Streamlet) processCertificate(qc *blockchain.QC) {
	log.Debugf("[%v] is processing a qc, view: %v, block id: %x", sl.ID(), qc.View, qc.BlockID)
	// silent一定会导致view不一致
	// if qc.View < sl.pm.GetCurView() {
	// 	return
	// }
	_, err := sl.bc.GetBlockByID(qc.BlockID)
	if err != nil && qc.View > 1 {
		log.Debugf("[%v] buffered the QC, view: %v, id: %x", sl.ID(), qc.View, qc.BlockID)
		sl.bufferedQCs[qc.BlockID] = qc
		return
	}
	if qc.Leader != sl.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if quorumIsVerified == false {
			log.Warningf("[%v] received a quorum with invalid signatures", sl.ID())
			return
		}
	}
	err = sl.updateNotarizedChain(qc)
	if err != nil {
		// the corresponding block does not exist
		log.Debugf("[%v] cannot notarize the block, %x: %w", sl.ID(), qc.BlockID, err)
		return
	}
	sl.pm.AdvanceView(qc.View)
	if qc.View < 3 {
		return
	}
	ok, block := sl.commitRule()
	if !ok {
		return
	}
	committedBlocks, forkedBlocks, err := sl.bc.CommitBlock(block.ID, sl.pm.GetCurView())
	if err != nil {
		log.Errorf("[%v] cannot commit blocks", sl.ID())
		return
	}

	var heightestBlock *blockchain.Block

	for _, cBlock := range committedBlocks {
		if heightestBlock == nil || int(cBlock.View) > int(heightestBlock.View) {
			heightestBlock = cBlock
		}
	}
	if heightestBlock != nil {
		heightestBlock.CommitFromThis = true
	}

	for _, cBlock := range committedBlocks {
		sl.committedBlocks <- cBlock
		delete(sl.echoedBlock, cBlock.ID)
		delete(sl.echoedVote, cBlock.ID)
		log.Debugf("[%v] is going to commit block, view: %v, id: %x", sl.ID(), cBlock.View, cBlock.ID)
	}

	for _, fBlock := range forkedBlocks {
		sl.forkedBlocks <- fBlock
		log.Debugf("[%v] is going to collect forked block, view: %v, id: %x", sl.ID(), fBlock.View, fBlock.ID)
	}
	b, ok := sl.bufferedBlocks[qc.BlockID]
	if ok {
		log.Debugf("[%v] found a buffered block by qc, qc.BlockID: %x", sl.ID(), qc.BlockID)
		_ = sl.ProcessBlock(b)
		delete(sl.bufferedBlocks, qc.BlockID)
	}
	qc, ok = sl.bufferedNotarizedBlock[qc.BlockID]
	if ok {
		log.Debugf("[%v] found a bufferred qc, view: %v, block id: %x", sl.ID(), qc.View, qc.BlockID)
		sl.processCertificate(qc)
		delete(sl.bufferedQCs, qc.BlockID)
	}
}

func (sl *Streamlet) updateNotarizedChain(qc *blockchain.QC) error {
	block, err := sl.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot find the block")
	}
	// check the last block in the notarized chain
	// could be improved by checking view
	if sl.GetNotarizedHeight() == 0 {
		log.Debugf("[%v] is processing the first notarized block, view: %v, id: %x", sl.ID(), qc.View, qc.BlockID)
		newArray := make([]*blockchain.Block, 0)
		newArray = append(newArray, block)
		sl.notarizedChain = append(sl.notarizedChain, newArray)
		return nil
	}
	for i := sl.GetNotarizedHeight() - 1; i >= 0 || i >= sl.GetNotarizedHeight()-3; i-- {
		lastBlocks := sl.notarizedChain[i]
		for _, b := range lastBlocks {
			if b.ID == block.PrevID {
				var blocks []*blockchain.Block
				if i < sl.GetNotarizedHeight()-1 {
					blocks = make([]*blockchain.Block, 0)
				}
				blocks = append(blocks, block)
				sl.notarizedChain = append(sl.notarizedChain, blocks)
				return nil
			}
		}
	}
	sl.bufferedNotarizedBlock[block.PrevID] = qc
	log.Debugf("[%v] the parent block is not notarized, buffered for now, view: %v, block id: %x", sl.ID(), qc.View, qc.BlockID)
	return fmt.Errorf("the block is not extending the notarized chain")
}

func (sl *Streamlet) GetChainStatus() string {
	chainGrowthRate := sl.bc.GetChainGrowth()
	blockIntervals := sl.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", sl.ID(), sl.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (sl *Streamlet) GetNotarizedHeight() int {
	return len(sl.notarizedChain)
}

// 1. get the tail of the longest notarized chain (could be more than one)
// 2. check if the block is extending one of them
func (sl *Streamlet) votingRule(block *blockchain.Block) bool {
	if block.View <= 2 {
		return true
	}
	lastBlocks := sl.notarizedChain[sl.GetNotarizedHeight()-1]
	for _, b := range lastBlocks {
		if block.PrevID == b.ID {
			return true
		}
	}

	return false
}

// 1. get the last three blocks in the notarized chain
// 2. check if they are consecutive
// 3. if so, return the second block to commit
func (sl *Streamlet) commitRule() (bool, *blockchain.Block) {
	height := sl.GetNotarizedHeight()
	if height < 3 {
		return false, nil
	}
	lastBlocks := sl.notarizedChain[height-1]
	if len(lastBlocks) != 1 {
		return false, nil
	}
	lastBlock := lastBlocks[0]
	secondBlocks := sl.notarizedChain[height-2]
	if len(secondBlocks) != 1 {
		return false, nil
	}
	secondBlock := secondBlocks[0]
	firstBlocks := sl.notarizedChain[height-3]
	if len(firstBlocks) != 1 {
		return false, nil
	}
	firstBlock := firstBlocks[0]
	// check three-chain
	if ((firstBlock.View + 1) == secondBlock.View) && ((secondBlock.View + 1) == lastBlock.View) {
		return true, secondBlock
	}
	return false, nil
}
