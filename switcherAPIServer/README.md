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
