package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Port != 8639 {
		t.Errorf("expected default port 8639, got %d", cfg.Port)
	}
	if cfg.Verbose {
		t.Error("expected verbose to be false by default")
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("port: 9000\nverbose: true\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if !cfg.Verbose {
		t.Error("expected verbose to be true")
	}
}

func TestLoadFromYAMLFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestMerge(t *testing.T) {
	base := Config{Port: 8639, Verbose: false}
	override := Config{Port: 9000}

	merged := Merge(base, override)
	if merged.Port != 9000 {
		t.Errorf("expected merged port 9000, got %d", merged.Port)
	}
}
