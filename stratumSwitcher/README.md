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
prlimit --nofile=327680 --pid=`supervisorctl pid switcher`
kill -USR2 `supervisorctl pid switcher`
```

进程将在原pid上载入新的二进制，不会产生新的pid。在原进程退出前，会写入“./runtime.json”（包含监听端口和其正在代理的所有连接的信息）供新进程恢复连接使用。请确保进程对其工作目录有写权限。

在大部分情况下，新进程可以恢复所有原进程正在代理的连接，但是所有处于认证阶段的连接将被抛弃。

不过偶尔有时候，新进程无法恢复某些连接（提示文件描述符无效），此时这些连接将断开，不会造成资源泄漏。该问题的起因是：在exec执行前，调用获取文件描述符的命令会导致进程占用的文件描述符加倍。一但文件描述符超过supervisor中设置的上限，后续连接就将无法保留。上面列出的`prlimit`命令就是为了解决该问题而添加的。

新的二进制将重新读取配置文件，并监听其定义的端口。因此可以在平滑重启前修改配置文件实现切换监听端口。

不过需要注意的是，如果文件描述符在保留连接阶段即达到上限，exec命令可能会因为缺少可用文件描述符而失败，此时程序将崩溃退出。请确保在`prlimit`命令中设置了足够的文件描述符。
