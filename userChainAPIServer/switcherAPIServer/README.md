# Switcher API Server

该进程用来修改 zookeeper 中的币种记录，以便控制 StratumSwitcher 进行币种切换。该进程一共有两种工作方式，一为通过定时任务拉取最新的用户币种信息，二为外部通过调用该进程提供的API来主动推送用户币种信息。

## 定时任务

在配置文件中设置 `EnableCronJob` 为 `true` 即可开启定时任务，此后进程将每隔 `CronIntervalSeconds` 拉取一次 `UserCoinMapURL`，以获得最新的用户币种信息。

### 接口约定

假设 `UserCoinMapURL` 为 `http://127.0.0.1:8000/usercoin.php`，则程序首次访问的实际URL为：
```
http://127.0.0.1:8000/usercoin.php?last_date=0
```

接口返回一个JSON字符串，包含所有用户及其正在挖的币种（无论是否进行过切换），形如：
```json
{
    "err_no": 0,
    "data": {
        "user_coin": {
            "user1": "btc",
            "user2": "bcc",
            "user3": "bcc",
            "user4": "btc"
        },
        "now_date": 1513239055
    }
}
```
其中，`user1`、`user2`、`user3`为子账户名，`btc`和`bcc`为币种，`now_date`为服务器的当前系统时间。

经过配置文件中设置的 `CronIntervalSeconds` 秒后，程序会再次访问如下URL：
```
http://127.0.0.1:8000/usercoin.php?last_date=1513239055
```
其中，`1513239055`为上次服务器返回的`now_date`。

此时，服务器可根据程序提供的`last_date`进行判断，如果在`last_date`到现在这段时间内没有任何用户进行过切换，则返回空`user_coin`对象：
```json
{
    "err_no": 0,
    "data": {
        "user_coin": {},
        "now_date": 1513239064
    }
}
```
> 注意：不可返回`user_coin`数组，如`"user_coin":[]`，否则程序会在日志中产生警告。使用PHP数组实现接口时，在输出前请先将`user_coin`成员的类型强制转换为对象。

否则，返回在这段时间内进行切换的用户及切换后的币种：
```json
{
    "err_no": 0,
    "data": {
        "user_coin": {
            "user1": "bcc",
            "user3": "btc"
        },
        "now_date": 1513239064
    }
}
```

备注：如果性能不受影响，服务器也可以忽略`last_date`参数，总是返回所有用户及其正在挖的币种，无论其是否或在什么时间进行过切换。

### 参考实现

`UserCoinMapURL` 的参考实现如下：

```php
<?php
header('Content-Type: application/json');

$last_id = (int) $_GET['last_id'];

$coins = ["btc", "bcc"];

$users = [
    'hu60' => $coins[rand(0,1)],
    'YihaoTest' => $coins[rand(0,1)],
    'YihaoTest3' => $coins[rand(0,1)],
    'testpool' => $coins[rand(0,1)],
];

if ($last_id >= count($users)) {
    $users = [];
}

echo json_encode(
    [
        'err_no' => 0,
        'err_msg' => null,
        'data' => (object) $users,
    ]
);
```

## API 文档

在配置文件中设置 EnableAPIServer 为 true 即可开启该API服务。外部在用户发起切换请求时可调用该API主动推送切换消息，以便 StratumSwitcher 第一时间进行币种切换。

目前共有两种调用方式：

### 单用户切换

#### 认证方式
HTTP Basic 认证

#### 请求URL
http://hostname:port/switch

#### 请求方式
GET 或 POST

#### 参数
|  名称  |  类型  |   含义   |
| ------ | ----- | -------- |
| puname | string | 子账户名 |
|  coin  | string |   币种  |

#### 例子

子账户aaaa切换到btc：
```bash
curl -u admin:admin 'http://127.0.0.1:8082/switch?puname=aaaa&coin=btc'
```

子账户aaaa切换到bcc：
```bash
curl -u admin:admin 'http://10.0.0.12:8082/switch?puname=aaaa&coin=bcc'
```

该API的返回结果：

