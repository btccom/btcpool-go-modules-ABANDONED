# Switcher API Server

调用该进程提供的API服务控制 StratumSwitcher 进行 Stratum 切换。该进程通过向 Zookeeper 的特定目录写入值来通知 StratumSwitcher 进行切换。


子账户aaaa切换到btc：
```
http://10.0.0.12:8082/switch?puname=aaaa&coin=btc
```

子账户aaaa切换到bcc：
```
http://10.0.0.12:8082/switch?puname=aaaa&coin=bcc
```

该API的返回结果：

成功：
```
{"err_no":0, "err_msg":"", success:true}
```

失败：
```
{"err_no":非0整数, "err_msg":"错误信息", success:false}
```


### 构建 & 运行

构建

```bash
mkdir -p /work/golang
apt install -y golang
export GOPATH=/work/golang
go get github.com/btccom/stratumSwitcher/switcherAPIServer
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

#### 更新

```bash
export GOPATH=/work/golang
go get -u github.com/btccom/stratumSwitcher/switcherAPIServer
diff /work/golang/src/github.com/btccom/stratumSwitcher/switcherAPIServer/config.default.json /work/golang/switcherAPIServer/config.json
```
