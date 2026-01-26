package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultKeysFile = "intermute.keys.yaml"

type keysFile struct {
	DefaultPolicy struct {
		AllowLocalhostWithoutAuth *bool `yaml:"allow_localhost_without_auth"`
	} `yaml:"default_policy"`
	Projects map[string]projectKeys `yaml:"projects"`
}

type projectKeys struct {
	Keys []string `yaml:"keys"`
}

type Keyring struct {
	AllowLocalhostWithoutAuth bool
	keyToProject              map[string]string
}

func ResolveKeysPath() string {
	if v := strings.TrimSpace(os.Getenv("INTERMUTE_KEYS_FILE")); v != "" {
		return v
	}
	return filepath.Join(".", defaultKeysFile)
}

func LoadKeyringFromEnv() (*Keyring, error) {
	return LoadKeyring(ResolveKeysPath())
}

func LoadKeyring(path string) (*Keyring, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultKeyring(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, err := BootstrapDevKey(path, "dev"); err != nil {
				return nil, fmt.Errorf("bootstrap dev key: %w", err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read keys file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("read keys file: %w", err)
		}
	}
	var cfg keysFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse keys file: %w", err)
	}
	ring := &Keyring{
		AllowLocalhostWithoutAuth: true,
		keyToProject:              make(map[string]string),
	}
	if cfg.DefaultPolicy.AllowLocalhostWithoutAuth != nil {
		ring.AllowLocalhostWithoutAuth = *cfg.DefaultPolicy.AllowLocalhostWithoutAuth
	}
	for project, keys := range cfg.Projects {
		for _, key := range keys.Keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if existing, ok := ring.keyToProject[key]; ok && existing != project {
				return nil, fmt.Errorf("key reused across projects: %q", key)
			}
			ring.keyToProject[key] = project
		}
	}
	return ring, nil
}

func defaultKeyring() *Keyring {
	return &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: make(map[string]string)}
}

func NewKeyring(allowLocalhost bool, keyToProject map[string]string) *Keyring {
	clone := make(map[string]string, len(keyToProject))
	for k, v := range keyToProject {
		clone[k] = v
	}
	return &Keyring{AllowLocalhostWithoutAuth: allowLocalhost, keyToProject: clone}
}

func (k *Keyring) ProjectForKey(key string) (string, bool) {
	if k == nil {
		return "", false
	}
	project, ok := k.keyToProject[key]
	return project, ok
}
