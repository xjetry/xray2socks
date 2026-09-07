package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func runManage(c AppConfig, args []string) (AppConfig, string, error) {
	if len(args) == 0 {
		return c, "", fmt.Errorf("%s", manageUsage)
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return c, "", fmt.Errorf("用法: list")
		}
		return c, formatProxyList(c, probeAll(c.Proxies)), nil
	case "add":
		uris, port, bind, err := parseAddArgs(args[1:])
		if err != nil {
			return c, "", err
		}
		next, err := addProxy(c, uris, port, bind)
		if err != nil {
			return c, "", err
		}
		return next, fmt.Sprintf("added %d %d\n", len(next.Proxies), next.Proxies[len(next.Proxies)-1].LocalPort), nil
	case "remove":
		if len(args) != 2 {
			return c, "", fmt.Errorf("用法: remove {id}")
		}
		id, err := parseID(args[1], len(c.Proxies))
		if err != nil {
			return c, "", err
		}
		next := c
		next.Proxies = append(append([]Proxy{}, c.Proxies[:id-1]...), c.Proxies[id:]...)
		return next, fmt.Sprintf("removed %d\n", id), nil
	case "edit":
		next, err := editProxy(c, args[1:])
		if err != nil {
			return c, "", err
		}
		return next, "ok\n", nil
	case "test":
		if len(args) < 2 {
			return c, "", fmt.Errorf("用法: test '{uri}' ['{uri}'...]")
		}
		p, err := parseProxyURIs(args[1:])
		if err != nil {
			return c, "", err
		}
		r := probeFn(p)
		return c, latencyText(r) + "\n", nil
	default:
		return c, "", fmt.Errorf("未知命令: %s\n%s", args[0], manageUsage)
	}
}

const manageUsage = `用法:
  add {uri} [uri...] [port] [bind]   多个 uri 为链式转发，最后一个为出口
  remove {id}
  edit {id} [--uri URI]... [--port PORT] [--bind ADDR]
  list
  test '{uri}' ['{uri}'...]`

func parseAddArgs(args []string) (uris []string, port int, bind string, err error) {
	usage := fmt.Errorf("用法: add {uri} [uri...] [port] [bind]")
	for _, a := range args {
		switch {
		case strings.Contains(a, "://"):
			uris = append(uris, a)
		case isPortToken(a):
			if port != 0 {
				return nil, 0, "", fmt.Errorf("端口重复")
			}
			port, _ = strconv.Atoi(a)
		default:
			if bind != "" {
				return nil, 0, "", usage
			}
			bind = a
		}
	}
	if len(uris) == 0 {
		return nil, 0, "", usage
	}
	return uris, port, bind, nil
}

func parseID(raw string, n int) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 || id > n {
		return 0, fmt.Errorf("id 无效")
	}
	return id, nil
}

func addProxy(c AppConfig, uris []string, port int, bind string) (AppConfig, error) {
	if len(uris) == 0 {
		return c, fmt.Errorf("用法: add {uri} [uri...] [port] [bind]")
	}
	p, err := parseProxyURIs(uris)
	if err != nil {
		return c, err
	}
	if bind == "" {
		bind = "0.0.0.0"
	}
	p.Listen = bind
	if port == 0 {
		port, err = nextFreePort(c, bind)
		if err != nil {
			return c, err
		}
	}
	p.LocalPort = port
	next := c
	next.Proxies = append(append([]Proxy{}, c.Proxies...), p)
	if err := validateConfig(next); err != nil {
		return c, err
	}
	return next, nil
}

// parseProxyURIs 把一组 URI 解析为入口节点 + 链式转发，最后一个 URI 为实际出口。
func parseProxyURIs(uris []string) (Proxy, error) {
	p, err := parseProxyURI(uris[0])
	if err != nil {
		return Proxy{}, err
	}
	for _, u := range uris[1:] {
		hop, err := parseProxyURI(u)
		if err != nil {
			return Proxy{}, err
		}
		hop.LocalPort, hop.Listen = 0, ""
		p.Chain = append(p.Chain, hop)
	}
	return p, nil
}

