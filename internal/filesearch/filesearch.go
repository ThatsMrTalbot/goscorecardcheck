package filesearch

import (
	"os"
	"path/filepath"
	"strings"
)

func Find(cwd string, targets []string, skipTests bool) ([]string, error) {
	// If there are no targets, default to all files
	if len(targets) == 0 {
		targets = []string{"./..."}
	}

	// First find all files
	var found []string
	for _, target := range targets {
		// If the directory has a golang "wildcard" expansion at the end
		// then we need to search all subdirectories
		if strings.HasSuffix(target, "/...") {
			expandedFileList, err := findFilesWithExtensionRecursive(filepath.Dir(target), "go")
			if err != nil {
				return nil, err
			}

			found = append(found, expandedFileList...)

			continue
		}

		// Stat the target, to check it A) exists and B) is a file or directory
		stat, err := os.Stat(target)
		if err != nil {
			return nil, err
		}

		// If the target is a directory, get golang files in it
		if stat.IsDir() {
			expandedFileList, err := findFilesWithExtension(target, "go")
			if err != nil {
				return nil, err
			}

			found = append(found, expandedFileList...)
		}

		// Target is a file, add it
		found = append(found, target)
	}

	// Filter out tests if desired
	var filtered []string
	for _, file := range found {
		if skipTests {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
		}

		if relativePath, err := filepath.Rel(cwd, file); err == nil {
			filtered = append(filtered, relativePath)
			continue
		}

		filtered = append(filtered, file)
	}

	return filtered, nil
}

func findFilesWithExtensionRecursive(root string, ext string) (found []string, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
		if strings.HasSuffix(info.Name(), "."+ext) {
			found = append(found, path)
		}

		return nil
	})

	return found, err
}

func findFilesWithExtension(root string, ext string) (found []string, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, "."+ext) {
			found = append(found, filepath.Join(root, name))
		}
	}

	return found, nil
}
