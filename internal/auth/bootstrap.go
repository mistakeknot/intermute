package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BootstrapResult contains info about a bootstrapped dev key.
type BootstrapResult struct {
	KeysFile string
	Project  string
	Key      string
	Created  bool
}

// BootstrapDevKey checks if the keys file exists. If not, it creates one
// with a dev key for the specified project. This helps developers get started
// quickly without manual setup.
func BootstrapDevKey(keysPath, project string) (*BootstrapResult, error) {
	if keysPath == "" {
		keysPath = ResolveKeysPath()
	}
	if project == "" {
		project = "dev"
	}

	// Check if file exists
	if _, err := os.Stat(keysPath); err == nil {
		// File exists, no bootstrap needed
		return &BootstrapResult{KeysFile: keysPath, Created: false}, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("check keys file: %w", err)
	}

	// Generate a new dev key
	key, err := generateDevKey()
	if err != nil {
		return nil, err
	}

	// Create the keys file
	cfg := keysFile{
		Projects: map[string]projectKeys{
			project: {Keys: []string{key}},
		},
	}
	allowLocalhost := true
	cfg.DefaultPolicy.AllowLocalhostWithoutAuth = &allowLocalhost

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal keys file: %w", err)
	}

	if err := os.WriteFile(keysPath, data, 0600); err != nil {
		return nil, fmt.Errorf("write keys file: %w", err)
	}

	return &BootstrapResult{
		KeysFile: keysPath,
		Project:  project,
		Key:      key,
		Created:  true,
	}, nil
}

func generateDevKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
