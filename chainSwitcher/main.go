package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/snappy"

	_ "github.com/segmentio/kafka-go/snappy"
)

// ChainSwitcherConfig 程序配置
type ChainSwitcherConfig struct {
	Kafka struct {
		Brokers         []string
		ControllerTopic string
		ProcessorTopic  string
	}
	ChainDispatchAPI      string
	SwitchIntervalSeconds time.Duration
	ChainNameMap          map[string]string
}

// ChainRecord HTTP API中的币种记录
type ChainRecord struct {
	DispatchHashrate     float64 `json:"dispatch_hashrate"`
	DispatchableHashrate float64 `json:"dispatchable_hashrate"`
}

// ChainDispatchRecord HTTP API响应
type ChainDispatchRecord struct {
	Coins map[string]ChainRecord
}

// KafkaMessage Kafka中接收的消息结构
type KafkaMessage struct {
	ID                  interface{} `json:"id"`
	Type                string      `json:"type"`
	Action              string      `json:"action"`
	CreatedAt           string      `json:"created_at"`
	NewChainName        string      `json:"new_chain_name"`
	OldChainName        string      `json:"old_chain_name"`
	Result              bool        `json:"result"`
	ServerID            int         `json:"server_id"`
	SwitchedConnections int         `json:"switched_connections"`
	SwitchedUsers       int         `json:"switched_users"`
}

// KafkaCommand Kafka中发送的消息结构
type KafkaCommand struct {
	ID        interface{} `json:"id"`
	Type      string      `json:"type"`
	Action    string      `json:"action"`
	CreatedAt string      `json:"created_at"`
	ChainName string      `json:"chain_name"`
}

// 配置数据
var configData *ChainSwitcherConfig

var currentChainName string

var controllerProducer *kafka.Writer
var processorConsumer *kafka.Reader
var commandID uint64

func main() {
	// 解析命令行参数
	configFilePath := flag.String("config", "./config.json", "Path of config file")
	flag.Parse()

	// 读取配置文件
	configJSON, err := ioutil.ReadFile(*configFilePath)

	if err != nil {
		glog.Fatal("read config failed: ", err)
		return
	}

	configData = new(ChainSwitcherConfig)
	err = json.Unmarshal(configJSON, configData)

	if err != nil {
		glog.Fatal("parse config failed: ", err)
		return
	}

	processorConsumer = kafka.NewReader(kafka.ReaderConfig{
		Brokers:   configData.Kafka.Brokers,
		Topic:     configData.Kafka.ProcessorTopic,
		Partition: 0,
		MinBytes:  128,  // 128B
		MaxBytes:  10e6, // 10MB
	})

	controllerProducer = kafka.NewWriter(kafka.WriterConfig{
		Brokers:          configData.Kafka.Brokers,
		Topic:            configData.Kafka.ControllerTopic,
		Balancer:         &kafka.LeastBytes{},
		CompressionCodec: snappy.NewCompressionCodec(),
	})

	go readResponse()
	run()
}

func run() {
	for {
		updateCurrentChain()

		if currentChainName != "" {
			commandID++
			command := KafkaCommand{
				commandID,
				"sserver_cmd",
				"auto_switch_chain",
				time.Now().UTC().Format("2006-01-02 15:04:05"),
				currentChainName}
			bytes, _ := json.Marshal(command)
			controllerProducer.WriteMessages(context.Background(), kafka.Message{Value: []byte(bytes)})
			glog.Info("Send to Kafka, id: ", command.ID,
				", created_at: ", command.CreatedAt,
				", type: ", command.Type,
				", action: ", command.Action,
				", chain_name: ", command.ChainName)
		}

		// 休眠
		time.Sleep(configData.SwitchIntervalSeconds * time.Second)
	}
}

func updateCurrentChain() {
	glog.Info("HTTP GET ", configData.ChainDispatchAPI)
	response, err := http.Get(configData.ChainDispatchAPI)
	if err != nil {
		glog.Error("HTTP Request Failed: ", err)
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		glog.Error("HTTP Fetch Body Failed: ", err)
		return
	}

	chainDispatchRecord := new(ChainDispatchRecord)
	err = json.Unmarshal(body, chainDispatchRecord)
	if err != nil {
		glog.Error("Parse Result Failed: ", err)
		return
	}

	var largeHashrate float64
	var bestChain string
	for chain, record := range chainDispatchRecord.Coins {
		if record.DispatchHashrate > largeHashrate {
			chainName, ok := configData.ChainNameMap[chain]
			if ok {
				largeHashrate = record.DispatchHashrate
				bestChain = chainName
			}
		}
	}

	if bestChain != "" {
		currentChainName = bestChain
	}
	glog.Info("Current Best Chain: ", bestChain)
}

func readResponse() {
	processorConsumer.SetOffset(kafka.LastOffset)
	for {
		m, err := processorConsumer.ReadMessage(context.Background())
		if err != nil {
			glog.Error("read kafka failed: ", err)
			continue
		}
		response := new(KafkaMessage)
		err = json.Unmarshal(m.Value, response)
		if err != nil {
			glog.Error("Parse Result Failed: ", err)
			continue
		}

		if response.Type == "sserver_response" && response.Action == "auto_switch_chain" {
			glog.Info("Server Response, id: ", response.ID,
				", created_at: ", response.CreatedAt,
				", server_id: ", response.ServerID,
				", result: ", response.Result,
				", old_chain_name: ", response.OldChainName,
				", new_chain_name: ", response.NewChainName,
				", switched_users: ", response.SwitchedUsers,
				", switched_connections: ", response.SwitchedConnections)
		}
	}
}
