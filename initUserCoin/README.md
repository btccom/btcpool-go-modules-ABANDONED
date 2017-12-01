# Init User Coin

通过拉取每个币种的子账户名/puid列表来初始化zookeeper里的用户币种记录

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
