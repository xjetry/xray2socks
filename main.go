package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web
var webFiles embed.FS

type Proxy struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	LocalPort   int    `json:"localPort"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid,omitempty"`
	Password    string `json:"password,omitempty"`
	Method      string `json:"method,omitempty"`
	Network     string `json:"network,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	ServerName  string `json:"serverName,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	ShortID     string `json:"shortId,omitempty"`
	Flow        string `json:"flow,omitempty"`
	Path        string `json:"path,omitempty"`
	Host        string `json:"host,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	Listen      string `json:"listen,omitempty"`
}

type AppConfig struct {
	BindHost string  `json:"bindHost,omitempty"`
	Proxies  []Proxy `json:"proxies"`
}

type app struct {
	mu     sync.Mutex
	config AppConfig
	cmd    *exec.Cmd
	file   string
}

func defaultConfig() AppConfig {
	return AppConfig{Proxies: []Proxy{}}
}

func newApp(file string) (*app, error) {
	a := &app{config: defaultConfig(), file: file}
	b, err := os.ReadFile(file)
	if err == nil {
		if err := json.Unmarshal(b, &a.config); err != nil {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return a, nil
}

func (a *app) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(a.file), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.file, append(b, '\n'), 0600)
}

func (a *app) running() bool { return a.cmd != nil }

func (a *app) xrayFile() string {
	return filepath.Join(filepath.Dir(a.file), "xray-runtime.json")
}

func (a *app) startLocked() error {
	if a.cmd != nil {
		return errors.New("Xray 已经在运行")
	}
	if err := validateConfig(a.config); err != nil {
		return err
	}
	bin, err := lookXray()
	if err != nil {
		return err
	}
	b, err := buildXrayConfig(a.config)
	if err != nil {
		return err
	}
	stopPidFile(a.pidFile())
	cmd, err := startXray(bin, a.xrayFile(), b)
	if err != nil {
		return err
	}
	a.cmd = cmd
	_ = os.WriteFile(a.pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0600)
	return nil
}

func (a *app) stopLocked() error {
	if a.cmd == nil {
		stopPidFile(a.pidFile())
		return nil
	}
	err := stopXray(a.cmd)
	a.cmd = nil
	_ = os.Remove(a.pidFile())
	return err
}

func validateConfig(c AppConfig) error {
	if len(c.Proxies) == 0 {
		return errors.New("至少需要配置一个代理")
	}
	ports := make(map[int]bool, len(c.Proxies))
	for i, p := range c.Proxies {
		if p.Name == "" || p.Address == "" || p.Port < 1 || p.Port > 65535 || p.LocalPort < 1 || p.LocalPort > 65535 {
			return fmt.Errorf("第 %d 个代理的名称、地址或端口无效", i+1)
		}
		if ports[p.LocalPort] {
			return fmt.Errorf("本地端口 %d 被重复使用", p.LocalPort)
		}
		ports[p.LocalPort] = true
		switch strings.ToLower(p.Type) {
		case "ss":
			if p.Password == "" || p.Method == "" {
				return fmt.Errorf("代理 %q 缺少密码或加密方式", p.Name)
			}
		case "vless":
			if p.UUID == "" {
				return fmt.Errorf("代理 %q 缺少 UUID", p.Name)
			}
		case "trojan":
			if p.Password == "" {
				return fmt.Errorf("代理 %q 缺少密码", p.Name)
			}
		default:
			return fmt.Errorf("代理 %q 的类型仅支持 ss、vless 或 trojan", p.Name)
		}
	}
	return nil
}

func socksListen(c AppConfig, p Proxy) string {
	return formatListenAddrs(socksListenAddrs(c, p))
}

func checkPorts(c AppConfig) error {
	for _, p := range c.Proxies {
		for _, addr := range socksListenAddrs(c, p) {
			ln, err := net.Listen("tcp", net.JoinHostPort(addr, strconv.Itoa(p.LocalPort)))
			if err != nil {
				return fmt.Errorf("本地 %s:%d 不可用: %w", addr, p.LocalPort, err)
			}
			_ = ln.Close()
		}
	}
	return nil
}

func (a *app) serve() http.Handler {
	mux := http.NewServeMux()
	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/config", a.configHandler)
	mux.HandleFunc("/api/status", a.statusHandler)
	mux.HandleFunc("/api/start", a.startHandler)
	mux.HandleFunc("/api/stop", a.stopHandler)
	mux.HandleFunc("/api/test", testHandler)
	mux.HandleFunc("/api/parse", parseHandler)
	return logging(mux)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *app) configHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.config)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "仅支持 GET 或 PUT", http.StatusMethodNotAllowed)
		return
	}
	var c AppConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, 400, map[string]string{"error": "配置 JSON 无效"})
		return
	}
	if err := validateConfig(c); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if a.cmd != nil {
		writeJSON(w, 409, map[string]string{"error": "请先停止 Xray 再修改配置"})
		return
	}
	if err := checkPorts(c); err != nil {
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	a.config = c
	if err := a.saveLocked(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, a.config)
}

func (a *app) statusHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ports := make([]map[string]any, 0, len(a.config.Proxies))
	for _, p := range a.config.Proxies {
		ports = append(ports, map[string]any{"name": p.Name, "port": p.LocalPort})
	}
	writeJSON(w, 200, map[string]any{"running": a.running(), "ports": ports})
}
func (a *app) startHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.startLocked(); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"running": true})
}
func (a *app) stopHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.stopLocked(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"running": false})
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI   string `json:"uri"`
		Proxy Proxy  `json:"proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求无效"})
		return
	}
	p := input.Proxy
	if input.URI != "" {
		var err error
		p, err = parseProxyURI(input.URI)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	got := probeProxy(p)
	body := map[string]any{"ok": got.OK, "status": got.Status, "latencyMs": got.LatencyMs}
	if got.Err != "" {
		body["error"] = got.Err
	}
	writeJSON(w, 200, body)
}

func parseHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求无效"})
		return
	}
	p, err := parseProxyURI(input.URI)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	p.LocalPort = 1080
	writeJSON(w, 200, p)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}

func installService(configFile, bind, webAddr string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	configFile, err = filepath.Abs(configFile)
	if err != nil {
		return err
	}
	execStart := fmt.Sprintf("%s --config %s --web-addr %s serve", executable, configFile, webAddr)
	if bind != "" {
		execStart = fmt.Sprintf("%s --config %s --bind %s --web-addr %s serve", executable, configFile, bind, webAddr)
	}
	unit := fmt.Sprintf(`[Unit]
Description=x2socks proxy service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, execStart)
	return os.WriteFile(unitX2socks, []byte(unit), 0644)
}

func main() {
	file := os.Getenv("XRAY2SOCKS_CONFIG")
	if file == "" {
		file = "config.json"
	}
	bind := os.Getenv("XRAY2SOCKS_BIND")
	addr := os.Getenv("XRAY2SOCKS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFlag := flags.String("config", file, "配置文件路径")
	bindFlag := flags.String("bind", bind, "SOCKS5 监听地址，留空监听所有 IPv4 网卡")
	webAddrFlag := flags.String("web-addr", addr, "管理页面监听地址和端口")
	if len(os.Args) > 1 && os.Args[1] == "install" {
		_ = flags.Parse(os.Args[2:])
		if err := installService(*configFlag, *bindFlag, *webAddrFlag); err != nil {
			log.Fatalf("安装 systemd 服务失败: %v", err)
		}
		log.Println("已写入 /etc/systemd/system/x2socks.service，请执行 systemctl daemon-reload && systemctl enable --now x2socks")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "uninstall" {
		fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
		config := fs.String("config", file, "配置文件路径")
		purge := fs.Bool("purge", false, "同时删除配置和 /etc/x2socks")
		_ = fs.Parse(os.Args[2:])
		if err := uninstall(*config, *purge); err != nil {
			log.Fatalf("卸载失败: %v", err)
		}
		if *purge {
			log.Println("已卸载并清理配置")
		} else {
			log.Println("已卸载，配置文件保留")
		}
		return
	}
	_ = flags.Parse(os.Args[1:])
	file = *configFlag
	bind = *bindFlag
	addr = *webAddrFlag
	args := flags.Args()
	if len(args) == 0 {
		fmt.Print(cliUsage)
		return
	}
	if args[0] == "tui" {
		a, err := newApp(file)
		if err != nil {
			log.Fatal(err)
		}
		if bind != "" {
			a.config.BindHost = bind
		}
		if err := runTUI(a, os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if args[0] == "serve" {
		if err := runServe(file, bind, addr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := runCommand(file, args); err != nil {
		log.Fatal(err)
	}
}

const cliUsage = `x2socks list
x2socks add {uri} [port] [bind]
x2socks edit {id} --port {port}
x2socks edit {id} --uri {uri}
x2socks edit {id} --bind {addr}
x2socks remove {id}
x2socks test '{uri}'
x2socks uninstall
x2socks uninstall --purge
`

func runServe(file, bind, addr string) error {
	a, err := newApp(file)
	if err != nil {
		return err
	}
	if bind != "" {
		a.config.BindHost = bind
	}
	a.mu.Lock()
	if err := a.startLocked(); err != nil {
		log.Printf("Xray 未启动: %v", err)
	}
	a.mu.Unlock()
	server := &http.Server{Addr: addr, Handler: a.serve(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("管理页面: http://%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	a.mu.Lock()
	_ = a.stopLocked()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
