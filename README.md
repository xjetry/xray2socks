# Xray2Socks

基于 Xray Core 的本地 SOCKS5 管理器。每个远端节点对应一个独立的本地 SOCKS5 端口，例如：

```text
ss1     -> 127.0.0.1:1080
ss2     -> 127.0.0.1:1081
vless1  -> 127.0.0.1:1082
trojan1 -> 127.0.0.1:1083
```

`x2socks` 只负责配置和启停，体积约 6MB；实际转发由旁边的 `xray` 二进制完成。

仓库：https://github.com/xjetry/xray2socks

## 安装

macOS / Linux 用 Homebrew（推荐）：

```bash
brew trust xjetry/tap        # Homebrew 6.0+ 需要，只需一次；旧版可跳过
brew install xjetry/tap/x2socks
```

或用脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/xjetry/xray2socks/main/install.sh | bash
```

GitHub 访问困难时用 R2：

```bash
curl -fsSL https://pub-119b1b5d6fec46188a13787ef4d3646b.r2.dev/xray2socks/install.sh | bash
```

需要走 GitHub 代理再装 Xray：

```bash
curl -fsSL https://pub-119b1b5d6fec46188a13787ef4d3646b.r2.dev/xray2socks/install.sh | XRAY2SOCKS_GH_PROXY=https://gh-proxy.com/ bash
```

root 装到 `/usr/local/bin/x2socks`，普通用户装到 `~/.local/bin/x2socks`。PATH 里还没有 `xray` 时会顺带从 Xray-core 官方 release 拉一份。

## 运行

命令名是 `x2socks`：

```bash
x2socks list
x2socks add 'ss://...'
x2socks add 'ss://...' 1081
x2socks add 'ss://...' 1081 '127.0.0.1,10.0.0.2,[2001:db8::1]'
x2socks add 'vless://...' 'ss://...'          # 链式转发
x2socks edit 1 --bind '127.0.0.1,::1'
x2socks edit 1 --port 1234
x2socks edit 1 --uri 'ss://...'
x2socks edit 1 --uri 'vless://...' --uri 'ss://...'   # 改成链式
x2socks remove 1
x2socks test 'ss://...'
x2socks test 'vless://...?security=reality&pbk=...#name'
x2socks test 'vless://...' 'ss://...'         # 测试整条链
```

URI 必须用单引号包住。`vless://` / `trojan://` 查询串里的 `&` 否则会被 shell 拆成后台任务，命令实际只收到 `?` 后面第一段。

`add` 可以接多个 URI 组成链式转发：第一个 URI 是入口，最后一个 URI 是实际出口，中间可以任意多跳。例如 `add 'ss://a' 'socks5://b' 'trojan://c'` 表示流量依次经过 `a -> b -> c`，出口 IP 是 `c`。跳支持 ss / vless / trojan / socks5（含 socks）/ http / https，例如前置一个公司内网的 socks5 或 http 代理很常见。`test` 同样支持多个 URI 测试整条链。`edit --uri` 重复多次即为链式；只传一个 `--uri` 会把已有链改成单节点。`list` 的 TARGET 列用 `->` 显示整条链路。

`add` 的端口和 bind 都可省略：端口从 1081 起跳过配置里已用的和系统占用的；bind 默认 `0.0.0.0`，多个地址用逗号分隔、不区分 v4/v6，IPv6 用方括号。`edit --bind` 可改监听地址。`add` / `edit` / `remove` 会后台启动 Xray。

`list` 的 ID 从 1 开始，并显示 generate204 延迟。`test {uri}` 只检查连通性，不写配置。URI 合法即可保存，不通也不拦。`edit` 的 `--uri`、`--port`、`--bind` 至少填一个。

网页管理：

```bash
x2socks serve --bind 127.0.0.1 --web-addr 127.0.0.1:8080
```

卸载：

```bash
sudo x2socks uninstall
sudo x2socks uninstall --purge
```

不加 `--purge` 只删程序和 systemd 单元；`--purge` 还会删配置、`xray-runtime.json`，以及 `/etc/x2socks`、`/etc/xray2socks`。不删除系统里的 `xray`。

## Docker Compose

发布镜像：`ghcr.io/xjetry/xray2socks`，镜像内已带 `xray`。把仓库里的 `docker-compose.yml` 放到工作目录后：

```bash
docker compose pull
docker compose up -d
```

管理页：`http://127.0.0.1:8080`。配置在 `./data/config.json`。默认映射本机 `1080-1090` 到容器 SOCKS；节点本地端口要落在这个区间，或改 `docker-compose.yml` 的 `ports`。

容器内同样可用命令行：

```bash
docker compose exec x2socks x2socks list
docker compose exec x2socks x2socks add 'ss://...' 1080
```

`add` / `edit` / `remove` 只改文件，需要 `docker compose restart` 后 Xray 才会用新配置。网页里保存并启动走的是同一进程，不用重启。

## systemd

```bash
sudo x2socks install --bind 127.0.0.1 --web-addr 127.0.0.1:8080 --config /etc/x2socks/config.json
sudo systemctl daemon-reload
sudo systemctl enable --now x2socks
```

不传 `--bind` 时，SOCKS5 入站监听所有 IPv4 网卡。配置默认保存为当前目录的 `config.json`，可使用 `--config /etc/x2socks/config.json` 指定路径。
