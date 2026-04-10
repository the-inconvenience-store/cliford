package codegen

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	regionStartPrefix = "// --- CUSTOM CODE START: "
	regionEndPrefix   = "// --- CUSTOM CODE END: "
	regionSuffix      = " ---"
)

// Region represents a custom code region extracted from a file.
type Region struct {
	Name    string
	Content string
	File    string
}

// ExtractRegions scans a file and returns all custom code regions found.
func ExtractRegions(filePath string) ([]Region, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	var regions []Region
	var current *Region
	var contentLines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if name, ok := parseRegionStart(line); ok {
			current = &Region{Name: name, File: filePath}
			contentLines = nil
			continue
		}

		if _, ok := parseRegionEnd(line); ok && current != nil {
			current.Content = strings.Join(contentLines, "\n")
			regions = append(regions, *current)
			current = nil
			contentLines = nil
			continue
		}

		if current != nil {
			contentLines = append(contentLines, line)
		}
	}

	return regions, scanner.Err()
}

// ExtractAllRegions scans all Go files in a directory tree and returns
// all custom code regions found, keyed by "filepath:regionname".
func ExtractAllRegions(dir string) (map[string]Region, error) {
	regions := make(map[string]Region)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}

		fileRegions, err := ExtractRegions(path)
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(dir, path)
		for _, r := range fileRegions {
			key := relPath + ":" + r.Name
			regions[key] = r
		}
		return nil
	})

	return regions, err
}

// InjectRegions takes generated file content and replaces empty custom code
// regions with previously extracted content. Returns the modified content
// and a list of orphaned region names (regions in old code but not in new).
func InjectRegions(content string, regions map[string]Region, relPath string) (string, []string) {
	var result strings.Builder
	var orphanedCheck = make(map[string]bool)

	// Track which regions from the old file are for this path
	for key := range regions {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && parts[0] == relPath {
			orphanedCheck[parts[1]] = true
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		result.WriteString(line + "\n")

		if name, ok := parseRegionStart(line); ok {
			key := relPath + ":" + name
			if r, found := regions[key]; found && strings.TrimSpace(r.Content) != "" {
				result.WriteString(r.Content + "\n")
			}
			delete(orphanedCheck, name)
		}
	}

	var orphaned []string
	for name := range orphanedCheck {
		orphaned = append(orphaned, name)
	}

	return result.String(), orphaned
}

// RegionMarkers generates the start and end markers for a custom code region.
func RegionMarkers(name string) (start, end string) {
	return regionStartPrefix + name + regionSuffix,
		regionEndPrefix + name + regionSuffix
}

func parseRegionStart(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, regionStartPrefix) && strings.HasSuffix(trimmed, regionSuffix) {
		name := trimmed[len(regionStartPrefix) : len(trimmed)-len(regionSuffix)]
		return name, true
	}
	return "", false
}

func parseRegionEnd(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, regionEndPrefix) && strings.HasSuffix(trimmed, regionSuffix) {
		name := trimmed[len(regionEndPrefix) : len(trimmed)-len(regionSuffix)]
		return name, true
	}
	return "", false
}
