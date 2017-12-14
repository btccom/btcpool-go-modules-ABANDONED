# Init User Coin

通过拉取每个币种的子账户名/puid列表来增量的初始化zookeeper里的用户币种记录。

该程序是可选的，取决于矿池的用户系统架构。如果你的子账户列表根本不区分币种，就不需要部署该程序，直接使用`Switcher API Server`的定时任务初始化币种记录即可。

### 子账户名/puid列表接口

本程序所要求的`子账户名/puid列表`与 BTCPool 里 [sserver](https://github.com/btccom/btcpool/blob/master/src/sserver/sserver.cfg) 要求的相同，具体说明如下：

#### 接口约定

假设配置文件为
```json
{
    "UserListAPI": {
        "btc": "http://127.0.0.1:8000/btc-userlist.php",
        "bcc": "http://127.0.0.1:8000/bcc-userlist.php"
    },
    "IntervalSeconds": 10,
    ...
}
```

则程序会启动两个`goroutine`（线程），同时访问 `btc` 和 `bcc` 的子账户名/puid列表接口。

以`btc`的子账户名/puid列表接口为例，首次访问的实际URL为：
```
http://127.0.0.1:8000/btc-userlist.php?last_id=0
```
接口返回完整的用户/puid列表，如：
```
{
    "err_no": 0,
    "err_msg": null,
    "data": {
        "aaa": 1,
        "bbb": 2,
        "mmm_btc": 4,
        "vvv": 5,
        "ddd": 6
    }
}
```
程序将遍历该列表并将用户`aaa`、`bbb`、`vvv`、`ddd`的所挖币种设置为`btc`。该程序只负责初始化，不负责后续的币种切换，因此它简单的认为出现在`btc`列表中的用户所挖币种就是`btc`，出现在`bcc`列表中的用户所挖币种就是`bcc`。

此外，带有下划线的用户将被跳过，因此程序不会设置`mmm_btc`子账户的所挖币种。

等待 `IntervalSeconds` 秒后，程序将再次访问如下URL：
```
http://127.0.0.1:8000/btc-userlist.php?last_id=6
```
其中`6`为上次得到的最大puid。

如果没有`puid`大于`6`的用户被注册，接口返回空`data`对象：
```json
{
    "err_no": 0,
    "err_msg": null,
    "data": {
    }
}
```
否则，接口返回`puid`大于`6`的用户，如：
```json
{
    "err_no": 0,
    "err_msg": null,
    "data": {
        "xxx": 7
    }
}
```
此后，用户`xxx`的所挖币种会被设置为`btc`，并且`last_id`变为`7`。

##### 备注

1. 重启该程序是安全的。虽然程序会重新开始遍历子账户列表，但是对于`zookeeper`中已经存在的子账户，该程序不会再写入记录。因此，该程序的重启不会影响用户后续的币种切换。

2. 该程序可以一直运行，这样它就可以增量的初始化刚注册的新用户的币种了。

3. 同一个子账户在`btc`和`bcc`列表中同时出现的话，该程序将其初始化为`btc`或`bcc`取决于它先处理了哪边的记录。如果你的所有子账户都会在两边同时出现，并且puid也相同，或者你的子账户列表根本不区分币种，就不需要部署该程序，直接使用`Switcher API Server`的定时任务初始化币种记录即可。

#### 参考实现

这里有一个实现`UserListAPI`的例子：https://github.com/btccom/btcpool/issues/16#issuecomment-278245381

### 构建 & 运行

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
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/stratumSwitcher/initUserCoin
```

编辑配置文件

```bash
mkdir /work/golang/initUserCoin
mkdir /work/golang/initUserCoin/log
cp /work/golang/src/github.com/btccom/stratumSwitcher/initUserCoin/config.default.json /work/golang/initUserCoin/config.json
vim /work/golang/initUserCoin/config.json
```

创建supervisor条目

```bash
vim /etc/supervisor/conf.d/switcher-inituser.conf
```

```conf
[program:switcher-inituser]
directory=/work/golang/initUserCoin
command=/work/golang/bin/initUserCoin -config=/work/golang/initUserCoin/config.json -log_dir=/work/golang/initUserCoin/log -v 2
autostart=true
autorestart=true
startsecs=6
startretries=20

redirect_stderr=true
stdout_logfile_backups=5
stdout_logfile=/work/golang/initUserCoin/log/stdout.log
```

运行

```bash
supervisorctl reread
supervisorctl update
supervisorctl status
```

#### 更新

```bash
export GOPATH=/work/golang
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/stratumSwitcher/initUserCoin
diff /work/golang/src/github.com/btccom/stratumSwitcher/initUserCoin/config.default.json /work/golang/initUserCoin/config.json
```
