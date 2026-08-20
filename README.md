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
x2socks edit 1 --bind '127.0.0.1,::1'
x2socks edit 1 --port 1234
x2socks edit 1 --uri 'ss://...'
x2socks remove 1
x2socks test 'ss://...'
x2socks test 'vless://...?security=reality&pbk=...#name'
```

URI 必须用单引号包住。`vless://` / `trojan://` 查询串里的 `&` 否则会被 shell 拆成后台任务，命令实际只收到 `?` 后面第一段。

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
