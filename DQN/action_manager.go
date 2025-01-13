package DQN

import (
	"sort"
	"sync"
	"time"
	"encoding/json"

	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/log"
"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/types"
	"github.com/gitferry/bamboo/election"
	
)

type GlobalState struct {
	election.Election


	globalDaley         [][]int
	role                map[int]int // 1 is leader, 0 is follower
	byzRatio            float64
	consensusStage      map[int]int
	voteRatio           map[int]float64
	blockGenerationRate float64
	blockCommitRate     float64
	forkRate            float64
	throughput          float64
	latency             float64
	lastCommittedBlock	int
	maliBlocks          int

	// util
	blockGenerated []QueueElement
	blockCommitted []QueueElement
	blockForked    []int
	networkControl *NetworkControl
	ForkedMap      map[int]int
	finishedView   map[int]int //0 表示未结束，1表示完成了但是没发送信息，2表示完成了并且发送了信息
}




type QueueElement struct {
	Time time.Time
}

var (
	globalState *GlobalState
	once        sync.Once
)

func GetGlobalState() *GlobalState {
	once.Do(func() {
		cfg := config.GetConfig()
		globalState = &GlobalState{
			globalDaley:         make([][]int, cfg.N()),
			role:                make(map[int]int),
			byzRatio:            float64(cfg.ByzNo) / float64(cfg.N()),
			consensusStage:      make(map[int]int),
			voteRatio:           make(map[int]float64),
			blockGenerationRate: 0,
			blockCommitRate:     0,
			blockGenerated:      make([]QueueElement, 0),
			blockCommitted:      make([]QueueElement,0),
			blockForked:         make([]int, 0),
			ForkedMap:          make(map[int]int),
			lastCommittedBlock:	 0,
			maliBlocks:           0,
			networkControl:      NewNetworkControl("127.0.0.1:23309"),
		}
		globalState.Election = election.NewRotation(config.GetConfig().N())
		globalState.init()
	})
	return globalState
}

func (gs *GlobalState) init() {
	n := config.GetConfig().N()
	gs.globalDaley = make([][]int, n)
	gs.finishedView = make(map[int]int)
	gs.finishedView[0] = 2
	for i := 0; i < n; i++ {
		gs.globalDaley[i] = make([]int, n)
		for j := 0; j < n; j++ {
			gs.globalDaley[i][j] = config.GetConfig().Delay / 2 // 设置默认值为 0
		}
	}
}

func (gs *GlobalState) UpdateRole(view int, role int) {
	gs.role[view] = role
}

func (gs *GlobalState) UpdateConsensusStage(view int, stage int) {
	if gs.consensusStage[view] >= stage {
		return
	}
	gs.consensusStage[view] = stage
}

func (gs *GlobalState) UpdateLastCommittedBlock(view int) {
	if gs.lastCommittedBlock >= view {
		return
	}
	gs.lastCommittedBlock = view
	
}

func (gs *GlobalState) GetMaliciousBlocks(currentView int) int {
	count := 0
	for i := gs.lastCommittedBlock; i <= currentView; i++ {
		if gs.FindLeaderFor(types.View(i)) <= identity.NewNodeID(config.GetConfig().ByzNo) {
			count++
		}
	}
	gs.maliBlocks = count
	return count
}

func (gs *GlobalState) UpdateVoteRatio(view int, ratio float64) {
	gs.voteRatio[view] = ratio
}

func (gs *GlobalState) UpdateGlobalDelay(sender int, receiver int, delay int) {
	gs.globalDaley[sender][receiver] = delay
}

func (gs *GlobalState) UpdateBlockGenerationRate(view int) {
	currentTime := time.Now()
	for len(gs.blockGenerated) > 1 && gs.blockGenerated[0].Time.Add(time.Second).Before(currentTime) {
		gs.blockGenerated = gs.blockGenerated[1:]
	}
	gs.blockGenerated = append(gs.blockGenerated, QueueElement{currentTime})
	gs.blockGenerationRate = float64(len(gs.blockGenerated)) // 每秒生成区块数量
}

func (gs *GlobalState) UpdateBlockCommitRate(view int) {
	currentTime := time.Now()
	for len(gs.blockCommitted) > 1 && gs.blockCommitted[0].Time.Add(time.Second).Before(currentTime) {
		gs.blockCommitted = gs.blockCommitted[1:]
	}
	gs.blockCommitted = append(gs.blockCommitted, QueueElement{currentTime})
	gs.blockCommitRate = float64(len(gs.blockCommitted))
	gs.UpdateForkRate(-1, view)
}

func (gs *GlobalState) UpdateForkRate(view int, currentView int) {
	if view != -1 {
		gs.blockForked = append(gs.blockForked, view)
		sort.Ints(gs.blockForked)
	}
	gs.ForkedMap[view] = 1
	
	for len(gs.blockForked) >= 1 && currentView-gs.blockForked[0] > 10 {
		gs.blockForked = gs.blockForked[1:]
	}
	gs.forkRate = float64(len(gs.blockForked)) / float64(10) //过去100个区块中，有多少个fork

}

func (gs *GlobalState) UpdateThroughput(throughput float64) {
	gs.throughput = throughput
}

