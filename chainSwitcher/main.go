package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/snappy"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/segmentio/kafka-go/snappy"
)

// ChainSwitcherConfig 程序配置
type ChainSwitcherConfig struct {
	Kafka struct {
		Brokers         []string
		ControllerTopic string
		ProcessorTopic  string
	}
	Algorithm             string
	ChainDispatchAPI      string
	SwitchIntervalSeconds time.Duration
	FailSafeChain         string
	FailSafeSeconds       time.Duration
	ChainNameMap          map[string]string
	MySQL                 struct {
		ConnStr string
		Table   string
	}
}

// ChainRecord HTTP API中的币种记录
type ChainRecord struct {
	Coins []string `json:"coins"`
}

// ChainDispatchRecord HTTP API响应
type ChainDispatchRecord struct {
	Algorithms map[string]ChainRecord `json:"algorithms"`
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
	Host                struct {
		Hostname string              `json:"hostname"`
		IP       map[string][]string `json:"ip"`
	} `json:"host"`
}

// KafkaCommand Kafka中发送的消息结构
type KafkaCommand struct {
	ID        interface{} `json:"id"`
	Type      string      `json:"type"`
	Action    string      `json:"action"`
	CreatedAt string      `json:"created_at"`
	ChainName string      `json:"chain_name"`
}

// ActionFailSafeSwitch API失效切换到默认币种时记录的api_result
type ActionFailSafeSwitch struct {
	Action         string `json:"action"`
	LastUpdateTime int64  `json:"last_update_time"`
	CurrentTime    int64  `json:"current_time"`
	OldChainName   string `json:"old_chain_name"`
	NewChainName   string `json:"new_chain_name"`
}

// 配置数据
var configData *ChainSwitcherConfig

var updateTime int64
var currentChainName string

var controllerProducer *kafka.Writer
var processorConsumer *kafka.Reader
var commandID uint64

var insertStmt *sql.Stmt
var mysqlConn *sql.DB

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

	initMySQL()
	go failSafe()
	go readResponse()
	updateChain()
}

func initMySQL() {
	var err error

	glog.Info("connecting to MySQL...")
	mysqlConn, err = sql.Open("mysql", configData.MySQL.ConnStr)
	if err != nil {
		glog.Fatal("mysql error: ", err)
	}

	err = mysqlConn.Ping()
	if err != nil {
		glog.Fatal("mysql error: ", err.Error())
	}

	mysqlConn.Exec("CREATE TABLE IF NOT EXISTS `" + configData.MySQL.Table + "`(" + `
		id bigint(20) NOT NULL AUTO_INCREMENT,
		algorithm varchar(255) NOT NULL,
		prev_chain varchar(255) NOT NULL,
		curr_chain varchar(255) NOT NULL,
		api_result text NOT NULL,
		created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id)
		)
	`)

	insertStmt, err = mysqlConn.Prepare("INSERT INTO `" + configData.MySQL.Table +
		"`(algorithm,prev_chain,curr_chain,api_result) VALUES(?,?,?,?)")
	if err != nil {
		glog.Fatal("mysql error: ", err.Error())
	}
}

func failSafe() {
	for {
		time.Sleep(configData.FailSafeSeconds * time.Second)

		now := time.Now().Unix()
		if updateTime+int64(configData.FailSafeSeconds) < now {
			oldChainName := currentChainName
			currentChainName = configData.FailSafeChain

			glog.Info("Fail Safe Switch: ", oldChainName, " -> ", currentChainName,
				", lastUpdateTime: ", time.Unix(updateTime, 0).UTC().Format("2006-01-02 15:04:05"),
				", currentTime: ", time.Unix(now, 0).UTC().Format("2006-01-02 15:04:05"))
			sendCurrentChainToKafka()

			apiResult := ActionFailSafeSwitch{
				"fail_safe_switch",
				updateTime,
				now,
				oldChainName,
				currentChainName}
			bytes, _ := json.Marshal(apiResult)
			_, err := insertStmt.Exec(configData.Algorithm, oldChainName, currentChainName, bytes)
			if err != nil {
				glog.Fatal("mysql error: ", err.Error())
			}

			updateTime = now
		}
	}
}

func sendCurrentChainToKafka() {
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

func updateChain() {
	for {
		updateCurrentChain()
		if currentChainName != "" {
			sendCurrentChainToKafka()
		}

		time.Sleep(configData.SwitchIntervalSeconds * time.Second)
	}
}

func updateCurrentChain() {
	oldChainName := currentChainName

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

	algorithms, ok := chainDispatchRecord.Algorithms[configData.Algorithm]
	if !ok {
		glog.Error("Cannot find algorithm ", configData.Algorithm, ", json: ", string(body))
		return
	}

	var bestChain string
	for _, coin := range algorithms.Coins {
		chainName, ok := configData.ChainNameMap[coin]
		if ok {
			bestChain = chainName
			break
		}
	}

	if bestChain != "" {
		currentChainName = bestChain
		updateTime = time.Now().Unix()
	}

	if oldChainName != currentChainName {
		glog.Info("Best Chain Changed: ", oldChainName, " -> ", bestChain)
		_, err := insertStmt.Exec(configData.Algorithm, oldChainName, currentChainName, body)
		if err != nil {
			glog.Fatal("mysql error: ", err.Error())
		}
	} else {
		glog.Info("Best Chain not Changed: ", bestChain)
	}
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
			continue
		}

		if response.Type == "sserver_notify" && response.Action == "online" {
			glog.Info("Server Online, ",
				", created_at: ", response.CreatedAt,
				", server_id: ", response.ServerID,
				", hostname: ", response.Host.Hostname,
				", ip: ", response.Host.IP)
			sendCurrentChainToKafka()
			continue
		}
	}
}
