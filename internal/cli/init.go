package cli

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type keysFile struct {
	DefaultPolicy struct {
		AllowLocalhostWithoutAuth *bool `yaml:"allow_localhost_without_auth"`
	} `yaml:"default_policy"`
	Projects map[string]projectKeys `yaml:"projects"`
}

type projectKeys struct {
	Keys []string `yaml:"keys"`
}

func InitKeysFile(path, project string) (string, error) {
	path = strings.TrimSpace(path)
	project = strings.TrimSpace(project)
	if path == "" {
		return "", fmt.Errorf("keys file path required")
	}
	if project == "" {
		return "", fmt.Errorf("project required")
	}

	cfg, err := loadKeysFile(path)
	if err != nil {
		return "", err
	}
	if cfg.Projects == nil {
		cfg.Projects = make(map[string]projectKeys)
	}
	key, err := generateKey()
	if err != nil {
		return "", err
	}
	pk := cfg.Projects[project]
	pk.Keys = append(pk.Keys, key)
	cfg.Projects[project] = pk
	if cfg.DefaultPolicy.AllowLocalhostWithoutAuth == nil {
		val := true
		cfg.DefaultPolicy.AllowLocalhostWithoutAuth = &val
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", fmt.Errorf("marshal keys file: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("write keys file: %w", err)
	}
	return key, nil
}

func loadKeysFile(path string) (keysFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return keysFile{}, nil
		}
		return keysFile{}, fmt.Errorf("read keys file: %w", err)
	}
	var cfg keysFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return keysFile{}, fmt.Errorf("parse keys file: %w", err)
	}
	return cfg, nil
}

func generateKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
