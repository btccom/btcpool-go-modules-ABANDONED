# Chain Switcher

根据HTTP API提供的币种价格信息，发送币种切换命令到kafka

## HTTP接口

接口应返回如下JSON：

```
{
    "coins": {
        "BCH": {
            "dispatch_hashrate": 258111.92670242672,
            "dispatchable_hashrate": 10000000
        },
        "BSV": {
            "dispatch_hashrate": 132395.62447732966,
            "dispatchable_hashrate": 10000000
        },
        "BTC": {
            "dispatch_hashrate": 9609492.448820245,
            "dispatchable_hashrate": 10000000
        }
    }
}
```

其中：
* `dispatchable_hashrate`: 参与分配的总算力
* `dispatch_hashrate`: 每个币种上推荐分配的算力

在设计接口时，`dispatchable_hashrate` 可设计为通过URL查询字符串接收，或者直接设置为固定值。目前BTCPool不支持按比例分配算力，所以切换器只会选取`dispatch_hashrate`最高的币种做为切换目标。

## 构建
```
go get github.com/segmentio/kafka-go
go get github.com/golang/snappy
go build
```

## 运行
```
./chainSwitcher --config config.default.json --logtostderr
```
