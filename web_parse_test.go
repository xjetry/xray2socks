package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postParse(t *testing.T, body string) (int, Proxy) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/parse", strings.NewReader(body))
	rec := httptest.NewRecorder()
	parseHandler(rec, req)
	var p Proxy
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return rec.Code, p
}

func TestParseHandlerChain(t *testing.T) {
	code, p := postParse(t, `{"uris":["`+testTrojan+`","`+testSS+`"]}`)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if p.Type != "trojan" || len(p.Chain) != 1 || p.Chain[0].Type != "ss" {
		t.Fatalf("parsed = %+v", p)
	}
	if p.Chain[0].LocalPort != 0 {
		t.Fatalf("hop localPort should be zeroed: %+v", p.Chain[0])
	}
}

func TestParseHandlerSplitsURIField(t *testing.T) {
	code, p := postParse(t, `{"uri":"`+testTrojan+` `+testSS+`"}`)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if len(p.Chain) != 1 || p.Chain[0].Address != "ss.example" {
		t.Fatalf("whitespace-separated uri should chain: %+v", p)
	}
}

func TestParseHandlerEmpty(t *testing.T) {
	if code, _ := postParse(t, `{}`); code != 400 {
		t.Fatalf("empty body status = %d", code)
	}
}
