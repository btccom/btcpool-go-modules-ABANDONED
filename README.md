# [User Chain API Server](userChainAPIServer/)

由两个模块合并而来：
* [Switcher API Server](userChainAPIServer/switcherAPIServer/)
  提供触发 Stratum 切换的API
* [Init User Coin](userChainAPIServer/initUserCoin/)
  初始化zookeeper里的用户币种记录

# [Merged Mining Proxy](mergedMiningProxy/)

多币种联合挖矿代理，支持域名币（Namecoin）、亦来云（Elastos）等同时与比特币联合挖矿。

# [Init NiceHash](initNiceHash/)

初始化 ZooKeeper 中的 NiceHash 配置，通过调用 NiceHash API 来获取各个算法要求的最小难度，写入 ZooKeeper 以备 sserver 来使用。

# [Chain Switcher](chainSwitcher/)
向sserver发送币种自动切换命令。

# [Stratum Switcher](stratumSwitcher/)

可切换币种的 Stratum 代理，用于配合 BTCPool 工作。

**已废弃，btcpool项目中的sserver现在直接具有币种切换功能，不再需要Stratum Switcher。**

* [BTCPool for Bitcoin Cash](https://github.com/btccom/bccpool)
* [BTCPool for Bitcoin](https://github.com/btccom/btcpool)
