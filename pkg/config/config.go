// Package config defines the webppipe configuration schema and registers it
// with go-core/config so values can be supplied via YAML file, CLI flags
// (auto-generated from struct tags) and WEBPPIPE_* environment variables.
package config

import (
	"fmt"
	"runtime"

	corecfg "github.com/valentin-kaiser/go-core/config"
)

// Config is the top-level webppipe configuration.
type Config struct {
	// Quality controls the WebP encoder quality (1-100). Ignored when Lossless is true.
	Quality int `yaml:"quality" usage:"WebP quality (1-100)"`
	// Lossless enables lossless WebP encoding.
	Lossless bool `yaml:"lossless" usage:"Use lossless WebP encoding"`
	// MaxWidth, when > 0, scales images so width does not exceed this value.
	MaxWidth int `yaml:"max-width" usage:"Maximum image width in pixels (0 = unlimited)"`
	// MaxHeight, when > 0, scales images so height does not exceed this value.
	MaxHeight int `yaml:"max-height" usage:"Maximum image height in pixels (0 = unlimited)"`
	// Include is a list of doublestar glob patterns matched against
	// repository-relative paths. A file is processed only if it matches at
	// least one Include pattern.
	Include []string `yaml:"include" usage:"Glob patterns of files to include"`
	// Exclude is a list of doublestar glob patterns. Matching files are skipped.
	Exclude []string `yaml:"exclude" usage:"Glob patterns of files to exclude"`
	// KeepOriginals retains the source PNG/JPEG files alongside the new .webp.
	// Replacement of the original is opt-in: set this to false (or pass
	// --replace-originals via the wrapper input) to delete the source after a
	// successful conversion.
	KeepOriginals bool `yaml:"keep-originals" usage:"Keep original files alongside generated .webp (set to false to replace)"`
	// DryRun reports planned conversions without writing or deleting any files.
	DryRun bool `yaml:"dry-run" usage:"Show changes without writing files"`
	// Concurrency is the number of parallel conversion workers.
	Concurrency int `yaml:"concurrency" usage:"Number of parallel conversion workers"`
	// RepoPath is the working directory (typically the repository root).
	RepoPath string `yaml:"repo-path" usage:"Path to the repository / working directory"`
	// Git contains git commit/push behaviour. Auto-disabled when DryRun is true.
	Git GitConfig `yaml:"git"`
}

// GitConfig configures the optional commit-and-push step.
type GitConfig struct {
	Enabled       bool   `yaml:"enabled" usage:"Commit and push converted files"`
	CommitMessage string `yaml:"commit-message" usage:"Commit message used when changes are committed"`
	AuthorName    string `yaml:"author-name" usage:"Git author name"`
	AuthorEmail   string `yaml:"author-email" usage:"Git author email"`
	Branch        string `yaml:"branch" usage:"Branch to push to (empty = current branch)"`
	Push          bool   `yaml:"push" usage:"Push the commit to the remote"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Quality:       80,
		Lossless:      false,
		MaxWidth:      0,
		MaxHeight:     0,
		Include:       []string{"**/*.png", "**/*.jpg", "**/*.jpeg", "**/*.PNG", "**/*.JPG", "**/*.JPEG"},
		Exclude:       []string{".git/**", "node_modules/**", "vendor/**"},
		KeepOriginals: true,
		DryRun:        false,
		Concurrency:   runtime.NumCPU(),
		RepoPath:      ".",
		Git: GitConfig{
			Enabled:       true,
			CommitMessage: "chore: optimize images to WebP",
			AuthorName:    "webppipe[bot]",
			AuthorEmail:   "webppipe@users.noreply.github.com",
			Branch:        "",
			Push:          true,
		},
	}
}

// Validate implements corecfg.Config.
func (c *Config) Validate() error {
	if c.Quality < 1 || c.Quality > 100 {
		return fmt.Errorf("quality must be between 1 and 100, got %d", c.Quality)
	}
	if c.MaxWidth < 0 {
		return fmt.Errorf("max-width must be >= 0, got %d", c.MaxWidth)
	}
	if c.MaxHeight < 0 {
		return fmt.Errorf("max-height must be >= 0, got %d", c.MaxHeight)
	}
	if c.Concurrency < 1 {
		return fmt.Errorf("concurrency must be >= 1, got %d", c.Concurrency)
	}
	if len(c.Include) == 0 {
		return fmt.Errorf("at least one include pattern is required")
	}
	if c.RepoPath == "" {
		return fmt.Errorf("repo-path must not be empty")
	}
	return nil
}

// Register registers the Config with go-core/config under the name "webppipe".
// Must be called before flag.Init() so that struct-derived flags are added.
func Register(cfg *Config) error {
	corecfg.Manager().WithName("webppipe")
	return corecfg.Manager().Register(cfg)
}
