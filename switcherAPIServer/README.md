# Switcher API Server

该进程用来修改 zookeeper 中的币种记录，以便控制 StratumSwitcher 进行币种切换。该进程一共有两种工作方式，一为通过定时任务拉取最新的用户币种信息，二为外部通过调用该进程提供的API来主动推送用户币种信息。

## 定时任务

在配置文件中设置 EnableCronJob 为 true 即可开启定时任务，此后进程将每隔 `CronIntervalSeconds` 拉取一次 `UserCoinMapURL`，以获得最新的用户币种信息。

`UserCoinMapURL` 的参考实现如下：

```php
待添加
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
http://hostname:port/switch-multi-user

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

## 构建 & 运行

安装golang

```bash
mkdir ~/source
cd ~/source
wget http://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz
cd /usr/local
tar zxf ~/source/go1.9.2.linux-amd64.tar.gz
ln -s /usr/local/go/bin/go /usr/local/bin/go
```

构建

```bash
mkdir -p /work/golang
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/stratumSwitcher/switcherAPIServer
```

编辑配置文件

```bash
mkdir /work/golang/switcherAPIServer
mkdir /work/golang/switcherAPIServer/log
cp /work/golang/src/github.com/btccom/stratumSwitcher/switcherAPIServer/config.default.json /work/golang/switcherAPIServer/config.json
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
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/stratumSwitcher/switcherAPIServer
diff /work/golang/src/github.com/btccom/stratumSwitcher/switcherAPIServer/config.default.json /work/golang/switcherAPIServer/config.json
```
