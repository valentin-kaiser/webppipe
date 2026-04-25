package processor

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/valentin-kaiser/go-core/apperror"
)

// Scan walks root and returns the set of repository-relative file paths
// that match include and do not match exclude. The .git directory is always
// skipped to avoid touching git internals.
func Scan(root string, include, exclude []string) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, apperror.Wrap(err)
	}

	var matches []string
	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(abs, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if matchAny(exclude, rel) || matchAny(exclude, rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if matchAny(exclude, rel) {
			return nil
		}
		if !matchAny(include, rel) {
			return nil
		}
		matches = append(matches, rel)
		return nil
	})
	if walkErr != nil {
		return nil, apperror.Wrap(walkErr)
	}
	return matches, nil
}

func matchAny(patterns []string, p string) bool {
	for _, pat := range patterns {
		// Plain prefix folder forms like "node_modules/**" should also match
		// the directory itself ("node_modules") so WalkDir can prune.
		if ok, _ := doublestar.Match(pat, p); ok {
			return true
		}
		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if p == prefix {
				return true
			}
		}
	}
	return false
}
