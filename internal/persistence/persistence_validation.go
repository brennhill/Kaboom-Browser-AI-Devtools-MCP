// Purpose: Validates namespace and key inputs against path traversal and separator injection.
// Why: Isolates input validation from CRUD operations to keep security checks centralized.
package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateStoreInput(value, label string) error {
	if value == "" {
		return nil
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("%s contains path traversal sequence", label)
	}
	if strings.ContainsRune(value, filepath.Separator) || strings.Contains(value, "/") {
		return fmt.Errorf("%s contains path separator", label)
	}
	return nil
}

func validatePathInDir(base, target string) error {
	cleanBase := filepath.Clean(base) + string(os.PathSeparator)
	cleanTarget := filepath.Clean(target)
	if !strings.HasPrefix(cleanTarget, cleanBase) {
		return fmt.Errorf("path escapes project directory")
	}
	return nil
}

func (s *SessionStore) validatedNsDir(namespace string) (string, error) {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return "", err
	}
	nsDir := filepath.Join(s.projectDir, namespace)
	if err := validatePathInDir(s.projectDir, nsDir); err != nil {
		return "", err
	}
	return nsDir, nil
}

func (s *SessionStore) validatedPath(namespace, key string) (nsDir, filePath string, err error) {
	nsDir, err = s.validatedNsDir(namespace)
	if err != nil {
		return "", "", err
	}
	if err := validateStoreInput(key, "key"); err != nil {
		return "", "", err
	}
	filePath = filepath.Join(nsDir, key+".json")
	if err := validatePathInDir(s.projectDir, filePath); err != nil {
		return "", "", err
	}
	return nsDir, filePath, nil
}

func jsonKeysFromDir(files sessionFilesystem, dir string) ([]string, error) {
	entries, err := files.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: a namespace directory does not exist until its
			// first key is persisted.
			return []string{}, nil
		}
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".json") {
			keys = append(keys, strings.TrimSuffix(name, ".json"))
		}
	}
	if keys == nil {
		keys = []string{}
	}
	return keys, nil
}
