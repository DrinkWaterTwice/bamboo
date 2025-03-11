// client.go
package DQN

import (
	"context"
	"time"
	"strconv"
	"sync"
	
"github.com/gitferry/bamboo/log"
	"google.golang.org/grpc"
   pb "github.com/gitferry/bamboo/DQN/protobuf" 
	 con  "github.com/gitferry/bamboo/DQN/config"
)


type Client struct {
	conn *grpc.ClientConn
	client pb.GreeterClient
	actionInfoBuffer []*pb.ActionInfo
	config *con.Config
}



var (
	instance *Client
	mu       sync.Mutex
)

func NewClient() (*Client, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance == nil {
		config, err := con.LoadConfig("../../../ControllerClient/client_config.json")
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		port := config.Local_port
		address := "127.0.0.1" + ":" + strconv.Itoa(port)
		log.Debugf("connect to "+address)
		conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		actionInfoBuffer := make([]*pb.ActionInfo, 0)
		client := pb.NewGreeterClient(conn)

		instance = &Client{
			conn:           conn,
			client:         client,
			actionInfoBuffer: actionInfoBuffer,
			config:         config,
		}
	}

	return instance, nil
}


func (c *Client) reconnect() {
	config := c.config
	// 重新连接
	port := config.Local_port
	address := "127.0.0.1" + ":" + strconv.Itoa(port)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	client := pb.NewGreeterClient(conn)
	c.conn = conn
	c.client= client
}


func(c *Client) Init(){
	go c.sendActionInfo()
}


func (c *Client) sendActionInfo() {
	ticker := time.NewTicker(1 * time.Second) // 每5秒发送一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if len(c.actionInfoBuffer) > 0 {
				log.Debugf("Sending buffered action info")
				
				err := c.FinishAction(c.actionInfoBuffer)
				if err != nil {
					log.Debugf("Failed to send action info: %v", err)
					c.reconnect()
				}
				c.actionInfoBuffer = c.actionInfoBuffer[:0] // 清空缓冲区
			} else {
				log.Debugf("No action info to send")
			}
		}
	}
}

func(c *Client) MaliciousActionInfo(reqs []*pb.Request) (*pb.MaliciousAction){
	stream, err := c.client.MaliciousActionInfo(context.Background())
	if err != nil {
		log.Debugf("could not create stream: %v", err)
	}

	for _, req := range reqs {
		if err := stream.Send(req); err != nil {
			log.Debugf("could not send request: %v", err)
		}
	}

	if err := stream.CloseSend(); err != nil {
		log.Debugf("could not close send stream: %v", err)
	}

		// 接收响应		
	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Debugf("could not receive reply: %v", err)
	}
	
	log.Debugf("Received mali actions: %v", reply.Delay)
	return reply
}

func(c *Client) FinishAction(actionInfo []*pb.ActionInfo) (error){
	stream, err := c.client.FinishAction(context.Background())
	if err != nil {
		log.Debugf("could not create stream: %v", err)
	}
	for _, req := range actionInfo {
		if err := stream.Send(req); err != nil {
			log.Debugf("could not send request: %v", err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		log.Debugf("could not close send stream: %v", err)
	}
	reply, err := stream.CloseAndRecv()

	

	log.Debugf("Received reply: ", reply)
	return err
	// for {
	// 	reply, err := stream.Recv()
	// 	if err != nil {
	// 		log.Fatalf("could not receive reply: %v", err)
	// 	}
	// 	log.Printf("Received reply: action=%s, parameter=%s", reply.Action, reply.Parameter)
	// }
}




func(c *Client) MonitoringData(monitoringData []*pb.Monitoring){}

func(c *Client) Proposal(id int32,view int32, role string){
	
	if c == nil{
		log.Debugf("Client is nil")
	}

	info := &pb.ActionInfo{
		Id: id, Type: "send_message", Time: time.Now().UnixNano() / int64(time.Second), Data: &pb.Data{
			Id: id,
			Role: role,
			View: view,
			Phase: "Propose",
			Action: &pb.Action{
				ActionId: 1,
				ActionParameter: "actionParameter1",
			},
		}}
		log.Debugf("Send proposal")
		c.actionInfoBuffer = append(c.actionInfoBuffer,info)
}

func(c *Client) SendBlock(id int32,view int32, role string, parameter string){
	info :=
	&pb.ActionInfo{
		Id: id, Type: "send_message", Time: time.Now().UnixNano() / int64(time.Second), Data: &pb.Data{
			Id: id,
			Role: role,
			View: view,
			Phase: "BrocastBlock",
			Action: &pb.Action{
				ActionId: 2,
				ActionParameter: parameter,
			},
		}}
		c.actionInfoBuffer = append(c.actionInfoBuffer,info)
}

func(c *Client) SendVote(id int32,view int32, role string, parameter string){
	info :=
	&pb.ActionInfo{
		Id: id, Type: "send_message", Time: time.Now().UnixNano() / int64(time.Second), Data: &pb.Data{
			Id: id,
			Role: role,
			View: view,
			Phase: "Vote",
			Action: &pb.Action{
				ActionId: 3,
				ActionParameter: parameter,
			},
		}}
		c.actionInfoBuffer = append(c.actionInfoBuffer,info)
}

func(c *Client) Commit(id int32,view int32, role string){
	info :=
	&pb.ActionInfo{
		Id: id, Type: "send_message", Time: time.Now().UnixNano() / int64(time.Second), Data: &pb.Data{
			Id: id,
			Role: role,
			View: view,
			Phase: "Commit",
			Action: &pb.Action{
				ActionId: 4,
				ActionParameter: "actionParameter1",
			},
		}}
		c.actionInfoBuffer = append(c.actionInfoBuffer,info)
}

