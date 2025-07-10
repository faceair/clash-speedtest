### 原文档见原库。

这里列一下自己常用的命令：
```
go build -v
```
**编译**
```
go build -o clash-speedtest.exe
```
**快速筛选出延迟低于 800ms 节点，并输出到 normal.yaml**
```
.\clash-speedtest.exe -fast -c "URL&flag=meta" -output normal.yaml -max-latency 800ms
```
