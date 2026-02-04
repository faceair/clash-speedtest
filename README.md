# Clash-SpeedTest

基于 Clash/Mihomo 核心的测速工具，快速测试你的节点速度。

Features:
1. 无需额外的配置，直接将 Clash/Mihomo 配置本地文件路径或者订阅地址作为参数传入即可
2. 支持 Proxies 和 Proxy Provider 中定义的全部类型代理节点，兼容性跟 Mihomo 一致
3. 不依赖额外的 Clash/Mihomo 进程实例，单一工具即可完成测试
4. 代码简单而且开源，不发布构建好的二进制文件，保证你的节点安全

<img width="1347" height="677" alt="Image" src="https://github.com/user-attachments/assets/9418f5e4-0866-43d3-bcd4-529c0bcbb183" />

## Prerequisites/注意事项

### OpenWRT 环境
在 OpenWRT 环境下使用本工具时，建议临时关闭 OpenClash/Clash/Mihomo 等代理服务，以避免路由冲突影响测速结果的准确性。或者给 OpenClash/Clash/Mihomo 配置进程规则绕过代理：
```
rules:
  - PROCESS-NAME,clash-speedtest,DIRECT
```

### Windows CMD 用户
在 Windows CMD 中使用时，如果订阅地址包含 `&` 字符，必须使用双引号而非单引号：
```bash
# 正确
> clash-speedtest -c "https://domain.com/api/v1/client/subscribe?token=secret&flag=meta"

# 错误
> clash-speedtest -c 'https://domain.com/api/v1/client/subscribe?token=secret&flag=meta'
```

## 使用方法

