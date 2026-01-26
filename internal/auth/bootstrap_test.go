package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapDevKeyCreatesFile(t *testing.T) {
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "test-keys.yaml")

	result, err := BootstrapDevKey(keysPath, "myproject")
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if !result.Created {
		t.Fatalf("expected Created=true")
	}
	if result.Key == "" {
		t.Fatalf("expected non-empty key")
	}
	if result.Project != "myproject" {
		t.Fatalf("expected project=myproject, got %s", result.Project)
	}

	// Verify file was created
	if _, err := os.Stat(keysPath); err != nil {
		t.Fatalf("keys file not created: %v", err)
	}

	// Verify keyring can be loaded
	ring, err := LoadKeyring(keysPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	proj, ok := ring.ProjectForKey(result.Key)
	if !ok || proj != "myproject" {
		t.Fatalf("expected key to map to myproject, got %s ok=%v", proj, ok)
	}
}

func TestBootstrapDevKeySkipsExisting(t *testing.T) {
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "test-keys.yaml")

	// Create an existing file
	if err := os.WriteFile(keysPath, []byte("existing"), 0600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	result, err := BootstrapDevKey(keysPath, "myproject")
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if result.Created {
		t.Fatalf("expected Created=false for existing file")
	}

	// Verify file wasn't modified
	data, _ := os.ReadFile(keysPath)
	if string(data) != "existing" {
		t.Fatalf("file was modified")
	}
}

func TestBootstrapDevKeyDefaultProject(t *testing.T) {
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "test-keys.yaml")

	result, err := BootstrapDevKey(keysPath, "")
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if result.Project != "dev" {
		t.Fatalf("expected default project=dev, got %s", result.Project)
	}
}
