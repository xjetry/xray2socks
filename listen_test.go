package main

import (
	"net"
	"strings"
	"testing"
)

func TestParseListenAddrs(t *testing.T) {
	got := parseListenAddrs("127.0.0.1,10.64.1.10,[2407:b9c0:f001:289:26a3:f0ff:fe4a:967f]")
	want := []string{"127.0.0.1", "10.64.1.10", "2407:b9c0:f001:289:26a3:f0ff:fe4a:967f"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v", got)
	}
	if parseListenAddrs("")[0] != "0.0.0.0" {
		t.Fatal("empty should default to 0.0.0.0")
	}
}

func TestNextFreePortSkipsUsedAndOccupied(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:1081")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	c := AppConfig{Proxies: []Proxy{{LocalPort: 1082, Listen: "127.0.0.1"}}}
	port, err := nextFreePort(c, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if port != 1083 {
		t.Fatalf("port = %d, want 1083", port)
	}
}
