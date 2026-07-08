package marketplace

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// lockFilePath returns the path to the marketplace lock file.
func lockFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kubectl-cwide", "marketplace.lock"), nil
}

// MarketplacePin records that a specific template was installed from a repo at a ref.
type MarketplacePin struct {
	Repo     string `yaml:"repo"`
	Resource string `yaml:"resource"`
	Template string `yaml:"template"`
	Ref      string `yaml:"ref"`
}

// LockFile is the on-disk format for marketplace.lock.
type LockFile struct {
	Pins []MarketplacePin `yaml:"pins"`
}

// LoadLockFile reads ~/.kubectl-cwide/marketplace.lock, returning an empty
// LockFile if it doesn't exist.
func LoadLockFile() (*LockFile, error) {
	path, err := lockFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{}, nil
		}
		return nil, err
	}
	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}
	return &lf, nil
}

// Save writes the lock file to disk, creating parent dirs as needed.
func (lf *LockFile) Save() error {
	path, err := lockFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(lf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert inserts or replaces a pin matching (repo, resource, template).
func (lf *LockFile) Upsert(p MarketplacePin) {
	for i, existing := range lf.Pins {
		if existing.Repo == p.Repo && existing.Resource == p.Resource && existing.Template == p.Template {
			lf.Pins[i] = p
			return
		}
	}
	lf.Pins = append(lf.Pins, p)
}
