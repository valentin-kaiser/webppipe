# webppipe

A Go CLI and Docker-based GitHub Action that scans a Git repository, converts PNG/JPEG images to **WebP** (optionally resizing), and commits & pushes the results.

## Features

- Recursively scans the repository for PNG / JPEG / JPG files
- Encodes to WebP with configurable quality or lossless mode (powered by libwebp via `chai2010/webp`)
- Optional max-width / max-height resize preserving aspect ratio
- Concurrent worker pool (defaults to `NumCPU`)
- Idempotent: skips files that already have an up-to-date `.webp` sibling
- Configurable include / exclude glob patterns
- `dry-run` mode and `keep-originals` mode
- Built-in commit & push (uses the token provided by `actions/checkout`)
- Configurable via YAML config file, CLI flags, or `WEBPPIPE_*` environment variables

## Quick start (GitHub Action)

```yaml
name: Optimize images
on:
  workflow_dispatch:
  push:
    branches: [main]
    paths:
      - "**/*.png"
      - "**/*.jpg"
      - "**/*.jpeg"

permissions:
  contents: write

jobs:
  optimize:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}
      - uses: valentin-kaiser/webppipe@v1
        with:
          quality: "82"
          max-width: "1920"
```

A complete example lives in [.github/workflows/example-usage.yml](.github/workflows/example-usage.yml).

## Action inputs

| Input            | Default                                                    | Description                                                  |
| ---------------- | ---------------------------------------------------------- | ------------------------------------------------------------ |
| `quality`        | `80`                                                       | WebP quality (1–100). Ignored when `lossless` is `true`.     |
| `lossless`       | `false`                                                    | Use lossless WebP encoding.                                  |
| `max-width`      | `0`                                                        | Maximum width in pixels (0 = unlimited).                     |
| `max-height`     | `0`                                                        | Maximum height in pixels (0 = unlimited).                    |
| `include`        | `**/*.png,**/*.jpg,**/*.jpeg,**/*.PNG,**/*.JPG,**/*.JPEG`  | Comma-separated globs of files to include.                   |
| `exclude`        | `.git/**,node_modules/**,vendor/**`                        | Comma-separated globs of files to exclude.                   |
| `keep-originals` | `true`                                                     | Keep originals next to the generated `.webp` files. Set to `false` to replace originals. |
| `dry-run`        | `false`                                                    | Report what would change without writing.                    |
| `concurrency`    | `0`                                                        | Worker count (0 = `NumCPU`).                                 |
| `repo-path`      | `.`                                                        | Working directory (typically the repo root).                 |
| `commit`         | `true`                                                     | Commit converted files.                                      |
| `push`           | `true`                                                     | Push the commit to the remote.                               |
| `commit-message` | `[GEN] optimize images to WebP`                           | Commit message.                                              |
| `author-name`    | `webppipe[bot]`                                            | Git author name.                                             |
| `author-email`   | `webppipe@users.noreply.github.com`                        | Git author email.                                            |
| `branch`         | _(empty)_                                                  | Branch to push to (empty = current branch).                  |

## Local CLI

Build (requires `libwebp-dev` and a C toolchain):

```bash
go build -o webppipe .
```

Or simply build/run with Docker:

```bash
docker build -t webppipe .
docker run --rm -v "$PWD:/work" -w /work webppipe --dry-run
```

### CLI examples

```bash
# Show what would change without touching anything
webppipe --dry-run

# Convert with quality 90 and cap dimensions
webppipe --quality 90 --max-width 1920 --max-height 1080

# Keep originals and skip git
webppipe --keep-originals --git.enabled=false
```

Flags are auto-generated from the configuration struct; run `webppipe --help` for the full list.

### Configuration file

Place a `webppipe.yaml` in the directory passed via `--path` (default `./data`):

```yaml
quality: 82
lossless: false
max-width: 1920
max-height: 0
include:
  - "**/*.png"
  - "**/*.jpg"
  - "**/*.jpeg"
exclude:
  - ".git/**"
  - "node_modules/**"
  - "vendor/**"
keep-originals: false
dry-run: false
concurrency: 4
repo-path: "."
git:
  enabled: true
  commit-message: "optimize images to WebP"
  author-name: "webppipe[bot]"
  author-email: "webppipe@users.noreply.github.com"
  branch: ""
  push: true
```

### Environment variables

Every config field can be supplied via an environment variable prefixed with `WEBPPIPE_`, e.g. `WEBPPIPE_QUALITY`, `WEBPPIPE_MAX_WIDTH`, `WEBPPIPE_GIT_PUSH`.

## Example image

The repository ships with [example.jpg](example.jpg) (1536×1024) so you can try the tool end-to-end without preparing your own assets:

```bash
# Build (requires libwebp-dev)
go build -o webppipe .

# Dry run — see what would happen
./webppipe --dry-run --git.enabled=false --include='example.jpg'

# Convert example.jpg → example.webp, keep the original (default)
./webppipe --git.enabled=false --include='example.jpg'

# Convert and replace the original
./webppipe --git.enabled=false --keep-originals=false --include='example.jpg'
```

## Idempotency

A source file is **skipped** when a sibling `.webp` already exists with a modification time greater than or equal to the source's. When `keep-originals` is `false`, the source is deleted after a successful conversion, so subsequent runs naturally find nothing to do.

## Caveats

- Removing originals will break references to the old `.png`/`.jpg` filenames inside HTML, Markdown, CSS, or source files. If your repository contains such references, run with `keep-originals: true` or update the references manually.
- WebP can occasionally be larger than an already-optimized PNG. There is no minimum-savings threshold yet — the converted file always replaces the original.
- The action uses `libwebp` via cgo. The published Docker image bundles the runtime library; building locally requires `libwebp-dev` (Debian/Ubuntu) or equivalent.

## Development

```bash
# Tests (requires libwebp-dev)
go test ./...

# Lint
go vet ./...

# Docker image
docker build -t webppipe:dev .
```
