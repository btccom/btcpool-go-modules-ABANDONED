# Merged Mining Proxy

一个比特币联合挖矿代理，用于同时挖掘多个符合[比特币联合挖矿标准](https://en.bitcoin.it/wiki/Merged_mining_specification)的币种。

可用于在同一个比特币矿池中同时挖掘域名币（Namecoin）、亦来云（Elastos）等。

### 构建 & 运行

#### 安装golang

```bash
mkdir ~/source
cd ~/source
wget http://storage.googleapis.com/golang/go1.10.3.linux-amd64.tar.gz
cd /usr/local
tar zxf ~/source/go1.10.3.linux-amd64.tar.gz
ln -s /usr/local/go/bin/go /usr/local/bin/go
```

#### 构建

```bash
mkdir -p /work/golang
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/btcpool-go-modules/mergedMiningProxy
```

#### 编辑配置文件

```bash
mkdir /work/golang/mergedMiningProxy
mkdir /work/golang/mergedMiningProxy/log
cp /work/golang/src/github.com/btccom/btcpool-go-modules/mergedMiningProxy/config.default.json /work/golang/mergedMiningProxy/config.json
vim /work/golang/mergedMiningProxy/config.json
```

##### 配置文件详解：

备注：JSON文件不支持注释，如果您想拷贝以下配置文件，请先**删除所有注释**。
```js
{
    "RPCServer": {
        "ListenAddr": "0.0.0.0:8999", // 监听IP和端口
        "User": "admin",  // Basic认证用户名
        "Passwd": "admin", // Basic认证密码
        "MainChain":"BTC",  // 指定联合挖矿的主链类型，如：bitcoin => "BTC", litecoin => "LTC"
        "PoolDb": {
            "host" : "127.0.0.1",
            "port" : 3306,
            "username" : "root",
            "password" : "root",
            "dbname" : "bpool_local_db"
        }
    },
    "AuxJobMaker": {
        "CreateAuxBlockIntervalSeconds": 5, // 更新联合挖矿任务的频率（秒）
        "AuxPowJobListSize": 1000, // 保留的联合挖矿任务数（假设客户端每隔5秒调用一次本程序的getauxblock接口，则1000个任务是5000秒）

        // 可选，任务允许的最大Target（即最小难度）。如果任务Target大于该值（难度小于该值对应的难度），则用该值替换。
        // 用于控制难度非常低的链的出块速度，如设为 "00000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffff" 以防爆块速度过快系统处理不过来。
        // 注意：该值设置不合理会导致无法正常爆块。如果不需要该功能，请保持默认值或者删除该选项。
        "MaxJobTarget": "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" 
    },
    "Chains": [
        // 可添加任意数量的链
        {
            "Name": "Namecoin", // 链名，仅用于日志记录，内容可自定义
            "AuxTableName" :"found_nmc_blocks",// 存储auxpow相关信息的表名
            "RPCServer": {
                "URL": "http://127.0.0.1:8444/", // 联合挖矿RPC服务器
                "User": "test", // Basic认证用户名
                "Passwd": "123" // Basic认证密码
            },
            // 定义创建联合挖矿任务的RPC
            // 因为不同的区块链可能有不同的RPC（包括方法、参数和返回值），所以通过配置文件进行定义
            "CreateAuxBlock": {
                "Method": "getauxblock", // 方法名
                "Params": [], // 参数，可以是任意类型（数组、对象、字符串等）
                // 返回值键名映射
                // RPC的返回值必须类似于下面的结构，其中键名可以与下面的例子不同。
                // 不是所有键都是必须的，目前只有“hash”和“bits”是必须的（键名可以不同）。
                // “chainid”在某些情况下是必须的（看后面的描述）。
                /*
                    {
                        "result": {
                            "hash": "47478e2d769c26e702108b624dd403bfcae669cd51171aed7a85b985805ab032",
                            "chainid": 1,
                            "previousblockhash": "05f9d32813005597ae98c9c57427ff708be9651ae81e899caafacc36d5520f39",
                            "coinbasevalue": 5000000000,
                            "bits": "207fffff",
                            "height": 41,
                            "_target": "0000000000000000000000000000000000000000000000000000000000ffff7f"
                        },
                        "error": null,
                        "id": "curltest"
                    }
                */
                // 定义返回值中各个数据的键名
                // 若返回值中没有某个可选数据，则直接把对应的 key-value 从下方 {} 中删去
                // 若返回值中没有某个必须的数据，则该区块链节点与本程序的当前版本不兼容
                "ResponseKeys": {
                    "Hash": "hash", // 联合挖矿的区块头哈希，必须。
                    "ChainID": "chainid", // 链id，如果配置文件中没有定义链id的具体值，则该键名必须存在。
                    "Bits": "bits", // 联合挖矿要求的难度，必须。采用[比特币区块头内nBits字段](https://bitcoin.org/en/developer-reference#target-nbits)的编码方法
                    "Height": "height", // 区块高度，可选。目前仅用于日志记录。
                    "PrevBlockHash": "previousblockhash", // 当前块的父区块哈希，可选。目前仅用于日志记录。
                    "CoinbaseValue": "coinbasevalue" // 挖到这个区块可获得的奖励，可选。目前仅用于日志记录。
                }
            },
            // 定义提交联合挖矿结果的RPC
            "SubmitAuxBlock": {
                "Method": "getauxblock", // 方法名
                // 参数
                // 支持两种类型的参数：
                //     1. JSON-RPC 1.0 数组参数
                //     1. JSON-RPC 2.0 命名参数（对象、Map、键值对）
                // 某个参数的值若为字符串，则可以包含“变量”标记，这些标记在提交时会被替换为对应的值。
                // 目前仅有两个可用的“变量”标记：
                //    {hash-hex}       联合挖矿的区块头哈希（从 CreateAuxBlock 中获得，用来表示挖的是哪个区块）
                //    {aux-pow-hex}    工作量证明数据的hex表示。该数据结构遵循比特币联合挖矿标准：https://en.bitcoin.it/wiki/Merged_mining_specification#Aux_proof-of-work_block
                // 参数中可以包含除“变量”标记外的其他文本（常量），或者也可以包含非字符串类型的参数。数值、null、数组、对象等都是允许的。
                // 但需要注意的是，只有最外层数组/对象中的字符串中的“变量”标记会被替换。
                // 如果某区块链节点要求将区块头哈希或工作量证明放入深层数组或对象中，则其与本程序的当前版本不兼容。
                "Params": [
                    "{hash-hex}",
                    "{aux-pow-hex}"
                ]
            }
        },
        {
            "Name": "Namecoin ChainID 7",
            // 可以在本程序中强制修改掉区块链的链id
            // 该选项通常只用于调试，或者可用于兼容RPC返回值中不包含链id的区块链节点
            // 若这里定义的链id与区块链实际要求的不同，则联合挖矿结果会被区块链节点拒绝
            "ChainID": 7, // 重载的链id
            "RPCServer":{
                "URL": "http://127.0.0.1:9444/",
                "User": "test",
                "Passwd": "123"
            },
            "CreateAuxBlock": {
                "Method": "createauxblock",
                "Params": [ "my2dxGb5jz43ktwGxg2doUaEb9WhZ9PQ7K" ],
                // 这里不必（且不能）出现"ChainID"字段，否则上面重载的链id不会生效
                "ResponseKeys": {
                    "Hash": "hash",
                    "Bits": "bits",
                    "Height": "height",
                    "PrevBlockHash": "previousblockhash",
                    "CoinbaseValue": "coinbasevalue"
                }
            },
            "SubmitAuxBlock": {
                "Method": "submitauxblock",
                "Params": [
                    "{hash-hex}",
                    "{aux-pow-hex}"
                ]
            }
        },
        {
            "Name": "Elastos Regtest",
            "AuxTableName" :"found_Elastos_blocks",
            "RPCServer":{
                "URL": "http://127.0.0.1:4336/",
                "User": "test",
                "Passwd": "123"
            },
            "CreateAuxBlock": {
                "Method": "createauxblock",
                // 这里使用了命名参数
                "Params": {
                    "paytoaddress": "8VYXVxKKSAxkmRrfmGpQR2Kc66XhG6m3ta"
                },
                "ResponseKeys": {
                    "Hash": "hash",
                    "ChainID": "chainid",
                    "Bits": "bits",
                    "Height": "height",
                    "PrevBlockHash": "previousblockhash",
                    "CoinbaseValue": "coinbasevalue"
                }
            },
            "SubmitAuxBlock": {
                "Method": "submitauxblock",
                // 命名参数
                // 注意，只有第一层对象的value中的“变量”标记可以被替换
                "Params": {
                    "blockhash": "{hash-hex}",
                    "auxpow": "{aux-pow-hex}"
                }
            }
        }
    ]
}
```

#### 创建supervisor条目

```bash
vim /etc/supervisor/conf.d/merged-mining-proxy.conf
```

```conf
[program:merged-mining-proxy]
directory=/work/golang/mergedMiningProxy
command=/work/golang/bin/mergedMiningProxy -config=/work/golang/mergedMiningProxy/config.json -log_dir=/work/golang/mergedMiningProxy/log -v 2
autostart=true
autorestart=true
startsecs=6
startretries=20

redirect_stderr=true
stdout_logfile_backups=5
stdout_logfile=/work/golang/mergedMiningProxy/log/stdout.log
```

#### 运行

```bash
supervisorctl reread
supervisorctl update
supervisorctl status
```

#### 更新

```bash
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/btcpool-go-modules/mergedMiningProxy
diff /work/golang/src/github.com/btccom/btcpool-go-modules/mergedMiningProxy/config.default.json /work/golang/mergedMiningProxy/config.json
```

### 调用该代理的RPC

支持 `getauxblock`, `createauxblock`, `submitauxblock` 等方法，方法的参数列表与域名币（Namecoin）一致。

如果不是上述方法，则该方法将被转发到配置文件中定义的第一个区块链节点处，并原样返回结果。

一个特例是 `help` 方法，为了兼容 BTCPool `nmcauxmaker` 的域名币节点版本检查逻辑，help 方法会在原始返回值后附加对 `createauxblock` 和 `submitauxblock` 方法的描述。

如：

```bash
# 获取任务
curl -v --user admin:admin --data-binary '{"id":null,"method":"getauxblock","params":[]}' -H 'content-type: application/json' http://localhost:8999/

# 提交任务
curl -v --user admin:admin --data-binary '{"id":1,"method":"getauxblock","params":["6c8adaefa07ab5ff14ddff1b8e2bae4ecfc59ef0a985064bd202565106ff054b","02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4b039ccd09041b96485b742f4254432e434f4d2ffabe6d6d6c8adaefa07ab5ff14ddff1b8e2bae4ecfc59ef0a985064bd202565106ff054b020000004204cb9a010388711000000000000000ffffffff0200000000000000001976a914c0174e89bd93eacd1d5a1af4ba1802d412afc08688ac0000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9000000002d6009ef30ae316bcebe42ea7f4feaf995fb34211aa80b9835e06b0388769ce6000000000000000000000000002075cc0a4e259833d348dd282c00a61ab112bea0e02d1ac85e4773d08a01b87b3f2dc921f1fd927d473649cbb7115debb95de77455401566d56b12a94cbfca8dff1b96485bffff7f201b96485b"]}' -H 'content-type: application/json' http://localhost:8999/

# 获取任务（这里指定的钱包将被忽略，因为无法确定是哪个币种的）
curl -v --user admin:admin --data-binary '{"id":null,"method":"createauxblock","params":["my2dxGb5jz43ktwGxg2doUaEb9WhZ9PQ7K"]}' -H 'content-type: application/json' http://localhost:8999/

# 提交任务
curl -v --user admin:admin --data-binary '{"id":1,"method":"submitauxblock","params":["6c8adaefa07ab5ff14ddff1b8e2bae4ecfc59ef0a985064bd202565106ff054b","02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4b039ccd09041b96485b742f4254432e434f4d2ffabe6d6d6c8adaefa07ab5ff14ddff1b8e2bae4ecfc59ef0a985064bd202565106ff054b020000004204cb9a010388711000000000000000ffffffff0200000000000000001976a914c0174e89bd93eacd1d5a1af4ba1802d412afc08688ac0000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9000000002d6009ef30ae316bcebe42ea7f4feaf995fb34211aa80b9835e06b0388769ce6000000000000000000000000002075cc0a4e259833d348dd282c00a61ab112bea0e02d1ac85e4773d08a01b87b3f2dc921f1fd927d473649cbb7115debb95de77455401566d56b12a94cbfca8dff1b96485bffff7f201b96485b"]}' -H 'content-type: application/json' http://localhost:8999/
```

#### 获取任务

返回的结果格式如下：

```js
{
    "id": "1",
    "result": {
        "hash": "9e077526b9e82040ec82492993d6e1d25c92ce572d03eb1caa6d3b868a558a32", // 联合挖矿区块hash（实际为多个区块hash的merkle root）
        "chainid": 1, // 虚假的链id，始终为1
        "previousblockhash": "949a1539fa4ac733d651f6709967d541374e3e23f4422ea6ac2bf925e385807a", // 第一个区块链的当前块的父区块哈希
        "coinbasevalue": 5000000000, // 第一个区块链的当前块奖励
        "bits": "207fffff", // 多个区块链中最小的难度对应的bits
        "height": 123, // 第一个区块链的当前块高度
        "_target": "0000000000000000000000000000000000000000000000000000000000ffff7f", // 上述bits对应的target
        "merkle_size": 2, // 联合挖矿 merkle tree 的大小
        "merkle_nonce": 2596996162 // 确定各个区块链在 merkle tree 上的位置用的随机值
    },
    "error": null
}
```

格式类似于域名币（Namecoin）的相同RPC的返回值，不过多了两个字段：`merkle_size` 和 `merkle_nonce`。

为了在主链（比特币）中正确嵌入多币种合并挖矿tag，矿池必须进行适配，识别这两个字段并放入[coinbase交易的联合挖矿tag中](https://en.bitcoin.it/wiki/Merged_mining_specification#Merged_mining_coinbase)。


#### 提交任务

若提交的工作量满足至少一个区块链的难度，返回：

```
{"id":1,"result":true,"error":null}
````

否则，返回

```
{"id":1,"result":null,"error":{"code":400,"message":"high-hash"}}
````

如果发生其他错误，比如`block hash`未找到，或`aux pow`格式不正确等，则会返回对应的错误提示，格式类似上面的 `400 high-hash`。
例如：

```
{"id":1,"result":null,"error":{"code":400,"message":"cannot found blockHash d725af6c2243cdc8eb1180f72f820b8692f360ec1a2d87df0ba0c7c1c61f2c95 from AuxPowData 02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4b039ccd09041b96485b742f4254432e434f4d2ffabe6d6d6c8adaefa07ab5ff14ddff1b8e2bae4ecfc59ef0a985064bd202565106ff054b020000004204cb9a010388711000000000000000ffffffff0200000000000000001976a914c0174e89bd93eacd1d5a1af4ba1802d412afc08688ac0000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9000000002d6009ef30ae316bcebe42ea7f4feaf995fb34211aa80b9835e06b0388769ce6000000000000000000000000002075cc0a4e259833d348dd282c00a61ab112bea0e02d1ac85e4773d08a01b87b3f2dc921f1fd927d473649cbb7115debb95de77455401566d56b12a94cbfca8dff1b96485bffff7f301b96485b"}}
```

注意，该RPC返回`true`不代表工作量真的被至少一个区块链接受。具体提交是否成功，还需要看本程序的日志。


### TODO

* 将各个区块链的爆块记录写入数据库。
