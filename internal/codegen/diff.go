package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiffResult holds the result of comparing current vs. new generation output.
type DiffResult struct {
	Files    []FileDiff
	HasDiff  bool
	Orphaned []string // Custom code regions that would be lost
}

// FileDiff represents the difference for a single file.
type FileDiff struct {
	Path     string
	Status   DiffStatus
	OldLines int
	NewLines int
	Diff     string // Unified diff output
}

// DiffStatus describes the change type for a file.
type DiffStatus string

const (
	DiffAdded    DiffStatus = "added"
	DiffModified DiffStatus = "modified"
	DiffRemoved  DiffStatus = "removed"
	DiffUnchanged DiffStatus = "unchanged"
)

// ComputeDiff compares the current output directory against a freshly
// generated temporary directory and produces a diff report.
func ComputeDiff(currentDir, newDir string) (*DiffResult, error) {
	result := &DiffResult{}

	// Collect files from both directories
	currentFiles := make(map[string]bool)
	filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(currentDir, path)
		if hasPathPrefix(rel, ".cliford") {
			return nil
		}
		currentFiles[rel] = true
		return nil
	})

	newFiles := make(map[string]bool)
	filepath.Walk(newDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(newDir, path)
		newFiles[rel] = true
		return nil
	})

	// Check new/modified files
	for rel := range newFiles {
		newContent, err := os.ReadFile(filepath.Join(newDir, rel))
		if err != nil {
			continue
		}

		if !currentFiles[rel] {
			result.Files = append(result.Files, FileDiff{
				Path:     rel,
				Status:   DiffAdded,
				NewLines: countLines(string(newContent)),
			})
			result.HasDiff = true
			continue
		}

		oldContent, err := os.ReadFile(filepath.Join(currentDir, rel))
		if err != nil {
			continue
		}

		if string(oldContent) == string(newContent) {
			result.Files = append(result.Files, FileDiff{
				Path:   rel,
				Status: DiffUnchanged,
			})
		} else {
			diff := simpleDiff(rel, string(oldContent), string(newContent))
			result.Files = append(result.Files, FileDiff{
				Path:     rel,
				Status:   DiffModified,
				OldLines: countLines(string(oldContent)),
				NewLines: countLines(string(newContent)),
				Diff:     diff,
			})
			result.HasDiff = true
		}
	}

	// Check removed files
	for rel := range currentFiles {
		if !newFiles[rel] {
			result.Files = append(result.Files, FileDiff{
				Path:   rel,
				Status: DiffRemoved,
			})
			result.HasDiff = true
		}
	}

	return result, nil
}

// FormatDiffReport produces a human-readable summary of the diff.
func FormatDiffReport(result *DiffResult) string {
	var sb strings.Builder

	if !result.HasDiff {
		sb.WriteString("No changes detected.\n")
		return sb.String()
	}

	var added, modified, removed, unchanged int
	for _, f := range result.Files {
		switch f.Status {
		case DiffAdded:
			added++
			sb.WriteString(fmt.Sprintf("  + %s (%d lines)\n", f.Path, f.NewLines))
		case DiffModified:
			modified++
			sb.WriteString(fmt.Sprintf("  ~ %s (%d -> %d lines)\n", f.Path, f.OldLines, f.NewLines))
		case DiffRemoved:
			removed++
			sb.WriteString(fmt.Sprintf("  - %s\n", f.Path))
		case DiffUnchanged:
			unchanged++
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d added, %d modified, %d removed, %d unchanged\n",
		added, modified, removed, unchanged))

	if len(result.Orphaned) > 0 {
		sb.WriteString("\nWarning: these custom code regions would be lost:\n")
		for _, name := range result.Orphaned {
			sb.WriteString(fmt.Sprintf("  ! %s\n", name))
		}
	}

	return sb.String()
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func simpleDiff(path, old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", path))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", path))

	// Simple line-by-line comparison (not a real unified diff algorithm,
	// but sufficient for previewing changes)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if i < len(oldLines) {
				sb.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if i < len(newLines) {
				sb.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	return sb.String()
}
