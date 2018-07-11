# Stratum Switcher

一个 Stratum 代理，可根据外部指令（Zookeeper下特定路径中的值）自动在不同币种的 Stratum 服务器之间进行切换。

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
GIT_TERMINAL_PROMPT=1 go get github.com/btccom/btcpool-go-modules/stratumSwitcher
```

生成安装包（可选）

```bash
cd $GOPATH/src/github.com/btccom/btcpool-go-modules/stratumSwitcher
mkdir build
cd build
cmake ..
make package
```

编辑配置文件

```bash
mkdir /work/golang/stratumSwitcher
mkdir /work/golang/stratumSwitcher/log
cp /work/golang/src/github.com/btccom/btcpool-go-modules/stratumSwitcher/config.default.json /work/golang/stratumSwitcher/config.json
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

更改supervisor文件描述符数量（即TCP最大连接数）
```bash
sed -i "s/\\[supervisord\\]/[supervisord]\nminfds=65535/" /etc/supervisor/supervisord.conf
service supervisor restart
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
GIT_TERMINAL_PROMPT=1 go get -u github.com/btccom/btcpool-go-modules/stratumSwitcher
diff /work/golang/src/github.com/btccom/btcpool-go-modules/stratumSwitcher/config.default.json /work/golang/stratumSwitcher/config.json
```

##### 平滑重启/热更新（实验性）

该功能可用于升级 stratumSwitcher 到新版本、更改 stratumSwitcher 配置使其生效，或单纯的重启服务。在服务重启过程中，大部分正在代理的Stratum连接都不会断开。

目前该功能仅在Linux上可用。

```bash
killall -USR2 stratumSwitcher
```

进程将在原pid上载入新的二进制，不会产生新的pid。在原进程退出前，会写入“./runtime.json”（包含监听端口和其正在代理的所有连接的信息）供新进程恢复连接使用。请确保进程对其工作目录有写权限。

在大部分情况下，新进程可以恢复所有原进程正在代理的连接，但是所有处于认证阶段的连接将被抛弃。

不过偶尔有时候，新进程无法恢复某些连接（提示文件描述符无效），此时这些连接将断开，不会造成资源泄漏。

但在极端情况下，新进程会无法恢复监听端口的文件描述符。新进程会尝试重新监听，但是偶尔也会失败，这将导致新进程无法继续对外服务，曾经观察到过这样的现象。

因此，在使用平滑重启功能后，请检查新进程是能正常接受新连接。