成功：
```json
{"err_no":0, "err_msg":"", "success":true}
```

失败：
```
{"err_no":非0整数, "err_msg":"错误信息", "success":false}
```
例如
```json
{"err_no":104,"err_msg":"coin is inexistent","success":false}
```

### 批量切换

#### 认证方式
HTTP Basic 认证

#### 请求URL
http://hostname:port/switch/multi-user

#### 请求方式
POST

`Content-Type: application/json`

#### 请求Body内容

```json
{
    "usercoins": [
        {
            "coin": "币种1",
            "punames": [
                "用户1",
                "用户2",
                "用户3",
                ...
            ]
        },
        {
            "coin": "币种2",
            "punames": [
                "用户4",
                "用户5",
                ...
            ]
        },
        ...
    ]
}
```

#### 例子

子账户a,b,c切换到btc，d,e切换到bcc：
```bash
curl -u admin:admin -d '{"usercoins":[{"coin":"btc","punames":["a","b","c"]},{"coin":"bcc","punames":["d","e"]}]}' 'http://127.0.0.1:8082/switch-multi-user'
```

该API的返回结果：

所有子账户均切换成功：
```json
{"err_no":0, "err_msg":"", "success":true}
```

任一子账户切换失败：
```
{"err_no":非0整数, "err_msg":"错误信息", "success":false}
```
例如
```json
{"err_no":108,"err_msg":"usercoins is empty","success":false}
```

### 获取子池Coinbase信息和爆块地址

#### 认证方式
HTTP Basic 认证

#### 请求URL
http://hostname:port/subpool/get-coinbase

#### 请求方式
POST

`Content-Type: application/json`

#### 请求Body内容
```json
{
	"coin": "币种",
	"subpool_name": "子池名称"
}
```

#### 响应

成功：
```json
{
	"success": true,
	"err_no": 0,
	"err_msg": "success",
	"subpool_name": "子池名称",
	"old": {
		"coinbase_info": "coinbase信息",
		"payout_addr": "爆块地址"
	}
}
```

失败：
```json
{
	"err_no": 错误代码,
	"err_msg": "错误信息",
	"success": false
}

数据竞争（多个get-coinbase/update-coinbase同时调用时可能会出现，错开时间再试一次就可以成功）：
{
	"err_no": 500,
	"err_msg": "data has been updated at query time",
	"success": false
}
```

例子：
```json
curl -uadmin:admin -d'{"coin":"btc","subpool_name":"pool3"}' http://localhost:8080/subpool/get-coinbase
{
	"success": true,
	"err_no": 0,
	"err_msg": "success",
	"subpool_name": "pool3",
	"old": {
		"coinbase_info": "tigerxx",
		"payout_addr": "34woZDygXWqaVPnNxp5SUnbN6RNQ5koBt4"
	}
}

curl -uadmin:admin -d'{"coin":"bch","subpool_name":"pool3"}' http://localhost:8080/subpool/get-coinbase
{
	"err_no": 404,
	"err_msg": "subpool 'pool3' does not exist",
	"success": false
}

curl -uadmin:admin -d'{"coin":"bch","subpool_name":"pool3"}' http://localhost:8080/subpool/get-coinbase
{
	"err_no": 500,
	"err_msg": "data has been updated at query time",
	"success": false
}
```

### 更新子池Coinbase信息和爆块地址

#### 认证方式
HTTP Basic 认证

#### 请求URL
http://hostname:port/subpool/update-coinbase

#### 请求方式
POST

`Content-Type: application/json`

#### 请求Body内容
```json
{
	"coin": "币种",
	"subpool_name": "子池名称",
	"payout_addr": "爆块地址",
	"coinbase_info": "coinbase信息"
}
```

#### 响应

成功：
```json
{
	"success": true,
	"err_no": 0,
	"err_msg": "success",
	"subpool_name": "子池名称",
	"old": {
		"coinbase_info": "旧coinbase信息",
		"payout_addr": "旧爆块地址"
	},
	"new": {
		"coinbase_info": "新coinbase信息",
		"payout_addr": "新爆块地址"
	}
}
```

