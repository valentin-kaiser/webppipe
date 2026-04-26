package processor

import (
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/valentin-kaiser/go-core/apperror"
	"golang.org/x/image/draw"
)

// targetWebPPath returns the .webp companion path for src.
func targetWebPPath(src string) string {
	ext := filepath.Ext(src)
	return strings.TrimSuffix(src, ext) + ".webp"
}

// alreadyOptimized returns true when a sibling .webp already exists with a
// modification time greater or equal to the source's.
func alreadyOptimized(src, dst string) (bool, error) {
	si, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	di, err := os.Stat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !di.ModTime().Before(si.ModTime()), nil
}

// resizeIfNeeded scales img to fit within maxW/maxH preserving aspect ratio.
// If both bounds are 0 or the image already fits, the original is returned.
func resizeIfNeeded(img image.Image, maxW, maxH int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if (maxW <= 0 || w <= maxW) && (maxH <= 0 || h <= maxH) {
		return img
	}

	scaleW := 1.0
	scaleH := 1.0
	if maxW > 0 && w > maxW {
		scaleW = float64(maxW) / float64(w)
	}
	if maxH > 0 && h > maxH {
		scaleH = float64(maxH) / float64(h)
	}
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// EncodeOptions describes WebP encoder behaviour.
type EncodeOptions struct {
	Quality   int
	Lossless  bool
	MaxWidth  int
	MaxHeight int
}

// ConvertResult holds per-file outcome details.
type ConvertResult struct {
	Source     string
	Target     string
	Skipped    bool
	SourceSize int64
	TargetSize int64
}

// ConvertFile decodes src, optionally resizes, and writes a .webp next to it.
// When dryRun is true no files are written or deleted; idempotency is still
// reported so dry-run output reflects what a real run would do.
func ConvertFile(src string, opts EncodeOptions, keepOriginal, dryRun bool) (ConvertResult, error) {
	dst := targetWebPPath(src)
	res := ConvertResult{Source: src, Target: dst}

	skip, err := alreadyOptimized(src, dst)
	if err != nil {
		return res, apperror.Wrap(err)
	}
	if skip {
		res.Skipped = true
		return res, nil
	}

	si, err := os.Stat(src)
	if err != nil {
		return res, apperror.Wrap(err)
	}
	res.SourceSize = si.Size()

	if dryRun {
		return res, nil
	}

	f, err := os.Open(src) //nolint:gosec // src comes from a controlled scan of the working tree
	if err != nil {
		return res, apperror.Wrap(err)
	}
	img, err := imaging.Decode(f, imaging.AutoOrientation(true))
	cerr := f.Close()
	if err != nil {
		return res, apperror.Wrap(err)
	}
	if cerr != nil {
		return res, apperror.Wrap(cerr)
	}

	img = resizeIfNeeded(img, opts.MaxWidth, opts.MaxHeight)

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".webppipe-*.tmp")
	if err != nil {
		return res, apperror.Wrap(err)
	}
	tmpPath := tmp.Name()
	encOpts := &webp.Options{
		Lossless: opts.Lossless,
		Quality:  float32(opts.Quality),
	}
	if err := webp.Encode(tmp, img, encOpts); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return res, apperror.Wrap(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return res, apperror.Wrap(err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return res, apperror.Wrap(err)
	}

	if di, err := os.Stat(dst); err == nil {
		res.TargetSize = di.Size()
	}

	if !keepOriginal {
		// Only delete when src and dst differ (e.g. when src already ends in
		// .webp this would be a no-op anyway).
		if filepath.Clean(src) != filepath.Clean(dst) {
			if err := os.Remove(src); err != nil {
				return res, apperror.Wrap(err)
			}
		}
	}
	return res, nil
}
