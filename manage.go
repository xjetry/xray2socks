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
		uri, port, bind, err := parseAddArgs(args[1:])
		if err != nil {
			return c, "", err
		}
		next, err := addProxy(c, uri, port, bind)
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
		if len(args) != 2 {
			return c, "", fmt.Errorf("用法: test {uri}")
		}
		p, err := parseProxyURI(args[1])
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
  add {uri} [port] [bind]
  remove {id}
  edit {id} [--uri URI] [--port PORT] [--bind ADDR]
  list
  test '{uri}'`

func parseAddArgs(args []string) (uri string, port int, bind string, err error) {
	if len(args) < 1 || len(args) > 3 {
		return "", 0, "", fmt.Errorf("用法: add {uri} [port] [bind]")
	}
	uri = args[0]
	bind = "0.0.0.0"
	switch len(args) {
	case 2:
		if isPortToken(args[1]) {
			port, _ = strconv.Atoi(args[1])
		} else {
			bind = args[1]
		}
	case 3:
		port, err = strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			return "", 0, "", fmt.Errorf("端口无效")
		}
		bind = args[2]
	}
	return uri, port, bind, nil
}

func parseID(raw string, n int) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 || id > n {
		return 0, fmt.Errorf("id 无效")
	}
	return id, nil
}

func addProxy(c AppConfig, uri string, port int, bind string) (AppConfig, error) {
	p, err := parseProxyURI(uri)
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

func editProxy(c AppConfig, args []string) (AppConfig, error) {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	uri := fs.String("uri", "", "")
	port := fs.Int("port", 0, "")
	bind := fs.String("bind", "", "")
	editUsage := "用法: edit {id} [--uri URI] [--port PORT] [--bind ADDR]"
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
	if *uri == "" && *port == 0 && !bindSet {
		return c, fmt.Errorf("edit 需要 --uri、--port 或 --bind")
	}
	p := c.Proxies[id-1]
	if *uri != "" {
		parsed, err := parseProxyURI(*uri)
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

func formatProxyList(c AppConfig, results []probeResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-4s %-8s %-5s %-11s %-8s %-24s %s\n", "ID", "TYPE", "PORT", "BIND", "LATENCY", "TARGET", "NAME")
	for i, p := range c.Proxies {
		lat := "-"
		if i < len(results) {
			lat = latencyText(results[i])
		}
		fmt.Fprintf(&b, "%-4d %-8s %-5d %-11s %-8s %-24s %s\n", i+1, p.Type, p.LocalPort, socksListen(c, p), lat, fmt.Sprintf("%s:%d", p.Address, p.Port), p.Name)
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
			if err := afterMutate(a); err != nil {
				fmt.Fprintln(out, err)
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
		return afterMutate(a)
	}
	fmt.Print(text)
	return nil
}
