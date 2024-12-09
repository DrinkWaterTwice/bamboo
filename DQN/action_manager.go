package DQN

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gitferry/bamboo/config"
)

type GlobalState struct {
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

	// util
	blockGenerated []QueueElement
	blockCommitted []QueueElement
	blockForked    []int
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
			blockGenerated:      make([]QueueElement, 10000),
			blockCommitted:      make([]QueueElement, 10000),
			blockForked:         make([]int, 10000),
		}
		globalState.init()
	})
	return globalState
}

func (gs *GlobalState) init() {
	n := config.GetConfig().N()
	gs.globalDaley = make([][]int, n)
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
	if gs.consensusStage[view] <= stage {
		return
	}
	gs.consensusStage[view] = stage
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
}

func (gs *GlobalState) UpdateForkRate(view int, currentView int) {
	gs.blockForked = append(gs.blockForked, view)
	sort.Ints(gs.blockForked)
	for len(gs.blockForked) >= 1 && currentView-gs.blockForked[0] > 1 {
		gs.blockForked = gs.blockForked[1:]
	}
	gs.forkRate = float64(len(gs.blockForked)) / 100 //过去100个区块中，有多少个fork

}

func (gs *GlobalState) UpdateThroughput(throughput float64) {
	gs.throughput = throughput
}

func (gs *GlobalState) UpdateLatency(latency float64) {
	gs.latency = latency
}

func (gs *GlobalState) Print() {
	fmt.Println("blockCommitRate: ", gs.blockCommitRate)
	fmt.Println("blockGenerationRate: ", gs.blockGenerationRate)
}

type Action struct {
}
