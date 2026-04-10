package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lockfile records the state of the last generation run.
type Lockfile struct {
	ClifordVersion string            `json:"clifordVersion"`
	SpecHash       string            `json:"specHash"`
	ConfigHash     string            `json:"configHash"`
	Timestamp      time.Time         `json:"timestamp"`
	Files          map[string]string `json:"files"` // relative path -> SHA256
}

// WriteLockfile creates or updates cliford.lock in the output directory.
func WriteLockfile(outputDir, specPath, configPath, clifordVersion string) error {
	lock := Lockfile{
		ClifordVersion: clifordVersion,
		Timestamp:      time.Now().UTC(),
		Files:          make(map[string]string),
	}

	// Hash spec
	if specPath != "" {
		h, err := hashFile(specPath)
		if err == nil {
			lock.SpecHash = h
		}
	}

	// Hash config
	if configPath != "" {
		h, err := hashFile(configPath)
		if err == nil {
			lock.ConfigHash = h
		}
	}

	// Hash generated files
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(outputDir, path)
		if rel == "cliford.lock" || hasLockPathPrefix(rel, ".cliford") {
			return nil
		}
		if filepath.Ext(path) == ".go" || filepath.Base(path) == "go.mod" {
			h, err := hashFile(path)
			if err == nil {
				lock.Files[rel] = h
			}
		}
		return nil
	})

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}

	lockPath := filepath.Join(outputDir, "cliford.lock")
	return os.WriteFile(lockPath, data, 0o644)
}

// ReadLockfile loads an existing cliford.lock.
func ReadLockfile(outputDir string) (*Lockfile, error) {
	lockPath := filepath.Join(outputDir, "cliford.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lockfile: %w", err)
	}

	return &lock, nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func hasLockPathPrefix(path, prefix string) bool {
	return path == prefix || (len(path) > len(prefix) && path[:len(prefix)+1] == prefix+string(filepath.Separator))
}