```bash
# 支持从源码安装，或从 Release 里下载由 Github Action 自动构建的二进制文件
> go install github.com/faceair/clash-speedtest@latest

# 查看版本
> clash-speedtest -v

# 查看帮助
> clash-speedtest -h
Usage of clash-speedtest:
  -c string
        configuration file path, also support http(s) url
  -f string
        filter proxies by name, use regexp (default ".*")
  -b string
        block proxies by keywords, use | to separate multiple keywords (example: -b 'rate|x1|1x')
  -server-url string
        server url or direct download url (default "https://dl.google.com/chrome/mac/universal/stable/GGRO/googlechrome.dmg")
  -speed-mode string
        speed test mode: fast, download, full (default "download")
  -download-size int
        download size for testing proxies (default 50MB)
  -upload-size int
        upload size for testing proxies (full mode only) (default 20MB)
  -timeout duration
        timeout for testing proxies (default 5s)
  -concurrent int
        download concurrent size (default 4)
  -output string
        output config file path (default "")
  -max-latency duration
        filter latency greater than this value (default 800ms)
  -max-packet-loss float
        filter packet loss greater than this value(unit: %) (default 100)
  -min-download-speed float
        filter speed less than this value(unit: MB/s) (default 5)
  -min-upload-speed float
        filter upload speed less than this value(unit: MB/s, full mode only) (default 2)
  -rename
        rename nodes with IP location and speed
  -fast
        fast mode (alias for --speed-mode fast)
  -gist-token string
        GitHub personal access token for gist upload
  -gist-address string
        gist URL or ID for uploading output file (filename: fastsub.yaml)

# 演示：

# 1. 测试全部节点，使用 HTTP 订阅地址
# 请在订阅地址后面带上 flag=meta 参数，否则无法识别出节点类型
> clash-speedtest -c 'https://domain.com/api/v1/client/subscribe?token=secret&flag=meta'

# 2. 测试香港节点，使用正则表达式过滤，使用本地文件
> clash-speedtest -c ~/.config/clash/config.yaml -f 'HK|港'
节点                                        	带宽          	延迟
Premium|广港|IEPL|01                        	484.80KB/s  	815.00ms
Premium|广港|IEPL|02                        	N/A         	N/A
Premium|广港|IEPL|03                        	2.62MB/s    	333.00ms
Premium|广港|IEPL|04                        	1.46MB/s    	272.00ms
Premium|广港|IEPL|05                        	3.87MB/s    	249.00ms

# 3. 当然你也可以混合使用
> clash-speedtest -c "https://domain.com/api/v1/client/subscribe?token=secret&flag=meta,/home/.config/clash/config.yaml"

# 4. 筛选出延迟低于 800ms 且下载速度大于 5MB/s 的节点，并输出到 filtered.yaml
> clash-speedtest -c "https://domain.com/api/v1/client/subscribe?token=secret&flag=meta" -output filtered.yaml -max-latency 800ms -min-speed 5
# 筛选后的配置文件可以直接粘贴到 Clash/Mihomo 中使用，或是贴到 Github\Gist 上通过 Proxy Provider 引用。

# 5. 使用 -rename 选项按照 IP 地区和下载速度重命名节点
> clash-speedtest -c config.yaml -output result.yaml -rename
# 重命名后的节点名称格式：🇺🇸 US 001 | ⬇️ 15.67MB/s
# 包含国旗 emoji、国家代码和下载速度

# 6. 快速测试模式
> clash-speedtest -f 'HK' -fast -c ~/.config/clash/config.yaml
# 此命令将只测试节点延迟，跳过其他测试项目，适用于：
# - 快速检查节点是否可用
# - 只需要检查延迟的场景
# - 需要快速得到测试结果的场景
🇭🇰 香港 HK-10 100% |██████████████████| (20/20, 13 it/min)
序号    节点名称                类型            延迟
1.      🇭🇰 香港 HK-01           Trojan          657ms
2.      🇭🇰 香港 HK-20           Trojan          649ms
3.      🇭🇰 香港 HK-15           Trojan          674ms
4.      🇭🇰 香港 HK-19           Trojan          649ms
5.      🇭🇰 香港 HK-12           Trojan          667ms

# 7. 上传到 GitHub Gist
> clash-speedtest -c config.yaml -output result.yaml -gist-token "ghp_xxx" -gist-address "https://gist.github.com/username/abc123"
# 测试完成后，会将 result.yaml 上传到指定的 Gist，文件名为 fastsub.yaml
# gist-address 可以是完整的 Gist URL，也可以是 Gist ID（如 abc123）

## 测速原理

通过 HTTP GET 请求下载指定大小的文件，默认使用 https://dl.google.com/chrome/mac/universal/stable/GGRO/googlechrome.dmg 进行测试，计算下载时间得到下载速度。因为 speedtest.cloudflare.com 容易返回 403，所以默认不再使用它作为测速入口。

当 server-url 不带 path 时 (https://speedtest.cloudflare.com 或自建测速服务)，使用 /__down 和 /__up 完成下载与上传测试。
当 server-url 带 path 时，会被识别为直接下载地址，只进行下载测速。

如果你确认 speedtest.cloudflare.com 可以访问并希望测试上传，请显式设置为 full 模式，例如：
```shell
clash-speedtest --server-url "https://speed.cloudflare.com" --speed-mode full
```

测试结果：
1. 带宽 是指下载指定大小文件的速度，即一般理解中的下载速度。当这个数值越高时表明节点的出口带宽越大。
2. 延迟 是指 HTTP GET 请求拿到第一个字节的的响应时间，即一般理解中的 TTFB。当这个数值越低时表明你本地到达节点的延迟越低，可能意味着中转节点有 BGP 部署、出海线路是 IEPL、IPLC 等。

请注意带宽跟延迟是两个独立的指标，两者并不关联：
1. 可能带宽很高但是延迟也很高，这种情况下你下载速度很快但是打开网页的时候却很慢，可能是是中转节点没有 BGP 加速，但出海线路带宽很充足。
2. 可能带宽很低但是延迟也很低，这种情况下你打开网页的时候很快但是下载速度很慢，可能是中转节点有 BGP 加速，但出海线路的 IEPL、IPLC 带宽很小。

Cloudflare 是全球知名的 CDN 服务商，其提供的测速服务器到海外绝大部分的节点速度都很快，一般情况下都没有必要自建测速服务器。

如果你不想使用 Cloudflare 的测速服务器，可以自己搭建一个测速服务器。

```shell
# 在您需要进行测速的服务器上安装和启动测速服务器
> go install github.com/faceair/clash-speedtest/download-server@latest
> download-server

# 此时在本地使用 http://your-server-ip:8080 作为 server-url 即可
> clash-speedtest --server-url "http://your-server-ip:8080"
```

## License

[GPL-3.0](LICENSE)
