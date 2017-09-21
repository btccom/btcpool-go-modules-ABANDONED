# Stratum Switcher

一个 Stratum 代理，可根据外部指令（Zookeeper下特定路径中的值）自动在不同币种的 Stratum 服务器之间进行切换。

### 构建 & 运行

构建

```bash
mkdir -p /work/golang
apt install -y golang
export GOPATH=/work/golang
go get github.com/btccom/stratumSwitcher/stratumSwitcher
```

编辑配置文件

```bash
mkdir /work/golang/stratumSwitcher
mkdir /work/golang/stratumSwitcher/log
cp /work/golang/src/github.com/btccom/stratumSwitcher/stratumSwitcher/config.default.json /work/golang/stratumSwitcher/config.json
vim /work/golang/stratumSwitcher/config.json
```

创建supervisor条目

```bash
vim /etc/supervisor/conf.d/switcher.conf
```

```conf
[program:switcher]
directory=/work/golang/stratumSwitcher
command=/work/golang/bin/stratumSwitcher -config=/work/golang/stratumSwitcher/config.json -log_dir=/work/golang/stratumSwitcher/log -v 2
autostart=true
autorestart=true
startsecs=6
startretries=20

redirect_stderr=true
stdout_logfile_backups=5
stdout_logfile=/work/golang/stratumSwitcher/log/stdout.log
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
go get -u github.com/btccom/stratumSwitcher/stratumSwitcher
diff /work/golang/src/github.com/btccom/stratumSwitcher/stratumSwitcher/config.default.json /work/golang/stratumSwitcher/config.json
```
