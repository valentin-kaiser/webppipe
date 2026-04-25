package processor_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	xwebp "golang.org/x/image/webp"

	"github.com/valentin-kaiser/webppipe/pkg/config"
	"github.com/valentin-kaiser/webppipe/pkg/processor"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func baseCfg(dir string) *config.Config {
	c := config.Default()
	c.RepoPath = dir
	c.Concurrency = 2
	c.Git.Enabled = false
	return c
}

func TestRunConverts(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 32, 32)
	writePNG(t, filepath.Join(dir, "sub", "b.png"), 16, 16)

	cfg := baseCfg(dir)
	stats, err := processor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.Converted != 2 {
		t.Fatalf("expected 2 conversions, got %d", stats.Converted)
	}
	for _, p := range []string{"a.webp", filepath.Join("sub", "b.webp")} {
		data, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatalf("expected %s to exist: %v", p, err)
		}
		if _, err := xwebp.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("output %s is not valid webp: %v", p, err)
		}
	}
	// Originals are kept by default; replacement is opt-in.
	if _, err := os.Stat(filepath.Join(dir, "a.png")); err != nil {
		t.Fatalf("expected original to be kept by default, err=%v", err)
	}
}

func TestRunReplacesOriginals(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 8, 8)

	cfg := baseCfg(dir)
	cfg.KeepOriginals = false
	if _, err := processor.Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.png")); !os.IsNotExist(err) {
		t.Fatalf("expected original to be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.webp")); err != nil {
		t.Fatalf("expected webp to exist, err=%v", err)
	}
}

func TestRunIdempotent(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 8, 8)

	cfg := baseCfg(dir)
	cfg.KeepOriginals = true
	if _, err := processor.Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	stats, err := processor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Converted != 0 {
		t.Fatalf("expected 0 conversions on second run, got %d", stats.Converted)
	}
	if stats.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", stats.Skipped)
	}
}

func TestRunDryRun(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 8, 8)

	cfg := baseCfg(dir)
	cfg.DryRun = true
	stats, err := processor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Converted != 1 {
		t.Fatalf("expected 1 planned conversion, got %d", stats.Converted)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.webp")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write output, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.png")); err != nil {
		t.Fatalf("dry-run must keep source, err=%v", err)
	}
}

func TestRunResize(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "big.png"), 200, 100)

	cfg := baseCfg(dir)
	cfg.MaxWidth = 50
	if _, err := processor.Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "big.webp"))
	if err != nil {
		t.Fatal(err)
	}
	cfgImg, err := xwebp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if cfgImg.Width != 50 || cfgImg.Height != 25 {
		t.Fatalf("expected 50x25, got %dx%d", cfgImg.Width, cfgImg.Height)
	}
}

func TestRunExclude(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "keep.png"), 8, 8)
	writePNG(t, filepath.Join(dir, "vendor", "skip.png"), 8, 8)

	cfg := baseCfg(dir)
	stats, err := processor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Converted != 1 {
		t.Fatalf("expected 1 conversion, got %d", stats.Converted)
	}
	if _, err := os.Stat(filepath.Join(dir, "vendor", "skip.png")); err != nil {
		t.Fatalf("excluded file should remain, err=%v", err)
	}
}
