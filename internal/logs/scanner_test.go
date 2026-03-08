package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()

	files := map[string]bool{
		"app.log":      false, // plain log
		"app.log.1.gz": true,  // compressed
		"app.log.gz":   true,  // compressed no number
		"app.log.2.gz": true,  // compressed
		"README.md":    false, // should be ignored
		"data.txt":     false, // should be ignored
	}

	for name := range files {
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
		got[lf.Name] = lf.Compressed
	}

	for name, wantCompressed := range files {
		if name == "README.md" || name == "data.txt" {
			if _, ok := got[name]; ok {
				t.Errorf("unexpected file in results: %s", name)
			}
			continue
		}
		c, ok := got[name]
		if !ok {
			t.Errorf("missing file in results: %s", name)
			continue
		}
		if c != wantCompressed {
			t.Errorf("%s: compressed=%v, want %v", name, c, wantCompressed)
		}
	}
}

func TestIsLogFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"app.log", true},
		{"redis.log", true},
		{"redis.log.1.gz", true},
		{"redis.log.gz", true},
		{"redis.log.10.gz", true},
		{"app.txt", false},
		{"README.md", false},
		{"logfile", false},
		{".gz", false},
	}

	for _, c := range cases {
		got := isLogFile(c.name)
		if got != c.want {
			t.Errorf("isLogFile(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
