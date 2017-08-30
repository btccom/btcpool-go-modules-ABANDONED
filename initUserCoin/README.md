# Init User Coin

通过拉取每个币种的子账户名/puid列表来初始化zookeeper里的用户币种记录

### 构建 & 运行

构建

```bash
mkdir -p /work/golang
apt install -y golang
export GOPATH=/work/golang
go get github.com/btccom/stratumSwitcher/initUserCoin
```

创建配置文件

```bash
mkdir /work/golang/initUserCoin
cp /work/golang/src/github.com/btccom/stratumSwitcher/initUserCoin/config.default.json /work/golang/initUserCoin/config.json
vim /work/golang/initUserCoin/config.json
```

运行

```bash
cd /work/golang/initUserCoin
/work/golang/bin/initUserCoin --logtostderr -v 2
```