失败：
```json
{
	"success": false,
	"err_no": 错误代码,
	"err_msg": "错误信息",
	"subpool_name": "子池名称",
	"old": {
		"coinbase_info": "旧coinbase信息",
		"payout_addr": "旧爆块地址"
	},
	"new": {
		"coinbase_info": "",
		"payout_addr": ""
	}
}
```

请求参数错误：
```json
{
	"err_no": 错误代码,
	"err_msg": "错误信息",
	"success": false
}
```

例子：
```json
curl -uadmin:admin -d'{"coin":"btc","subpool_name":"pool3","payout_addr":"bc1qjl8uwezzlech723lpnyuza0h2cdkvxvh54v3dn","coinbase_info":"tiger"}' http://localhost:8080/subpool/update-coinbase
{
	"success": true,
	"err_no": 0,
	"err_msg": "success",
	"subpool_name": "pool3",
	"old": {
		"coinbase_info": "hellobtc",
		"payout_addr": "34woZDygXWqaVPnNxp5SUnbN6RNQ5koBt4"
	},
	"new": {
		"coinbase_info": "tiger",
		"payout_addr": "bc1qjl8uwezzlech723lpnyuza0h2cdkvxvh54v3dn"
	}
}

curl -uadmin:admin -d'{"coin":"btc","subpool_name":"pool3","payout_addr":"bc0qjl8uwezzlech723lpnyuza0h2cdkvxvh54v3dn","coinbase_info":"tiger"}' http://localhost:8080/subpool/update-coinbase
{
	"success": false,
	"err_no": 500,
	"err_msg": "invalid payout address",
	"subpool_name": "pool3",
	"old": {
		"coinbase_info": "tiger",
		"payout_addr": "bc1qjl8uwezzlech723lpnyuza0h2cdkvxvh54v3dn"
	},
	"new": {
		"coinbase_info": "",
		"payout_addr": ""
	}
}

curl -uadmin:admin -d'{"coin":"btc","subpool_name":"pool4","payout_addr":"bc1qjl8uwezzlech723lpnyuza0h2cdkvxvh54v3dn","coinbase_info":"tiger"}' http://localhost:8080/subpool/update-coinbase
{
	"err_no": 404,
	"err_msg": "subpool 'pool4' does not exist",
	"success": false
}
```


## 构建 & 运行

安装golang

```bash
mkdir ~/source
cd ~/source
wget http://storage.googleapis.com/golang/go1.10.3.linux-amd64.tar.gz
cd /usr/local
tar zxf ~/source/go1.10.3.linux-amd64.tar.gz
ln -s /usr/local/go/bin/go /usr/local/bin/go
```

构建

```bash
mkdir -p /work/golang
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/btcpool-go-modules/switcherAPIServer
```

编辑配置文件

```bash
mkdir /work/golang/switcherAPIServer
mkdir /work/golang/switcherAPIServer/log
cp /work/golang/src/github.com/btccom/btcpool-go-modules/switcherAPIServer/config.default.json /work/golang/switcherAPIServer/config.json
vim /work/golang/switcherAPIServer/config.json
```

创建supervisor条目

```bash
vim /etc/supervisor/conf.d/switcher-api.conf
```

```conf
[program:switcher-api]
directory=/work/golang/switcherAPIServer
command=/work/golang/bin/switcherAPIServer -config=/work/golang/switcherAPIServer/config.json -log_dir=/work/golang/switcherAPIServer/log -v 2
autostart=true
autorestart=true
startsecs=6
startretries=20

redirect_stderr=true
stdout_logfile_backups=5
stdout_logfile=/work/golang/switcherAPIServer/log/stdout.log
```

运行

```bash
supervisorctl reread
supervisorctl update
supervisorctl status
```

## 更新

```bash
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/btcpool-go-modules/switcherAPIServer
diff /work/golang/src/github.com/btccom/btcpool-go-modules/switcherAPIServer/config.default.json /work/golang/switcherAPIServer/config.json
```
