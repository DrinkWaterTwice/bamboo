package hotstuff

import (
	"fmt"
	"sync"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

const FORK = "fork"

type HotStuff struct {
	node.Node
	election.Election
	pm              *pacemaker.Pacemaker
	preferredView   types.View
	highQC          *blockchain.QC
	height          int
	bc              *blockchain.BlockChain
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	bufferedQCs     map[crypto.Identifier]*blockchain.QC
	bufferedBlocks  map[types.View]*blockchain.Block
	mu              sync.Mutex
}

func NewHotStuff(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block) *HotStuff {
	hs := new(HotStuff)
	hs.Node = node
	hs.Election = elec
	hs.pm = pm
	hs.bc = blockchain.NewBlockchain(config.GetConfig().N())
	hs.bufferedBlocks = make(map[types.View]*blockchain.Block)
	hs.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	hs.highQC = &blockchain.QC{View: 0}
	hs.committedBlocks = committedBlocks
	hs.forkedBlocks = forkedBlocks
	return hs
}

func (hs *HotStuff) ProcessBlock(block *blockchain.Block) error {

	log.Debugf("[%v] is processing block from %v, view: %v, id: %x, isFork: %v", hs.ID(), block.Proposer.Node(), block.View, block.ID, block.GetForkNum())
	curView := hs.pm.GetCurView()
	if block.Proposer != hs.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", hs.ID())
		}
	}
	if block.View > curView+1 {
		//	buffer the block
		hs.bufferedBlocks[block.View-1] = block
		log.Debugf("[%v] the block is buffered, id: %x", hs.ID(), block.ID)
		return nil
	}
	if block.QC != nil {
		hs.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}
	// does not have to process the QC if the replica is the proposer
	if block.Proposer != hs.ID() {
		hs.processCertificate(block.QC, block.View-1)
	}
	curView = hs.pm.GetCurView()
	if block.View < curView {
		log.Warningf("[%v] received a stale proposal from %v", hs.ID(), block.Proposer)
		return nil
	}
	if !hs.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	hs.bc.AddBlock(block)
	// process buffered QC
	qc, ok := hs.bufferedQCs[block.ID]
	if ok {
		hs.processCertificate(qc, block.View-1)
		delete(hs.bufferedQCs, block.ID)
	}

	shouldVote, err := hs.votingRule(block)
	if err != nil {
		log.Errorf("[%v] cannot decide whether to vote the block, %w", hs.ID(), err)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", hs.ID(), block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, hs.ID(), block.ID)
	// vote is sent to the next leader
	voteAggregator := hs.FindLeaderFor(block.View + 1)
	if voteAggregator == hs.ID() {
		log.Debugf("[%v] vote is sent to itself, id: %x", hs.ID(), vote.BlockID)
		hs.ProcessVote(vote)
	} else {
		log.Debugf("[%v] vote is sent to %v, id: %x", hs.ID(), voteAggregator, vote.BlockID)
		hs.Send(voteAggregator, vote)
	}
	b, ok := hs.bufferedBlocks[block.View]
	if ok {
		_ = hs.ProcessBlock(b)
		delete(hs.bufferedBlocks, block.View)
	}
	return nil
}

func (hs *HotStuff) ProcessVote(vote *blockchain.Vote) {
	// if hs.IsByz(){
	// 	return
	// }

	log.Debugf("[%v] is processing the vote, block id: %x", hs.ID(), vote.BlockID)
	if vote.Voter != hs.ID() {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Warningf("[%v] Error in verifying the signature in vote id: %x", hs.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with invalid signature. vote id: %x", hs.ID(), vote.BlockID)
			return
		}
	}
	isBuilt, qc := hs.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", hs.ID(), vote.BlockID)
		return
	}
	qc.Leader = hs.ID()
	// buffer the QC if the block has not been received
	_, err := hs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		hs.bufferedQCs[qc.BlockID] = qc
		return
	}
	hs.processCertificate(qc, vote.View)
}

func (hs *HotStuff) ProcessRemoteTmo(tmo *pacemaker.TMO) {
	log.Debugf("[%v] is processing tmo from %v", hs.ID(), tmo.NodeID)
	hs.processCertificate(tmo.HighQC, tmo.View-1)
	isBuilt, tc := hs.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		return
	}
	log.Debugf("[%v] a tc is built for view %v", hs.ID(), tc.View)
	hs.processTC(tc)
}

func (hs *HotStuff) ProcessLocalTmo(view types.View) {
	hs.pm.AdvanceView(view)
	tmo := &pacemaker.TMO{
		View:   view,
		NodeID: hs.ID(),
		HighQC: hs.GetHighQC(),
	}
	hs.Broadcast(tmo)
	hs.ProcessRemoteTmo(tmo)
}

func (hs *HotStuff) MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block {
	qc, forkNum, height := hs.forkChoice()

	block := blockchain.MakeBlock(view, qc, qc.BlockID, payload, hs.ID(), hs.IsByz(), forkNum, height)

	return block
}

func (hs *HotStuff) forkChoice() (*blockchain.QC, int, int) {
	// var choice *blockchain.QC
	if !hs.IsByz() || !config.GetConfig().ForkATK {
		parBlockID := hs.GetHighQC().BlockID
		parBlock, err := hs.bc.GetBlockByID(parBlockID)
		if err == nil {
			return hs.GetHighQC(), -1, parBlock.GetHeight() + 1
		} else {
			return hs.GetHighQC(), -1, hs.GetHeight() + 1
		}
	}
	return hs.forkRule()
}

