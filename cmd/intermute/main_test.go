package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommandCreatesKey(t *testing.T) {
	tmp := t.TempDir()
	keyPath := filepath.Join(tmp, "intermute.keys.yaml")

	cmd := initCmd()
	cmd.SetArgs([]string{"--project", "demo", "--keys-file", keyPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute init: %v", err)
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read keys file: %v", err)
	}
	if !bytes.Contains(data, []byte("demo")) {
		t.Fatalf("expected project section to be written")
	}
}