func editProxy(c AppConfig, args []string) (AppConfig, error) {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var uris uriList
	fs.Var(&uris, "uri", "URI，可重复多次组成链式转发")
	port := fs.Int("port", 0, "")
	bind := fs.String("bind", "", "")
	editUsage := "用法: edit {id} [--uri URI]... [--port PORT] [--bind ADDR]"
	if len(args) == 0 {
		return c, fmt.Errorf("%s", editUsage)
	}
	id, err := parseID(args[0], len(c.Proxies))
	if err != nil {
		return c, err
	}
	if err := fs.Parse(args[1:]); err != nil {
		return c, fmt.Errorf("%s", editUsage)
	}
	if fs.NArg() != 0 {
		return c, fmt.Errorf("%s", editUsage)
	}
	bindSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "bind" {
			bindSet = true
		}
	})
	if len(uris) == 0 && *port == 0 && !bindSet {
		return c, fmt.Errorf("edit 需要 --uri、--port 或 --bind")
	}
	p := c.Proxies[id-1]
	if len(uris) > 0 {
		parsed, err := parseProxyURIs(uris)
		if err != nil {
			return c, err
		}
		parsed.LocalPort = p.LocalPort
		parsed.Listen = p.Listen
		p = parsed
	}
	if *port != 0 {
		p.LocalPort = *port
	}
	if bindSet {
		p.Listen = *bind
		if p.Listen == "" {
			p.Listen = "0.0.0.0"
		}
	}
	next := c
	next.Proxies = append([]Proxy{}, c.Proxies...)
	next.Proxies[id-1] = p
	if err := validateConfig(next); err != nil {
		return c, err
	}
	return next, nil
}

// uriList 实现 flag.Value，支持 --uri 重复指定多个 URI。
type uriList []string

func (l *uriList) String() string { return strings.Join(*l, " ") }

func (l *uriList) Set(s string) error {
	*l = append(*l, s)
	return nil
}

func targetText(p Proxy) string {
	s := fmt.Sprintf("%s:%d", p.Address, p.Port)
	for _, h := range p.Chain {
		s += " -> " + fmt.Sprintf("%s:%d", h.Address, h.Port)
	}
	return s
}

func formatProxyList(c AppConfig, results []probeResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-4s %-8s %-5s %-11s %-8s %-24s %s\n", "ID", "TYPE", "PORT", "BIND", "LATENCY", "TARGET", "NAME")
	for i, p := range c.Proxies {
		lat := "-"
		if i < len(results) {
			lat = latencyText(results[i])
		}
		fmt.Fprintf(&b, "%-4d %-8s %-5d %-11s %-8s %-24s %s\n", i+1, p.Type, p.LocalPort, socksListen(c, p), lat, targetText(p), p.Name)
	}
	return b.String()
}

func runTUI(a *app, in io.Reader, out io.Writer) error {
	fmt.Fprintln(out, manageUsage)
	fmt.Fprintln(out, "  quit")
	sc := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		switch args[0] {
		case "quit", "exit":
			return sc.Err()
		case "help":
			fmt.Fprintln(out, manageUsage)
			continue
		}
		next, text, err := runManage(a.config, args)
		if err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		if args[0] != "list" && args[0] != "test" {
			a.config = next
			if err := a.saveLocked(); err != nil {
				return err
			}
			if pidAlive(a.pidFile()) {
				fmt.Fprintln(out, "Xray 正在运行，执行 x2socks serve 应用新配置")
			}
		}
		fmt.Fprint(out, text)
	}
	return sc.Err()
}

func runCommand(file string, args []string) error {
	a, err := newApp(file)
	if err != nil {
		return err
	}
	next, text, err := runManage(a.config, args)
	if err != nil {
		return err
	}
	if args[0] != "list" && args[0] != "test" {
		a.config = next
		if err := a.saveLocked(); err != nil {
			return err
		}
		fmt.Print(text)
		if pidAlive(a.pidFile()) {
			fmt.Println("Xray 正在运行，执行 x2socks serve 应用新配置")
		}
		return nil
	}
	fmt.Print(text)
	return nil
}
