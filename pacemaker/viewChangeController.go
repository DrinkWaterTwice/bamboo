package pacemaker

import (
	"sync"

	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/types"
)

type ViewChangeController struct {
	n        int                                     // the size of the network
	timeouts map[types.View]map[identity.NodeID]*VMO // keeps track of timeout msgs
	mu       sync.Mutex
}

func NewViewChangeController(n int) *ViewChangeController {
	tcl := new(ViewChangeController)
	tcl.n = n
	tcl.timeouts = make(map[types.View]map[identity.NodeID]*VMO)
	return tcl
}

func (vcl *ViewChangeController) AddVmo(vmo *VMO) (bool, *VC) {
	vcl.mu.Lock()
	defer vcl.mu.Unlock()
	if vcl.superMajority(vmo.View) {
		return false, nil
	}
	_, exist := vcl.timeouts[vmo.View]
	if !exist {
		//	first time of receiving the timeout for this view
		vcl.timeouts[vmo.View] = make(map[identity.NodeID]*VMO)
	}
	vcl.timeouts[vmo.View][vmo.NodeID] = vmo
	if vcl.superMajority(vmo.View) {
		return true, NewVC(vmo.View, vcl.timeouts[vmo.View])
	}

	return false, nil
}

func (vcl *ViewChangeController) superMajority(view types.View) bool {
	return vcl.total(view) > vcl.n*2/3
}

func (vcl *ViewChangeController) total(view types.View) int {
	return len(vcl.timeouts[view])
}
