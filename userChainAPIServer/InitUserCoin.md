# 代码已重构

文档需要更新

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

此外，带有下划线的子账户名将被跳过，因此程序不会设置`mmm_btc`子账户的所挖币种。

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

3. 同一个子账户在`btc`和`bcc`列表中同时出现的话，该程序将其初始化为`btc`或`bcc`取决于它先处理了哪边的记录。如果你的所有子账户都会在两边同时出现，并且puid也相同，或者你的子账户列表根本不区分币种，就不需要部署该程序，直接使用[Switcher API Server](../switcherAPIServer#定时任务)的定时任务初始化币种记录即可。

##### 关于带有下划线的子账户名

带有下划线的子账户名可以用于“用户其实在`btc`和`bcc`币种下各有一个子账户，但是想让用户感觉自己只有一个子账户”的情况。具体的做法是：

1. 假设用户在`btc`币种下已经有了一个子账户，为`mmm`。
2. 用户操作币种切换功能，欲切换到`bcc`。此时，系统自动为用户在`bcc`币种下创建子账户，名为`mmm_bcc`。该子账户可能具有和`mmm`子账户不同的`puid`。
3. 系统同时调用[币种切换API](../switcherAPIServer#单用户切换)，将用户`mmm`的币种切换为`bcc`，如`http://10.0.0.12:8082/switch?puname=mmm&coin=bcc`。
4. 与此同时，要保证[UserCoinMapURL](../switcherAPIServer#接口约定) 返回的用户`mmm`的币种也为`bcc`。此外，`UserCoinMapURL`的返回结果中不应该出现带有下划线的子账户名（因为从逻辑上来说带有下划线和不带下划线的子账户为同一个子账户）。
5. 用户依然使用子账户名`mmm`连接矿池。此时，`stratumSwitcher`将会把连接转发到`bcc`的`sserver`。但是`bcc`处没有名为`mmm`的子账户，所以矿机认证会失败。此时，`stratumSwitcher`会自动将子账户名转换为`mmm_bcc`重试，此时便会成功。用户已有的矿机也会这样被切换到`bcc`币种的`mmm_bcc`子账户。

#### 参考实现

这里有一个实现`UserListAPI`的例子：https://github.com/btccom/btcpool/issues/16#issuecomment-278245381

### 构建 & 运行

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
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/btcpool-go-modules/initUserCoin
```

编辑配置文件

```bash
mkdir /work/golang/initUserCoin
mkdir /work/golang/initUserCoin/log
cp /work/golang/src/github.com/btccom/btcpool-go-modules/initUserCoin/config.default.json /work/golang/initUserCoin/config.json
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
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/btcpool-go-modules/initUserCoin
diff /work/golang/src/github.com/btccom/btcpool-go-modules/initUserCoin/config.default.json /work/golang/initUserCoin/config.json
```