func (gs *GlobalState) GetConsensusStage(view int) int {
	stage, exists := gs.consensusStage[view]
	if !exists {
			return 0
	}
	return stage
}
func (gs *GlobalState) UpdateLatency(latency float64) {
	gs.latency = latency
}

func (gs *GlobalState) GetForkedNum(view int) (int,int) {
	count := 0
	maliBlocks := 0
	if gs.ForkedMap[view - 1] == 1 {
		count++
		if gs.FindLeaderFor(types.View(view - 1)) <= identity.NewNodeID(config.GetConfig().ByzNo) {
			maliBlocks++
		}
	}else{
		return count, maliBlocks
	}
	if gs.ForkedMap[view - 2] == 1 {
		count++
		if gs.FindLeaderFor(types.View(view - 2)) <= identity.NewNodeID(config.GetConfig().ByzNo) {
			maliBlocks++
		}
	}
	return count, maliBlocks
}

func (gs *GlobalState) GetPreViewRole(view int) []int {
	preViewRole := make([]int, 3)
	if view <= 5 {
		return preViewRole
	}
	preViewRole[2] = gs.IsByzView(view)
	preViewRole[1] = gs.IsByzView(view - 1) 
	preViewRole[0] = gs.IsByzView(view - 2)

	return preViewRole
}

func (gs *GlobalState) IsByzView(view int) int {
	if  gs.FindLeaderFor(types.View(view)) <= identity.NewNodeID(config.GetConfig().ByzNo){
		return 1
	}
	return 0
}

func (gs *GlobalState) Print(view int) {
	// log.Debugf("---------------------------------")
	// log.Debugf("blockCommitRate: %v", gs.blockCommitRate)
	// log.Debugf("blockGenerationRate: %v", gs.blockGenerationRate)
	// log.Debugf("byzRatio: %v", gs.byzRatio)
	// log.Debugf("consensusStage: %v", gs.consensusStage[view])
	// log.Debugf("forkRate: %v", gs.forkRate)
	// log.Debugf("role: %v", gs.role[view])
	// log.Debugf("throughput: %v", gs.throughput)
	// // gs.networkControl.getAction()
	// log.Debugf("---------------------------------")
}

func  (gs *GlobalState) GetAction(ty int, view int) int {
	


	if view <= 5 {
		return 0
	}
	forkedBlocks,forkedMaliBlocks := gs.GetForkedNum(view)


	request := Request{
		RequestType: "request",
		NodeId:    1,
		PrimaryId: 2,
		Delays:    []float64{0.5, 0.5, 0.5, 0.5},
		Role: 1,
		PreViewRole: gs.GetPreViewRole(view),
		ByzRatio:  0.5,
		ConsensusStage: gs.GetConsensusStage(view),
		VoteRatio: gs.voteRatio[view],
		BlockGenerationRate: gs.blockGenerationRate,
		BlockCommitRate: gs.blockCommitRate,
		ForkRate: gs.forkRate,
		MaliBlocks: gs.GetMaliciousBlocks(view),
		LastCommittedBlock: gs.lastCommittedBlock,
		ForkNumber: forkedBlocks,
		ForkMaliNumber: forkedMaliBlocks,
		Throughput: gs.throughput,
		Latency: gs.latency,
		View: view,
	}

	jsonData, err := json.Marshal(request)

	if err != nil {
		log.Errorf("Failed to marshal request: %v\n", err)
		return 0
	}
	
	response := gs.networkControl.sendMessage(jsonData)


	if ty == 1{
		return response.Actions
	}
	if ty == 0{
		return response.Actions
	}
	return 0
}

func (gs *GlobalState) ViewDone(view int) {

	if (gs.finishedView[view - 1] != 2){
		return
	}
	gs.finishedView[view] = 2

	if view <= 5 {
		return 
	}
	forkedBlocks,forkedMaliBlocks := gs.GetForkedNum(view)
	request := Request{
		RequestType: "viewDone",
		NodeId:    1,
		PrimaryId: 2,
		Delays:    []float64{0.5, 0.5, 0.5, 0.5},
		Role: 1,
		PreViewRole: gs.GetPreViewRole(view),
		ByzRatio:  0.5,
		ConsensusStage: gs.GetConsensusStage(view),
		VoteRatio: gs.voteRatio[view],
		BlockGenerationRate: gs.blockGenerationRate,
		BlockCommitRate: gs.blockCommitRate,
		MaliBlocks: gs.GetMaliciousBlocks(view),
		LastCommittedBlock: view - gs.lastCommittedBlock,
		ForkRate: gs.forkRate,
		ForkNumber: forkedBlocks,
		ForkMaliNumber: forkedMaliBlocks,
		Throughput: gs.throughput,
		Latency: gs.latency,
		View: view,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Errorf("Failed to marshal request: %v\n", err)
		return 
	}
	log.Debugf(string(jsonData))
	gs.networkControl.sendMessage(jsonData)
	if gs.finishedView[view + 1] == 1 {
		gs.ViewDone(view + 1)
	}
}

func (gs *GlobalState) CommitView(view int) {
	gs.finishedView[view] = 1
	gs.ViewDone(view)

}


type Action struct {
}
