# Komari Lite Monitor

以监控为核心、仅含源码的 Komari 精简分支，已按公开托管要求整理：仓库内**不含**生产数据库、账号、节点 Token、TLS 证书、凭据、备份、发布二进制和继承自上游的 Git 历史。

[English README](README.md)

## 保留与移除

保留：节点、实时/历史指标、Ping 探测、告警、登录与 2FA、审计日志、备份恢复、公开 Glass 监控页、面板本地 Agent 分发。

移除：远程终端/远程控制/远程命令、Cloudflare Tunnel 管理、自更新与版本检查、主题市场与主题上传/导入/更新/切换、Nezha、回程线路检测，以及除简体中文和英文之外的语言。

公开监控页使用固定的本地 `Glass` 主题：仅在该主题缺失时初始化，后续程序构建不会覆盖已安装的本地 Glass 副本；主题设置保留，但主题生命周期接口刻意不可用。

## 目录

- `backend/` — Go 面板服务及内嵌的公开页/后台页资源。
- `frontend/` — React/Vite 后台管理界面源码。
- `agent/` — 仅监控的 Go Agent 源码。
- `scripts/build-agent-release.sh` — 构建面板本地分发的 Agent 制品。
- `deploy/` — 通用原生 systemd 更新模板，不含任何生产环境取值。

## 环境要求

- Go 1.25 或更高；面板需开启 CGO 以支持 SQLite。
- Node.js 20+ 与 npm。
- 常用 Linux 工具：`bash`、`python3`、`sha256sum`；可选 Caddy 提供 TLS。

## 1. 编译主程序

面板会把后台界面内嵌进二进制，因此必须按下面顺序构建：

```bash
git clone <你的 GitHub 仓库地址> komari-lite
cd komari-lite

cd frontend
npm ci
npm test
npm run build

# 把后台构建产物复制到 Go 内嵌目录
rm -rf ../backend/web/public/systemUI/dist
mkdir -p ../backend/web/public/systemUI/dist
cp -a dist/. ../backend/web/public/systemUI/dist/

cd ../backend
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go build -trimpath -ldflags '-s -w' -o ../release/komari .
```

最终二进制为 `release/komari`。

## 2. 编译 Agent

Agent 不含远程控制、终端、命令执行、MCP 与自动更新模块。一次构建全部平台制品并生成面板分发清单：

```bash
cd komari-lite
chmod +x scripts/build-agent-release.sh
./scripts/build-agent-release.sh 1.0.2
```

输出到 `release/agent-1.0.2/`：

- 14 个 `komari-agent-<os>-<arch>` 制品；
- `manifest.json`：版本号与每个制品的 SHA-256；
- `SHA256SUMS.txt`：离线校验用。

该发布目录已被 Git 忽略。部署前先校验：

```bash
cd release/agent-1.0.2
sha256sum -c SHA256SUMS.txt
```

## 3. 传统原生部署（Debian/systemd，不使用 Docker）

下面示例中的用户名、路径和域名请替换为你自己的取值。

### 服务器侧准备

```bash
sudo useradd --system --home /opt/komari --shell /usr/sbin/nologin komari
sudo install -d -m 0775 -o root -g komari /opt/komari
sudo install -d -o komari -g komari /opt/komari/data
sudo install -m 0755 release/komari /opt/komari/komari
sudo chown -R komari:komari /opt/komari/data
```

创建 `/etc/systemd/system/komari.service`：

```ini
[Unit]
Description=Komari Lite Monitor
After=network-online.target
Wants=network-online.target

[Service]
User=komari
Group=komari
WorkingDirectory=/opt/komari
ExecStart=/opt/komari/komari server --listen 127.0.0.1:25774
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/komari

[Install]
WantedBy=multi-user.target
```

`/opt/komari` 仅向服务组提供组写权限，以便备份还原在 `data/` 同级创建暂存目录并原子替换数据。二进制仍保持 root 所有、`0755` 权限。还原包暂存后程序会以状态 0 退出，因此必须使用 `Restart=always` 让 systemd 再次启动以执行还原。

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now komari.service
sudo systemctl status komari.service
```

### 用 Caddy 终止 TLS

创建 `/etc/caddy/Caddyfile`，把 `monitor.example.com` 换成你的域名：

```caddy
monitor.example.com {
    reverse_proxy 127.0.0.1:25774
}
```

按 Caddy 官方文档安装并启动。面板只监听回环地址，80/443 由 Caddy 占用。

### 把 Agent 发布到面板

面板启动后，把构建好的发布目录放入面板运行数据目录：

```bash
sudo install -d -o komari -g komari /opt/komari/data/agent-release
sudo cp -a release/agent-1.0.2/. /opt/komari/data/agent-release/
sudo chown -R komari:komari /opt/komari/data/agent-release
sudo chmod 0640 /opt/komari/data/agent-release/manifest.json
sudo find /opt/komari/data/agent-release -type f -name 'komari-agent-*' -exec chmod 0755 {} \;
```

只更新 Agent 制品无需重启面板。面板通过 `/agent/install.sh`、`/agent/install.ps1` 与 `/agent/download/<artifact>` 提供下载。

## 4. 更新主程序

把校验过的二进制放到 `/opt/komari/komari.new`，在服务器上运行通用更新脚本：

```bash
sudo install -m 0755 deploy/update-native.sh /opt/komari/update.sh

export EXPECTED_HOSTNAME="your-hostname"
export EXPECTED_MACHINE_ID="$(cat /etc/machine-id)"
sudo /opt/komari/update.sh /opt/komari/komari.new
```

`deploy/update-native.sh` 会保留带时间戳的旧二进制；更新校验失败时自动回滚。详见 [`deploy/README.md`](deploy/README.md)。

## 提交前检查

不要把 `data/`、Agent 发布制品、凭据、密钥/证书、数据库、备份和编译产物提交到仓库；`.gitignore` 已拦截这些路径。

每次推送前运行：

```bash
./scripts/github-preflight.sh
```

该脚本会拒绝运行数据、可识别的密钥/Token 特征以及本机专用域名与 IP，并会检查仓库文本中是否残留部署标识。最后仍建议人工复核 `git diff --cached`。

## License

上游项目许可证保留在 [`LICENSE`](LICENSE) 与 [`backend/NOTICE`](backend/NOTICE)。再分发时请保留上游声明并遵守其条款。
