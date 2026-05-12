package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/perfectscale/poc-cli/internal/config"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
)

var ErrProfileNotFound = errors.New("profile not found")

type notFoundError struct {
	name string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("profile %q not found; run `%s auth login` first", e.name, config.BinaryName)
}

func (e *notFoundError) Unwrap() error {
	return ErrProfileNotFound
}

type Store struct {
	baseDir string
}

func NewStore(baseDir string) (*Store, error) {
	if strings.TrimSpace(baseDir) == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user config directory: %w", err)
		}
		baseDir = filepath.Join(configDir, "perfectscale-cli", "profiles")
	}

	if err := os.MkdirAll(baseDir, dirPerm); err != nil {
		return nil, fmt.Errorf("create profile directory: %w", err)
	}

	return &Store{baseDir: baseDir}, nil
}

func (s *Store) Path(name string) string {
	return filepath.Join(s.baseDir, sanitizeName(name)+".json")
}

func (s *Store) Load(name string) (*Data, error) {
	body, err := os.ReadFile(s.Path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &notFoundError{name: name}
		}
		return nil, fmt.Errorf("read profile %q: %w", name, err)
	}

	var data Data
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode profile %q: %w", name, err)
	}
	if data.Name == "" {
		data.Name = name
	}

	return &data, nil
}

func (s *Store) Save(data *Data) error {
	if data == nil {
		return fmt.Errorf("profile data is required")
	}
	if data.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	if err := os.MkdirAll(s.baseDir, dirPerm); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}

	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile %q: %w", data.Name, err)
	}

	if err := os.WriteFile(s.Path(data.Name), body, filePerm); err != nil {
		return fmt.Errorf("write profile %q: %w", data.Name, err)
	}

	return nil
}

func (s *Store) Delete(name string) error {
	if err := os.Remove(s.Path(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete profile %q: %w", name, err)
	}

	return nil
}

func sanitizeName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "default"
	}
	clean = strings.ReplaceAll(clean, string(filepath.Separator), "_")
	return clean
}
