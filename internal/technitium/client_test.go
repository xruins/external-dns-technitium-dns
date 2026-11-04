package technitium

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/xruins/external-dns-technitium-dns/internal/config"
)

func newTestClient(baseURL string, zones ...string) *Client {
	c, _ := NewClient(&config.Config{TechnitiumURL: baseURL, Zones: zones})
	return c
}

func TestZoneFor_longestSuffixMatch(t *testing.T) {
	c := newTestClient("", "example.com", "a.example.com")

	cases := map[string]string{
		"example.com":         "example.com",
		"www.example.com":     "example.com",
		"a.example.com":       "a.example.com",
		"svc.a.example.com":   "a.example.com",
		"SVC.A.Example.COM.":  "a.example.com",
		"ba.example.com":      "example.com",
		"x.ba.example.com":    "example.com",
		"a-svc.a.example.com": "a.example.com",
	}
	for name, want := range cases {
		got, err := c.zoneFor(name)
		if err != nil {
			t.Errorf("zoneFor(%q): unexpected error: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("zoneFor(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestZoneFor_noMatch(t *testing.T) {
	c := newTestClient("", "example.com")
	for _, name := range []string{"example.org", "notexample.com"} {
		if _, err := c.zoneFor(name); err == nil {
			t.Errorf("zoneFor(%q): expected error", name)
		}
	}
}

type fakeTechnitium struct {
	mu      sync.Mutex
	records map[string][]Record
	calls   []map[string]string
}

func (f *fakeTechnitium) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	call := map[string]string{"path": r.URL.Path}
	for k := range q {
		call[k] = q.Get(k)
	}
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	resp := map[string]any{"status": "ok"}
	if r.URL.Path == "/api/zones/records/get" {
		resp["response"] = map[string]any{"records": f.records[q.Get("zone")]}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func TestGetAllRecords_mergesZonesAndSkipsShadowed(t *testing.T) {
	fake := &fakeTechnitium{records: map[string][]Record{
		"example.com": {
			{Name: "www.example.com", Type: "A", RData: json.RawMessage(`{"ipAddress":"1.1.1.1"}`)},
			{Name: "svc.a.example.com", Type: "A", RData: json.RawMessage(`{"ipAddress":"9.9.9.9"}`)},
		},
		"a.example.com": {
			{Name: "svc.a.example.com", Type: "A", RData: json.RawMessage(`{"ipAddress":"2.2.2.2"}`)},
		},
	}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	c := newTestClient(srv.URL, "example.com", "a.example.com")
	records, err := c.GetAllRecords(context.Background())
	if err != nil {
		t.Fatalf("GetAllRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(records), records)
	}
	got := map[string]string{}
	for _, r := range records {
		target, _ := ExtractTarget(r.Type, r.RData)
		got[r.Name] = target
	}
	if got["www.example.com"] != "1.1.1.1" || got["svc.a.example.com"] != "2.2.2.2" {
		t.Errorf("unexpected records: %v", got)
	}
}

func TestAddAndDeleteRecord_useOwningZone(t *testing.T) {
	fake := &fakeTechnitium{}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	c := newTestClient(srv.URL, "example.com", "a.example.com")
	ctx := context.Background()
	if err := c.AddRecord(ctx, "svc.a.example.com", "TXT", "heritage=external-dns", 0, "c", false); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if err := c.DeleteRecord(ctx, "www.example.com", "A", "1.1.1.1"); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if err := c.AddRecord(ctx, "www.example.org", "A", "1.1.1.1", 0, "c", false); err == nil {
		t.Errorf("AddRecord outside managed zones: expected error")
	}

	if len(fake.calls) != 2 {
		t.Fatalf("expected 2 API calls, got %d: %v", len(fake.calls), fake.calls)
	}
	if fake.calls[0]["zone"] != "a.example.com" {
		t.Errorf("add: zone = %q, want a.example.com", fake.calls[0]["zone"])
	}
	if fake.calls[1]["zone"] != "example.com" {
		t.Errorf("delete: zone = %q, want example.com", fake.calls[1]["zone"])
	}
}
