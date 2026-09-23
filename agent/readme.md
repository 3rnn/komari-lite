# Komari Lite Agent (monitoring only)

跨平台节点监控 Agent，仅上报节点数据与执行探测任务。本仓库版本已移除远程终端、文件管理、远程命令、MCP 代理和自动更新模块；一键部署从与面板版本一致的 GitHub Release 下载 Agent 制品。

## 能力

- 上报 CPU、内存、磁盘、网络、负载、进程、GPU、系统信息与累计流量。
- 执行面板下发的延迟/Ping 探测任务并回报结果。
- 支持 v2 协议压缩、IPv4/IPv6 连接偏好、自定义 DNS、自定义上报 IP、网卡筛选。
- 支持面板在线下发部分采集配置（采集间隔、流量重置日、网卡/挂载点筛选、内存口径、GPU 监控）。
- 支持 Cloudflare Access Service Token（主连接、上报与任务结果均携带认证头）。

已移除：远程终端、远程命令、文件管理、MCP 代理、自动更新/版本检查、回程探测。

`--enable-remote-control` / `--disable-web-ssh` 参数仅为兼容旧配置保留，不再产生任何远程能力。

## 安装

面板后台「添加节点」生成的一键部署命令会自动带上你的面板地址与节点 Token；安装脚本由面板提供，Agent 二进制固定从对应面板版本的 GitHub Release 下载：

```bash
# Linux / macOS / FreeBSD
curl -fsSL https://<your-panel>/agent/install.sh | sudo bash -s -- \
  --endpoint "https://<your-panel>" \
  --token "<node-token>"
```

```powershell
# Windows (PowerShell)
iwr "https://<your-panel>/agent/install.ps1" -UseBasicParsing -OutFile install.ps1
.\install.ps1 --endpoint "https://<your-panel>" --token "<node-token>"
```

`/agent/install.sh` 与 `/agent/install.ps1` 仍由面板提供以接收节点 Token；它们通过 `--install-source` 固定到 `https://github.com/3rnn/komari-lite/releases/download/v<panel-version>`，仅下载同版本、同平台的 Agent 制品。

## 手动运行

```bash
./komari-agent-linux-amd64 --endpoint "https://<your-panel>" --token "<node-token>"
```

## 配置

参数可通过 JSON 配置文件、环境变量或命令行参数传入，优先级：默认值 < JSON 配置文件 < 环境变量 < 显式命令行参数。

```json
{
  "endpoint": "https://<your-panel>",
  "token": "<node-token>",
  "interval": 3,
  "ignore_unsafe_cert": false
}
```

常用配置项：

| JSON 字段 | 环境变量 | 命令行参数 | 说明 |
| --- | --- | --- | --- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | 面板地址 |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | 节点 Token |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | 数据采集间隔（秒） |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | 忽略不安全证书，默认关闭 |
| `max_retries` | `AGENT_MAX_RETRIES` | `--max-retries`, `-r` | 最大重试次数 |
| `reconnect_interval` | `AGENT_RECONNECT_INTERVAL` | `--reconnect-interval`, `-c` | 重连间隔（秒） |
| `info_report_interval` | `AGENT_INFO_REPORT_INTERVAL` | `--info-report-interval` | 基础信息上报间隔（分钟） |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | 仅统计指定网卡，逗号分隔 |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | 排除指定网卡，逗号分隔 |
| `include_mountpoints` | `AGENT_INCLUDE_MOUNTPOINTS` | `--include-mountpoint` | 仅统计指定挂载点，分号分隔 |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | 流量重置日，`0` 为禁用 |
| `month_rotate_time` | `AGENT_MONTH_ROTATE_TIME` | `--month-rotate-time` | 重置时刻 `HH:MM:SS`，默认 `00:00:00` |
| `month_rotate_timezone` | `AGENT_MONTH_ROTATE_TIMEZONE` | `--month-rotate-timezone` | 重置时区，默认 `Asia/Shanghai` |
| `memory_include_cache` | `AGENT_MEMORY_INCLUDE_CACHE` | `--memory-include-cache` | 内存使用量包含缓存 |
| `memory_report_raw_used` | `AGENT_MEMORY_REPORT_RAW_USED` | `--memory-exclude-bcf` | 使用排除 buffer/cache 的口径 |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | 启用详细 GPU 监控 |
| `custom_ipv4` / `custom_ipv6` | `AGENT_CUSTOM_IPV4` / `AGENT_CUSTOM_IPV6` | `--custom-ipv4` / `--custom-ipv6` | 自定义上报 IP |
| `get_ip_addr_from_nic` | `AGENT_GET_IP_ADDR_FROM_NIC` | `--get-ip-addr-from-nic` | 从网卡获取上报 IP |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | 自定义 DNS 服务器 |
| `protocol_version` | `AGENT_PROTOCOL_VERSION` | `--protocol-version` | 上报协议版本，仅支持 `2` |
| `disable_compression` | `AGENT_DISABLE_COMPRESSION` | `--disable-compression` | 禁用 v2 传输压缩 |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | 优先使用的 IP 版本，`4` 或 `6` |
| `cf_access_client_id` | `AGENT_CF_ACCESS_CLIENT_ID` | `--cf-access-client-id` | Cloudflare Access Client ID |
| `cf_access_client_secret` | `AGENT_CF_ACCESS_CLIENT_SECRET` | `--cf-access-client-secret` | Cloudflare Access Client Secret |

完整参数见 `./komari-agent-<os>-<arch> --help`。

## 编译

需要 Go 1.24 或更高版本。单平台编译：

```bash
cd agent
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o komari-agent .
```

构建全部 14 个平台制品并生成面板分发清单：

```bash
./scripts/build-agent-release.sh 1.0.3
# 输出 release/agent-1.0.3/：14 个制品 + manifest.json + SHA256SUMS.txt
```

将该目录中的 14 个 `komari-agent-*` 制品上传到与面板相同版本的 GitHub Release；一键安装会按平台从该 Release 下载对应文件。

## 模块路径

`go.mod` 目前沿用上游模块路径 `github.com/nuomiiiii/lite-agent`。若要发布到你自己的 GitHub 命名空间，需先全局替换 module 行与所有 import，并执行 `go mod tidy` 后再提交。

## License

见 [`LICENSE`](LICENSE)。保留上游声明并遵守其条款。
