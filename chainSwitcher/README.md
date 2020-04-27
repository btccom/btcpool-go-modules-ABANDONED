# Chain Switcher

根据HTTP API提供的币种价格信息，发送币种切换命令到kafka

## HTTP接口

接口应返回如下JSON：

```
{
    "algorithms": {
        "SHA256": {
            "coins": [
                "BCH",
                "BTC"
            ]
        }
    }
}
```

其中：`coins` 为推荐挖掘的币种，按收益从高到低排序。

## 构建
```
go get github.com/segmentio/kafka-go
go get github.com/golang/snappy
go get github.com/go-sql-driver/mysql
go get github.com/golang/glog
go build
```

## 运行
```
cp config.default.json config.json
./chainSwitcher --config config.json --logtostderr
```

# Docker

## 构建
```
cd btcpool-go-modules/chainSwitcher
docker build -t btcpool-chain-switcher -f Dockerfile ..
```

## 运行
```
docker run -it --rm --network=host \
    -e KafkaBrokers=127.0.0.1:9092,127.0.0.2:9092,127.0.0.3:9092 \
    -e KafkaControllerTopic=BtcManController \
    -e KafkaProcessorTopic=BtcManProcessor \
    -e Algorithm=SHA256 \
    -e ChainDispatchAPI=http://127.0.0.1:8000/chain-dispatch.php \
    -e FailSafeChain=btc \
    -e ChainNameMap='{"BTC":"btc","BCH":"bcc"}' \
    -e MySQLConnStr="root:root@/bpool_local_db" \
    btcpool-chain-switcher -logtostderr -v 2

# 全部参数：
docker run -it --rm --network=host \
    -e KafkaBrokers=127.0.0.1:9092,127.0.0.2:9092,127.0.0.3:9092 \
    -e KafkaControllerTopic=BtcManController \
    -e KafkaProcessorTopic=BtcManProcessor \
    -e Algorithm=SHA256 \
    -e ChainDispatchAPI=http://127.0.0.1:8000/chain-dispatch.php \
    -e SwitchIntervalSeconds=60 \
    -e FailSafeChain=btc \
    -e FailSafeSeconds=600 \
    -e ChainNameMap='{"BTC":"btc","BCH":"bcc","BSV":"bsv"}' \
    -e MySQLConnStr="root:root@tcp(localhost:3306)/bpool_local_db" \
    -e MySQLTable="chain_switcher_record" \
    \
    -e ChainLimits_bcc_MaxHashrate="100P" \
    -e ChainLimits_bcc_MySQLConnStr="user:password@tcp(localhost:3306)/bcc_local_db" \
    -e ChainLimits_bcc_MySQLTable="mining_workers" \
    \
    -e ChainLimits_bsv_MaxHashrate="100P" \
    -e ChainLimits_bsv_MySQLConnStr="user:password@tcp(localhost:3306)/bsv_local_db" \
    -e ChainLimits_bsv_MySQLTable="mining_workers" \
    \
    -e RecordLifetime="60" \
    btcpool-chain-switcher -logtostderr -v 2

# 守护进程
docker run -it --name chain-switcher --network=host --restart always -d \
    -e KafkaBrokers=127.0.0.1:9092,127.0.0.2:9092,127.0.0.3:9092 \
    -e KafkaControllerTopic=BtcManController \
    -e KafkaProcessorTopic=BtcManProcessor \
    -e Algorithm=SHA256 \
    -e ChainDispatchAPI=http://127.0.0.1:8000/chain-dispatch.php \
    -e FailSafeChain=btc \
    -e ChainNameMap='{"BTC":"btc","BCH":"bcc"}' \
    -e MySQLConnStr="root:root@/bpool_local_db" \
    btcpool-chain-switcher -logtostderr -v 2
```

## 数据库变更
程序会自动尝试创建如下数据表：
```
CREATE TABLE IF NOT EXISTS `<configData.MySQL.Table的值>`(
    id bigint(20) NOT NULL AUTO_INCREMENT,
    algorithm varchar(255) NOT NULL,
    prev_chain varchar(255) NOT NULL,
    curr_chain varchar(255) NOT NULL,
    api_result text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
)
```
