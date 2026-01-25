package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type testKeysFile struct {
	DefaultPolicy struct {
		AllowLocalhostWithoutAuth bool `yaml:"allow_localhost_without_auth"`
	} `yaml:"default_policy"`
	Projects map[string]struct {
		Keys []string `yaml:"keys"`
	} `yaml:"projects"`
}

func TestInitKeysFileCreatesProjectKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	key, err := InitKeysFile(path, "autarch")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if key == "" {
		t.Fatalf("expected generated key")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read keys file: %v", err)
	}
	var cfg testKeysFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	keys := cfg.Projects["autarch"].Keys
	if len(keys) == 0 || keys[0] != key {
		t.Fatalf("expected autarch key %q, got %+v", key, keys)
	}
}
