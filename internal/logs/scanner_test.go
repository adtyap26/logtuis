package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()

	// name → want included
	cases := map[string]bool{
		"app.log":      true,
		"app.log.1.gz": true,
		"app.log.gz":   true,
		"app.log.2.gz": true,
		"server.log.2": true,
		"server.log.10": true,
		"notes.txt":    true,
		"README.md":    false,
		"data.csv":     false,
		"binary":       false,
	}

	for name := range cases {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	got := map[string]bool{}
	for _, lf := range results {
		got[lf.Name] = true
	}

	for name, wantIncluded := range cases {
		_, found := got[name]
		if found != wantIncluded {
			t.Errorf("%s: included=%v, want %v", name, found, wantIncluded)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name           string
		wantMatch      bool
		wantCompressed bool
	}{
		{"app.log", true, false},
		{"redis.log", true, false},
		{"notes.txt", true, false},
		{"server.log.2", true, false},
		{"server.log.10", true, false},
		{"redis.log.1.gz", true, true},
		{"redis.log.gz", true, true},
		{"app.txt", true, false},
		{"README.md", false, false},
		{"data.csv", false, false},
		{"logfile", false, false},
		{".gz", false, false},
		{"server.log.abc", false, false}, // non-digit suffix, not a rotated log
	}

	for _, c := range cases {
		match, compressed := classify(c.name)
		if match != c.wantMatch {
			t.Errorf("classify(%q) match=%v, want %v", c.name, match, c.wantMatch)
		}
		if compressed != c.wantCompressed {
			t.Errorf("classify(%q) compressed=%v, want %v", c.name, compressed, c.wantCompressed)
		}
	}
}

func TestIsRotatedLog(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"server.log.2", true},
		{"server.log.10", true},
		{"app.log.1", true},
		{"app.log.abc", false},
		{"app.log", false},
		{"app.log.1.gz", false}, // gz handled separately
		{"server.log.", false},  // trailing dot, no digit
	}

	for _, c := range cases {
		got := isRotatedLog(c.name)
		if got != c.want {
			t.Errorf("isRotatedLog(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
