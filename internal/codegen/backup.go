package codegen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const maxBackups = 5

// Backup copies the current generated output to .cliford/backup/<timestamp>/
// before overwriting. Keeps only the last maxBackups backups.
type Backup struct {
	outputDir string
}

// NewBackup creates a backup manager for the given output directory.
func NewBackup(outputDir string) *Backup {
	return &Backup{outputDir: outputDir}
}

// Create makes a backup of all generated Go files in the output directory.
// Returns the backup directory path, or empty string if nothing to back up.
func (b *Backup) Create() (string, error) {
	backupRoot := filepath.Join(b.outputDir, ".cliford", "backup")

	// Check if there's anything to back up
	hasFiles := false
	filepath.Walk(b.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(b.outputDir, path)
		if hasPathPrefix(rel, ".cliford") {
			return nil
		}
		if filepath.Ext(path) == ".go" || filepath.Base(path) == "go.mod" || filepath.Base(path) == "go.sum" {
			hasFiles = true
			return filepath.SkipAll
		}
		return nil
	})

	if !hasFiles {
		return "", nil
	}

	// Create timestamped backup directory
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(backupRoot, timestamp)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	// Copy generated files
	err := filepath.Walk(b.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		rel, _ := filepath.Rel(b.outputDir, path)

		// Skip backup directory itself and non-generated files
		if hasPathPrefix(rel, ".cliford") {
			return filepath.SkipDir
		}
		if filepath.Ext(path) != ".go" && filepath.Base(path) != "go.mod" {
			return nil
		}

		destPath := filepath.Join(backupDir, rel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		return copyFile(path, destPath)
	})

	if err != nil {
		return "", fmt.Errorf("backup files: %w", err)
	}

	// Prune old backups
	if err := b.prune(backupRoot); err != nil {
		// Non-fatal — warn but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to prune old backups: %v\n", err)
	}

	return backupDir, nil
}

func (b *Backup) prune(backupRoot string) error {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}

	sort.Strings(dirs)

	// Remove oldest backups exceeding the limit
	for len(dirs) > maxBackups {
		oldest := dirs[0]
		if err := os.RemoveAll(filepath.Join(backupRoot, oldest)); err != nil {
			return err
		}
		dirs = dirs[1:]
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func hasPathPrefix(path, prefix string) bool {
	return path == prefix || len(path) > len(prefix) && path[:len(prefix)+1] == prefix+string(filepath.Separator)
}
