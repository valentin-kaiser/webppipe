// Command webppipe converts PNG/JPEG images in a Git repository to WebP and
// optionally commits and pushes the result. It is designed to run as a
// Docker-based GitHub Action but works equally well as a local CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	core "github.com/valentin-kaiser/go-core/config"
	"github.com/valentin-kaiser/go-core/flag"

	"github.com/valentin-kaiser/webppipe/pkg/config"
	"github.com/valentin-kaiser/webppipe/pkg/git"
	"github.com/valentin-kaiser/webppipe/pkg/processor"
)

func main() {
	if err := run(); err != nil {
		log.Error().Err(err).Msg("webppipe failed")
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Default()
	if err := config.Register(cfg); err != nil {
		return err
	}

	flag.Init()
	if flag.Help {
		flag.PrintHelp()
		return nil
	}

	// go-core/config uses flag.Path as the config directory and looks for
	// "<name>.yaml" inside it. Read() loads (or creates) the file and applies
	// it on top of registered defaults / env / flags.
	if err := core.Read(); err != nil {
		// A missing config file is fine; warn and continue with defaults.
		log.Warn().Err(err).Msg("could not read config file, using defaults / flags / env")
	}
	loaded, _ := core.Get().(*config.Config)
	if loaded != nil {
		cfg = loaded
	}

	level := zerolog.InfoLevel
	if flag.Debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if cfg.DryRun {
		// Don't perform git operations when nothing actually changes on disk.
		cfg.Git.Enabled = false
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stats, err := processor.Run(ctx, cfg)
	logSummary(stats)
	if err != nil {
		return err
	}

	if cfg.Git.Enabled && stats.Converted > 0 {
		if err := runGit(cfg, stats); err != nil {
			return err
		}
	}

	if stats.Failed > 0 {
		return fmt.Errorf("%d file(s) failed to convert", stats.Failed)
	}
	return nil
}

func runGit(cfg *config.Config, stats *processor.Stats) error {
	gc := &git.Client{Dir: cfg.RepoPath}
	if !gc.IsRepo() {
		log.Warn().Str("dir", cfg.RepoPath).Msg("not a git repository, skipping commit")
		return nil
	}
	if err := gc.Configure(cfg.Git.AuthorName, cfg.Git.AuthorEmail); err != nil {
		return err
	}
	if err := gc.AddPaths(stats.ChangedPaths); err != nil {
		return err
	}
	hasChanges, err := gc.HasChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		log.Info().Msg("no staged changes after add, nothing to commit")
		return nil
	}
	if err := gc.Commit(cfg.Git.CommitMessage); err != nil {
		return err
	}
	log.Info().Str("message", cfg.Git.CommitMessage).Msg("committed changes")
	if cfg.Git.Push {
		if err := gc.Push(cfg.Git.Branch); err != nil {
			return err
		}
		log.Info().Msg("pushed to remote")
	}
	return nil
}

func logSummary(s *processor.Stats) {
	if s == nil {
		return
	}
	saved := s.BytesBefore - s.BytesAfter
	log.Info().
		Int("scanned", s.Scanned).
		Int("converted", s.Converted).
		Int("skipped", s.Skipped).
		Int("failed", s.Failed).
		Int64("bytes_before", s.BytesBefore).
		Int64("bytes_after", s.BytesAfter).
		Int64("bytes_saved", saved).
		Msg("run summary")
}
