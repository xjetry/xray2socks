#!/usr/bin/env bash
# 用法：curl -fsSL https://raw.githubusercontent.com/xjetry/xray2socks/main/install.sh | bash
set -euo pipefail

BASE="${XRAY2SOCKS_BASE:-https://github.com/xjetry/xray2socks/releases/latest/download}"
GH_PROXY="${XRAY2SOCKS_GH_PROXY:-}"
if [[ -n "$GH_PROXY" && "$GH_PROXY" != */ ]]; then
  GH_PROXY="$GH_PROXY/"
fi

github_url() {
  case "$1" in
    https://github.com/*) echo "${GH_PROXY}$1" ;;
    *) echo "$1" ;;
  esac
}

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "需要 $1" >&2; exit 1; }
}
need curl

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "不支持的架构: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux | darwin) ;;
  *) echo "不支持的系统: $os" >&2; exit 1 ;;
esac

if [[ "$(id -u)" -eq 0 ]]; then
  BIN_DIR="${XRAY2SOCKS_BIN_DIR:-/usr/local/bin}"
else
  BIN_DIR="${XRAY2SOCKS_BIN_DIR:-$HOME/.local/bin}"
fi
mkdir -p "$BIN_DIR"

name="x2socks-${os}-${arch}"
echo "下载 $name -> $BIN_DIR/x2socks"
curl -fsSL "$(github_url "$BASE/$name")" -o "$BIN_DIR/x2socks"
chmod +x "$BIN_DIR/x2socks"
rm -f "$BIN_DIR/xray2socks"

install_xray() {
  case "$os-$arch" in
    linux-amd64) zip=Xray-linux-64.zip ;;
    linux-arm64) zip=Xray-linux-arm64-v8a.zip ;;
    darwin-amd64) zip=Xray-macos-64.zip ;;
    darwin-arm64) zip=Xray-macos-arm64-v8a.zip ;;
    *) echo "没有对应的 Xray 包: $os $arch" >&2; return 1 ;;
  esac
  url="$(github_url "https://github.com/XTLS/Xray-core/releases/latest/download/$zip")"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN
  echo "下载 Xray ($zip)"
  curl -fsSL "$url" -o "$tmp/xray.zip"
  if command -v unzip >/dev/null 2>&1; then
    unzip -o -j -q "$tmp/xray.zip" xray -d "$tmp"
  else
    need python3
    python3 - "$tmp/xray.zip" "$tmp/xray" <<'PY'
import sys, zipfile
z, dest = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(z) as f:
    open(dest, "wb").write(f.read("xray"))
PY
  fi
  chmod +x "$tmp/xray"
  mv "$tmp/xray" "$BIN_DIR/xray"
}

if ! command -v xray >/dev/null 2>&1 && [[ ! -x "$BIN_DIR/xray" ]]; then
  install_xray
fi

echo
echo "已安装: $BIN_DIR/x2socks"
command -v xray >/dev/null 2>&1 && echo "Xray: $(command -v xray)" || echo "Xray: $BIN_DIR/xray"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "请把 $BIN_DIR 加入 PATH" ;;
esac
echo
echo "管理:  x2socks list | add | edit | remove | test"
echo "网页:  x2socks serve --bind 127.0.0.1"
echo "服务:  sudo x2socks install --bind 127.0.0.1 --config /etc/x2socks/config.json"
echo "卸载:  sudo x2socks uninstall [--purge]"
