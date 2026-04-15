package decrypt

import (
	"path/filepath"
	"testing"
)

func TestGetSimpleDBFileSupportsDarwinV4(t *testing.T) {
	want := filepath.Join("db_storage", "message", "message_0.db")

	if got := GetSimpleDBFile("darwin", 4); got != want {
		t.Fatalf("expected darwin db file %q, got %q", want, got)
	}

	if got := GetSimpleDBFile("windows", 4); got != want {
		t.Fatalf("expected windows db file %q, got %q", want, got)
	}
}
