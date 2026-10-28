package audit_test

import (
	"path/filepath"
	"testing"

	"github.com/Segners/PachinGo/internal/audit"
)

func TestLogAndTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := audit.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Log("check", "web-01", true, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := l.Log("upgrade", "web-01", false, "boom"); err != nil {
		t.Fatal(err)
	}
	events, err := l.Tail(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len=%d", len(events))
	}
	if events[1].Action != "upgrade" || events[1].Success {
		t.Fatalf("%+v", events[1])
	}
}
