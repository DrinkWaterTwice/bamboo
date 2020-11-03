package replica

import (
	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

type Safety interface {
	ProcessBlock(block *blockchain.Block) error
	ProcessVote(vote *blockchain.Vote)
	ProcessRemoteTmo(tmo *pacemaker.TMO)
	ProcessLocalTmo(view types.View)
	ProcessCertificate(qc *blockchain.QC)
	MakeProposal(payload []*message.Transaction) *blockchain.Block
}
