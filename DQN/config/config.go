// config.go
package __

import (
	"fmt"
	"encoding/json"
	"io/ioutil"
)

// Config 结构体对应 client_config.json 文件的内容
type Config struct {
	Node_id int         `json:"node_id"`
	Center_ip       string            `json:"center_ip"`
	Port      int  `json:"port"`
	Local_port   int `json:"local_port"`
}


func LoadConfig(filename string) (*Config, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 数据
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func A(){
	fmt.Println("A")
}