func (hs *HotStuff) forkRule() (*blockchain.QC, int, int) {
	var flag int = 0
	var height int = 0
	var choice *blockchain.QC
	//罗要fork的block的parent block
	parBlockID := hs.GetHighQC().BlockID
	parBlock, err := hs.bc.GetBlockByID(parBlockID)
	//罗 要fork的block的grandparent block，-次fork两个block
	grandParBlock, err1 := hs.bc.GetParentBlock(parBlockID)
	if err != nil {
		log.Warningf("cannot get parent block of block id: %x: %w", parBlockID, err)
	}
	if parBlock.View < hs.preferredView || parBlock.GetMali() {
		flag = 0
		choice = hs.GetHighQC()
		height = hs.height + 1
		return choice, flag, height
	} else {
		flag = 1
		choice = parBlock.QC
		height = parBlock.GetHeight()
	}
	if grandParBlock.View >= hs.preferredView && err1 == nil && !grandParBlock.GetMali() {
		flag = 2
		choice = grandParBlock.QC
		height = grandParBlock.GetHeight()
	}
	// to simulate Tc's view
	// 如果advance的时候是block的就不会有问题了
	// choice.View = hs.pm.GetCurView() - 1
	return choice, flag, height
}

func (hs *HotStuff) processTC(tc *pacemaker.TC) {
	if tc.View < hs.pm.GetCurView() {
		return
	}
	hs.pm.AdvanceView(tc.View)
}

func (hs *HotStuff) GetHighQC() *blockchain.QC {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.highQC
}

func (hs *HotStuff) GetHeight() int {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.height
}

func (hs *HotStuff) GetChainStatus() string {
	chainGrowthRate := hs.bc.GetChainGrowth()
	blockIntervals := hs.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", hs.ID(), hs.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (hs *HotStuff) updateHighQC(qc *blockchain.QC) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if qc.View > hs.highQC.View {
		hs.highQC = qc
	}
}

// 做个尝试，将advance的时候的view修改为使用真实的view
func (hs *HotStuff) processCertificate(qc *blockchain.QC, view types.View) {
	log.Debugf("[%v] is processing a QC, block id: %x", hs.ID(), qc.BlockID)
	// if qc.View < hs.pm.GetCurView() {
	// 	return
	// }
	if qc.Leader != hs.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if quorumIsVerified == false {
			log.Warningf("[%v] received a quorum with invalid signatures", hs.ID())
			return
		}
	}
	err := hs.updatePreferredView(qc)
	if err != nil {
		hs.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, id: %x", hs.ID(), qc.View, qc.BlockID)
		return
	}
	hs.pm.AdvanceView(view)
	hs.updateHighQC(qc)
	if qc.View < 3 {
		return
	}
	// 如果要forked了，那就不可能在这里提交，直接跳过
	// if hs.IsByz() && config.GetConfig().ForkATK {
	// block, _ := hs.bc.GetBlockByID(qc.BlockID)
	// if !block.GetMali() {
	// 	return
	// }

	// }

	ok, block, _ := hs.commitRule(qc)
	if !ok {
		return
	}
	// forked blocks are found when pruning
	committedBlocks, forkedBlocks, err := hs.bc.CommitBlock(block.ID, hs.pm.GetCurView())
	if err != nil {
		log.Errorf("[%v] cannot commit blocks, %w", hs.ID(), err)
		return
	}
	var heightestBlock *blockchain.Block
	for _, cBlock := range committedBlocks {
		hs.committedBlocks <- cBlock
		if heightestBlock == nil || int(cBlock.View) > int(heightestBlock.View) {
			heightestBlock = cBlock
		}
	}
	if heightestBlock != nil {
		heightestBlock.CommitFromThis = true
	}
	for _, fBlock := range forkedBlocks {
		hs.forkedBlocks <- fBlock
	}
}

func (hs *HotStuff) votingRule(block *blockchain.Block) (bool, error) {
	// if block.View <= 2 {
	// 	return true, nil
	// }
	// parentBlock, err := hs.bc.GetParentBlock(block.ID)
	// if err != nil {
	// 	return false, fmt.Errorf("cannot vote for block: %w", err)
	// }
	// if (block.View <= hs.lastVotedView) || (parentBlock.View < hs.preferredView) {
	// 	return false, nil
	// }
	return true, nil
}

func (hs *HotStuff) commitRule(qc *blockchain.QC) (bool, *blockchain.Block, error) {
	parentBlock, err := hs.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	grandParentBlock, err := hs.bc.GetParentBlock(parentBlock.ID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	if ((grandParentBlock.View + 1) == parentBlock.View) && ((parentBlock.View + 1) == qc.View) {
		log.Debugf("three view numbers are: %v, %v, %v", grandParentBlock.View, parentBlock.View, qc.View)
		return true, grandParentBlock, nil
	}
	return false, nil, nil
}

func (hs *HotStuff) updatePreferredView(qc *blockchain.QC) error {
	if qc.View <= 2 {
		return nil
	}
	_, err := hs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	grandParentBlock, err := hs.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if grandParentBlock.View > hs.preferredView {
		hs.preferredView = grandParentBlock.View
	}
	return nil
}
