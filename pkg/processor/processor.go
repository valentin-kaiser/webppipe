// Package processor implements the scan + convert pipeline.
package processor

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/valentin-kaiser/go-core/apperror"
	"golang.org/x/sync/errgroup"

	"github.com/valentin-kaiser/webppipe/pkg/config"
)

// Stats summarises a processor run.
type Stats struct {
	Scanned     int
	Converted   int
	Skipped     int
	Failed      int
	BytesBefore int64
	BytesAfter  int64
	// ChangedPaths lists repository-relative paths that were created or
	// removed and therefore need to be staged in git.
	ChangedPaths []string
}

// Run scans the configured repo path and converts matching images concurrently.
// Per-file errors are logged but never abort the run; only ctx cancellation
// does. Stats is always returned, even on error.
func Run(ctx context.Context, cfg *config.Config) (*Stats, error) {
	files, err := Scan(cfg.RepoPath, cfg.Include, cfg.Exclude)
	if err != nil {
		return &Stats{}, apperror.Wrap(err)
	}

	stats := &Stats{Scanned: len(files)}
	if len(files) == 0 {
		log.Info().Msg("no candidate images found")
		return stats, nil
	}
	log.Info().Int("candidates", len(files)).Msg("scanned repository")

	opts := EncodeOptions{
		Quality:   cfg.Quality,
		Lossless:  cfg.Lossless,
		MaxWidth:  cfg.MaxWidth,
		MaxHeight: cfg.MaxHeight,
	}

	var (
		mu          sync.Mutex
		converted   int64
		skipped     int64
		failed      int64
		bytesBefore int64
		bytesAfter  int64
		changed     []string
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Concurrency)

	for _, rel := range files {
		rel := rel
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			abs := filepath.Join(cfg.RepoPath, rel)
			res, err := ConvertFile(abs, opts, cfg.KeepOriginals, cfg.DryRun)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				log.Error().Err(err).Str("file", rel).Msg("conversion failed")
				return nil
			}
			if res.Skipped {
				atomic.AddInt64(&skipped, 1)
				log.Debug().Str("file", rel).Msg("already optimized, skipping")
				return nil
			}
			atomic.AddInt64(&converted, 1)
			atomic.AddInt64(&bytesBefore, res.SourceSize)
			atomic.AddInt64(&bytesAfter, res.TargetSize)

			relTarget := filepath.ToSlash(filepath.Join(filepath.Dir(rel), filepath.Base(targetWebPPath(rel))))
			mu.Lock()
			changed = append(changed, relTarget)
			if !cfg.KeepOriginals && filepath.Clean(rel) != filepath.Clean(relTarget) {
				changed = append(changed, rel)
			}
			mu.Unlock()

			if cfg.DryRun {
				log.Info().Str("source", rel).Str("target", relTarget).Msg("[dry-run] would convert")
			} else {
				log.Info().
					Str("source", rel).
					Str("target", relTarget).
					Int64("before", res.SourceSize).
					Int64("after", res.TargetSize).
					Msg("converted")
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		stats.Converted = int(converted)
		stats.Skipped = int(skipped)
		stats.Failed = int(failed)
		stats.BytesBefore = bytesBefore
		stats.BytesAfter = bytesAfter
		stats.ChangedPaths = changed
		return stats, apperror.Wrap(err)
	}

	stats.Converted = int(converted)
	stats.Skipped = int(skipped)
	stats.Failed = int(failed)
	stats.BytesBefore = bytesBefore
	stats.BytesAfter = bytesAfter
	stats.ChangedPaths = changed
	return stats, nil
}
