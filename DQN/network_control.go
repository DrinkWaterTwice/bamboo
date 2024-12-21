package DQN

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type NetworkControl struct {
	serverAddr string
}

// Response 结构体定义了从服务器接收的数据格式
type Response struct {
	Actions int `json:"actions"`
}

type Request struct {
	RequestType string  `json:"requestType"`
	NodeId    int     `json:"nodeId"`
	PrimaryId int     `json:"primaryId"`
	View 			int 		`json:"view"`
	Reward    float64 `json:"reward"`
	Delays    []float64 `json:"delays"`
	Role 			int `json:"role"`
	PreViewRole []int `json:"preViewRole"`
	ByzRatio  float64 `json:"byzRatio"`
	ConsensusStage int `json:"consensusStage"`
	VoteRatio float64 `json:"voteRatio"`
	BlockGenerationRate float64 `json:"blockGenerationRate"`
	BlockCommitRate float64 `json:"blockCommitRate"`
	ForkRate float64 `json:"forkRate"`
	LastCommittedBlock int `json:"lastCommittedBlock"`
	MaliBlocks int `json:"maliBlocks"` 
	Throughput float64 `json:"throughput"`
	Latency float64 `json:"latency"`
	ForkNumber int `json:"forkNumber"`
	ForkMaliNumber int `json:"forkedMaliNumber"`
}

func NewNetworkControl(serverAddr string) *NetworkControl {
	return &NetworkControl{
		serverAddr: serverAddr,
	}
}

func (nc NetworkControl) sendMessage(request []byte) Response{
	// 服务器地址和端口
	serverAddr := nc.serverAddr
	var response Response
	// 创建 TCP 连接
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()


	// 发送 JSON 数据到服务器
	_, err = conn.Write(request)
	if err != nil {
		fmt.Printf("Failed to send data to server: %v\n", err)
		return response
	}

	// 读取服务器响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read response from server: %v\n", err)
		return response
	}
	err = json.Unmarshal(buffer[:n], &response)
	if err != nil {
		fmt.Printf("Failed to unmarshal response: %v\n", err)
		return response
	}

	// 打印响应数据
	fmt.Printf("Received response from server: %+v\n", response)
	return response
}

