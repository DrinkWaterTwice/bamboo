package bcb

import (
	"sync"

	"github.com/gitferry/zeitgeber/log"

	"github.com/gitferry/zeitgeber"
)

type bcb struct {
	zeitgeber.Node
	zeitgeber.Election

	curView  zeitgeber.View
	quorum   *zeitgeber.Quorum
	highCert *zeitgeber.TC

	newViewChan chan zeitgeber.View

	mu sync.Mutex
}

func NewBcb(n zeitgeber.Node, election zeitgeber.Election) zeitgeber.Synchronizer {
	bcb := new(bcb)
	bcb.Node = n
	bcb.Election = election
	bcb.newViewChan = make(chan zeitgeber.View)
	bcb.quorum = zeitgeber.NewQuorum()
	bcb.Register(zeitgeber.TCMsg{}, bcb.HandleTC)
	bcb.Register(zeitgeber.TmoMsg{}, bcb.HandleTmo)
	bcb.highCert = zeitgeber.NewTC(0)
	return bcb
}

func (b *bcb) HandleTmo(tmo zeitgeber.TmoMsg) {
	b.mu.Lock()
	if tmo.View < b.curView {
		log.Warningf("[%s] received timeout msg with view %d lower thant the current view %d", b.ID(), tmo.View, b.curView)
		b.mu.Unlock()
		return
	}
	// store the wish
	b.quorum.ACK(tmo.View, tmo.NodeID)
	if !b.quorum.SuperMajority(tmo.View) {
		return
	}
	// TC is generated
	b.mu.Unlock()
	b.Send(b.FindLeaderFor(tmo.View), zeitgeber.TCMsg{View: tmo.View})
	log.Infof("[%s] a time certificate for view %d is sent", b.ID(), tmo.View)
	b.NewView(tmo.View)
}

func (b *bcb) HandleTC(tc zeitgeber.TCMsg) {
	log.Infof("[%v] received tc from %v for view %v", b.ID(), tc.NodeID, tc.View)
	b.mu.Lock()
	if tc.View < b.curView {
		log.Warningf("[%s] received tc's view %v is lower than current view %v", b.ID(), tc.View, b.curView)
		b.mu.Unlock()
		return
	}
	if tc.View > b.highCert.View {
		b.highCert = zeitgeber.NewTC(tc.View)
	}
	b.mu.Unlock()
	b.NewView(tc.View)
}

// TimeoutFor broadcasts the timeout msg for the view when it timeouts
func (b *bcb) TimeoutFor(view zeitgeber.View) {
	tmoMsg := zeitgeber.TmoMsg{
		View:   view,
		NodeID: b.ID(),
	}
	log.Debugf("[%s] is timeout for view %v", b.ID(), view)
	if b.IsByz() {
		b.MulticastQuorum(zeitgeber.GetConfig().ByzNo, tmoMsg)
		return
	}
	b.Broadcast(tmoMsg)
}

func (b *bcb) NewView(view zeitgeber.View) {
	b.mu.Lock()
	if view < b.curView {
		log.Warningf("the view %v is lower than current view %v", view, b.curView)
		b.mu.Unlock()
		return
	}
	b.curView = view + 1
	b.mu.Unlock()
	b.newViewChan <- view + 1 // reset timer for the next view
}

func (b *bcb) EnteringViewEvent() chan zeitgeber.View {
	return b.newViewChan
}

func (b *bcb) GetCurView() zeitgeber.View {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.curView
}

func (b *bcb) GetHighCert() *zeitgeber.TC {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.highCert
}