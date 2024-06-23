package blockchain

import (
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/types"
)

type Block struct {
	types.View
	QC             *QC
	Proposer       identity.NodeID
	Timestamp      time.Time
	Payload        []*message.Transaction
	PrevID         crypto.Identifier
	Sig            crypto.Signature
	ID             crypto.Identifier
	Ts             time.Duration
	Mali           bool
	forkNum        int
	height         int
	CommitFromThis bool
}

type rawBlock struct {
	types.View
	QC       *QC
	Proposer identity.NodeID
	Payload  []string
	PrevID   crypto.Identifier
	Sig      crypto.Signature
	ID       crypto.Identifier
}

// MakeBlock creates an unsigned block
func MakeBlock(view types.View, qc *QC, prevID crypto.Identifier, payload []*message.Transaction, proposer identity.NodeID, mali bool, forkNum int, height int) *Block {
	b := new(Block)
	b.View = view
	b.Proposer = proposer
	b.QC = qc
	b.Payload = payload
	b.PrevID = prevID
	b.Mali = mali
	b.forkNum = forkNum
	b.height = height
	b.makeID(proposer)
	b.CommitFromThis = false
	return b
}

func (b *Block) makeID(nodeID identity.NodeID) {
	raw := &rawBlock{
		View:     b.View,
		QC:       b.QC,
		Proposer: b.Proposer,
		PrevID:   b.PrevID,
	}
	var payloadIDs []string
	for _, txn := range b.Payload {
		payloadIDs = append(payloadIDs, txn.ID)
	}
	raw.Payload = payloadIDs
	b.ID = crypto.MakeID(raw)
	// TODO: uncomment the following
	b.Sig, _ = crypto.PrivSign(crypto.IDToByte(b.ID), nodeID, nil)
}

func (b *Block) SetMali(mali bool) {
	b.Mali = mali
}

func (b *Block) GetMali() bool {
	return b.Mali
}

func (b *Block) SetForkNum(forkNum int) {
	b.forkNum = forkNum
}

func (b *Block) GetForkNum() int {
	return b.forkNum
}

func (b *Block) SetHeight(height int) {
	b.height = height
}

func (b *Block) GetHeight() int {
	return b.height
}
