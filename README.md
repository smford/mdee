# mdee: Terminal Markdown Viewer for macOS iTerm2

[![CI](https://github.com/smford/mdee/actions/workflows/ci.yml/badge.svg)](https://github.com/smford/mdee/actions/workflows/ci.yml)
[![Website](https://img.shields.io/badge/Website-smford.github.io%2Fmdee-6366f1?logo=google-chrome&logoColor=white)](https://smford.github.io/mdee/)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)](https://golang.org)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Platform: macOS iTerm2](https://img.shields.io/badge/Platform-macOS%20iTerm2-blue?logo=apple)](https://iterm2.com)

`mdee` is a production-grade terminal Markdown viewer written in Go, specifically engineered for macOS and optimized for [iTerm2](https://iterm2.com).

Designed through a Senior Site Reliability Engineering lens, `mdee` solves common terminal Markdown rendering issues: **garbled wide tables**, **missing or broken images**, **pipe panics**, and **unreadable terminal wrapping**.

<p align="center">
  <img src="assets/screenshots/demo-overview.png" alt="mdee Terminal Markdown Viewer in macOS iTerm2" width="850" />
</p>

---

## Key Features

### 1. Accurate Inline Image Rendering (iTerm2 OSC 1337)
- **Native iTerm2 Protocol**: Directly transmits image bitstreams using the proprietary `OSC 1337` inline image escape sequence.
- **Multi-Format Support**: Displays PNG, JPEG, GIF (including animated GIFs), WebP, TIFF, and SVG natively.
- **Smart Asset Resolution**: Resolves relative file paths (e.g. `![diagram](./assets/arch.png)`) relative to the Markdown document's location, not just current working directory.
- **Resilient Remote Fetching**: Streams `http://` and `https://` images with bounded HTTP timeouts (10s) and a 25MB safety buffer to protect system memory.
- **tmux Passthrough**: Transparently wraps image payloads in tmux DCS escape sequences (`\033Ptmux;...`) when running inside tmux sessions.
- **Graceful Degradation**: Automatically falls back to formatted diagnostic placeholders on unsupported terminals or when `--images=never` is selected.

### 2. Native Mermaid Diagram Rendering & Cascading Fallback
- **Embedded `golang-mermaid` Engine**: Uses [`github.com/smford/golang-mermaid`](https://github.com/smford/golang-mermaid) to render Mermaid diagrams (`flowchart`, `sequenceDiagram`, `stateDiagram`, etc.) directly in the terminal.
- **Multi-Protocol Graphical Rendering**: Outputs inline images using iTerm2 OSC 1337, Kitty APC (`\033_G`), or DEC Sixel (`\033Pq`) graphics protocols.
- **High-DPI Retina Resampling**: Upscales diagrams using Catmull-Rom bicubic filtering (`--mermaid-scale 2.0`) to eliminate blurry text on high-density displays.
- **Background Contrast Compositing**: Automatically solves transparent PNG readability on dark/light terminal windows by compositing onto a high-contrast padded canvas (`#1e1e2e` for dark themes, `#ffffff` for light themes, or custom `#RRGGBB` via `--mermaid-bg`). Pass `--mermaid-bg transparent` for raw alpha.
- **Responsive Terminal Column Sizing**: Computes proportional terminal cell widths (`--mermaid-width auto` or explicit columns like `-w 80`) to ensure diagrams render prominently rather than as unreadable thumbnails.
- **Automated SRE Graceful Degradation**: When graphics protocols are not supported by the terminal, when piped or redirected, or when image generation is unreachable, it seamlessly cascades to high-fidelity Unicode box-drawing text art using `mmaid-go`.
- **ANSI & 7-Bit ASCII Modes**: Supports explicit user rendering preferences via `--mermaid ansi`, `--mermaid unicode`, or `--mermaid ascii`.
- **Scripting & Pipe Safety**: In `--plain` mode or when piped to non-interactive destinations, diagrams automatically render as pure 7-bit ASCII without ANSI escapes or binary sequences.
- **Content-Addressed Caching**: Diagram renders are cached by SHA-256 hash to eliminate redundant processing and enable offline operation.

### 3. High-Fidelity Table Layout Engine
- **Strict Column Alignment**: Preserves Markdown syntax alignments (`:---` Left, `:---:` Center, `---:` Right) across headers and data rows.
- **Unicode & Emoji Width Precision**: Calculates visual column boundaries using Unicode Standard Annex #29 / UAX #11 grapheme cluster metrics and ANSI stripping—eliminating jagged borders caused by emojis (`✅`, `⚠️`, `🚀`), variation selectors, or CJK glyphs.
- **Intelligent Word-Wrapping**: When tables exceed the available terminal columns, columns are proportionally sized and wrapped cleanly at word boundaries instead of overflowing the screen.
- **Multi-Line Row Synchronization**: Aligns wrapped multi-line cells seamlessly with matching vertical border continuations.
- **Customizable Borders**: Switch between `rounded`, `box` (sharp), `double`, `ascii`, `markdown`, and `minimal` borders via `--table-style`.

<p align="center">
  <img src="assets/screenshots/tables-and-alignment.png" alt="High-Fidelity Tables and Alignment Engine" width="850" />
</p>

### 4. Syntax Highlighting & Code Blocks
- **Chroma Syntax Highlighting**: Automatic language detection and theme matching (Dracula, Monokai, Solarized, GitHub).
- **Framed Code Enclosures**: Code blocks are enclosed in rounded border cards with language tags.
- **Line Numbers**: Toggle line numbers via `-n` / `--line-numbers`.

### 5. Clickable OSC 8 Hyperlinks
- Terminal links render as native OSC 8 clickable hyperlinks in iTerm2—`Cmd+Click` on any link text to open the target URL directly in your browser.

### 6. Production SRE Reliability
- **Safe Broken Pipes (`EPIPE`)**: Gracefully handles downstream pipe termination (e.g. `mdee doc.md | head -n 5`) without emitting runtime panics or broken pipe error traces.
- **Interactive Pager**: Optionally pipe long output through `$PAGER` (defaulting to `less -R -F -X`) using the `--pager` flag.
- **Plain Mode for Scripting**: Use `--plain` to strip all ANSI codes and borders, outputting clean plain text suitable for `grep`, `awk`, or saving to log files.
- **Built-in Diagnostics (`mdee doctor`)**: Inspect terminal capabilities, iTerm2 detection, truecolor, and inline graphic protocols in one command.

---

## Installation

### Via Homebrew (macOS & Linux)

Install using the custom Homebrew tap:

```bash
brew install smford/tap/mdee
```

Or tap first:

```bash
brew tap smford/tap
brew install mdee
```

To upgrade:

```bash
brew update && brew upgrade mdee
```

### From Source (Go 1.24+)

```bash
git clone https://github.com/smford/mdee.git
cd mdee
make install
```

This installs the `mdee` binary into your `$GOPATH/bin` (typically `~/go/bin/mdee`). Ensure `~/go/bin` is in your `$PATH`.

### Manual Build

```bash
make build
# Binary is generated at ./bin/mdee
./bin/mdee --version
```

---

## Usage

```bash
# View a local Markdown file
mdee README.md

# View a Markdown file from a remote URL
mdee https://raw.githubusercontent.com/smford/mdee/main/README.md

# Read from standard input (stdin)
cat architecture.md | mdee

# View with custom theme and table style
mdee --theme dracula --table-style box doc.md

# Constrain rendering width to 100 columns
mdee -w 100 report.md

# Show line numbers in code blocks
mdee -n main.md

# Plain text output (no ANSI escapes or colors, ASCII diagrams)
mdee --plain guide.md | grep "Configuration"

# Render Mermaid diagrams graphically (auto-detects terminal protocol)
mdee test.md

# Force Mermaid rendering as ANSI / Unicode box-drawing art
mdee --mermaid ansi test.md

# Force Mermaid rendering as pure 7-bit ASCII diagrams
mdee --mermaid ascii test.md

# Display raw syntax-highlighted Mermaid source code
mdee --mermaid raw test.md

# View Mermaid image with custom width and high-contrast background
mdee --mermaid-width 80 --mermaid-bg "#1e1e2e" test.md

# View Mermaid image with clean white canvas card
mdee --mermaid-bg white test.md
```

---

## CLI Options

| Flag | Shorthand | Default | Description |
| :--- | :---: | :---: | :--- |
| `--width` | `-w` | `0` | Explicit terminal width in columns (`0` = auto-detect) |
| `--theme` | `-t` | `"dark"` | Color theme: `dark`, `light`, `dracula`, `monokai`, `solarized-dark`, `solarized-light`, `plain` |
| `--table-style` | `-s` | `"rounded"` | Border style: `rounded`, `box`, `double`, `ascii`, `markdown`, `minimal` |
| `--images` | `-i` | `"auto"` | Inline image mode: `auto` (iTerm2 only), `always`, `never` |
| `--image-width` | | `"auto"` | Image width constraint: `auto`, `100%`, `80`, `400px` |
| `--image-height`| | `"auto"` | Image height constraint: `auto`, `20`, `300px` |
| `--mermaid` | `-m` | `"auto"` | Mermaid render mode: `auto`, `image`, `ansi`, `unicode`, `ascii`, `raw` |
| `--mermaid-theme` | | `""` | Mermaid theme: `dark`, `default`, `slate`, `blueprint`, `neon`, `neutral`, `forest` |
| `--mermaid-width` | | `"auto"` | Mermaid image display width: `auto` (smart responsive), `100%`, `80`, `800px` |
| `--mermaid-bg`    | | `"auto"` | Mermaid image background canvas: `auto`, `dark`, `light`, `transparent`, `#RRGGBB` |
| `--mermaid-scale` | | `2.0` | Mermaid image rasterization scale factor (`1.0` - `4.0` for Retina/HiDPI) |
| `--line-numbers`| `-n` | `false` | Display line numbers in code blocks |
| `--hyperlinks`  | | `true` | Enable OSC 8 clickable terminal hyperlinks |
| `--no-hyperlinks`| | `false` | Disable OSC 8 clickable terminal hyperlinks (display full URLs) |
| `--pager`       | | `false` | Enable pager for output longer than terminal screen |
| `--no-pager`    | | `false` | Disable pager output |
| `--plain`       | | `false` | Output plain text without ANSI escape sequences |
| `--debug`       | | `false` | Print diagnostic debug logs to stderr |

---

## Subcommands

### `mdee doctor`
Diagnoses your current terminal environment and verifies protocol support:

```bash
$ mdee doctor

╭──────────────────────────────────────────────────────────────╮
│          mdee - SRE Terminal Diagnostic & Capabilities       │
╰──────────────────────────────────────────────────────────────╯

╭──────────────────────────┬───────────────────┬─────────────────────────╮
│ Diagnostic Check         │ Value             │ Status                  │
├──────────────────────────┼───────────────────┼─────────────────────────┤
│ OS / Architecture        │ darwin / arm64    │ PASS                    │
│ Go Runtime Version       │ go1.26.0          │ PASS                    │
│ Standard Output TTY      │ true              │ ENABLED                 │
│ Terminal Dimensions      │ 120 cols x 36 rows│ INFO                    │
│ TERM_PROGRAM             │ iTerm.app         │ INFO                    │
│ TERM                     │ xterm-256color    │ INFO                    │
│ iTerm2 Detection         │ true              │ DETECTED (macOS iTerm2) │
│ Inside tmux Session      │ false             │ No                      │
│ OSC 1337 (Inline Images) │ true              │ ENABLED                 │
│ OSC 8 (Terminal Links)   │ true              │ ENABLED                 │
│ TrueColor (24-bit)       │ true              │ ENABLED                 │
│ Mermaid Protocol         │ iterm2            │ iTerm2 Graphics (OSC 1337)│
│ Mermaid CLI (mmdc)       │ false             │ Not Found (Remote / Fallback) │
│ Mermaid Text Engine      │ mmaid-go          │ READY (Pure-Go)         │
╰──────────────────────────┴───────────────────┴─────────────────────────╯

── Protocol Verification Test ──────────────────────────────────

  • OSC 8 Hyperlink: Click here to test OSC 8 GitHub link
  • OSC 1337 Inline Image Test (32x32 color gradient swatch):
    [Inline graphic test rendered here]
  • Mermaid Diagram Rendering Test (image mode, iterm2 protocol):
    [Inline Mermaid diagram graphic rendered here]
```

<p align="center">
  <img src="assets/screenshots/doctor-diagnostics.png" alt="mdee doctor Terminal Diagnostics and Capabilities" width="850" />
</p>

---

## tmux Configuration for iTerm2 Images

If you run inside `tmux` within iTerm2, tmux blocks terminal escape sequences by default unless passthrough is enabled. To display images seamlessly inside tmux:

1. Add the following line to `~/.tmux.conf`:
   ```tmux
   set -g allow-passthrough on
   ```
2. Reload tmux:
   ```bash
   tmux source-file ~/.tmux.conf
   ```

`mdee` automatically detects tmux sessions and formats images with the necessary DCS passthrough encapsulation.

---

## Architecture & Codebase Layout

```
.
├── cmd/
│   └── mdee/
│       ├── main.go          # CLI entrypoint, flag parsing, broken pipe handling
│       └── main_test.go     # CLI command execution tests
├── internal/
│   ├── config/              # Runtime configuration defaults and types
│   ├── doctor/              # SRE terminal diagnostics & capability probe
│   ├── image/               # OSC 1337 encoder, fetcher, dimension parser
│   ├── renderer/            # Goldmark AST terminal renderer & Mermaid engine
│   │   ├── renderer.go      # Markdown node visitor and inline element formatting
│   │   └── mermaid.go       # Contrast canvas compositing, Catmull-Rom HiDPI scaling
│   ├── table/               # Accurate table engine, Unicode grapheme wrapping, borders
│   ├── term/                # iTerm2 detection, TTY size, OSC 8 links, pager
│   └── theme/               # Color palettes and Lipgloss/Chroma style sheets
├── testdata/
│   ├── demo.md              # Comprehensive demo document
│   └── sample.png           # Test telemetry chart image
├── Makefile                 # Build, test, lint, and install automation
└── .github/workflows/       # GitHub Actions CI & release pipelines
```

---

## Testing & Quality Assurance

All packages include table-driven unit tests, race-detector coverage, and static analysis:

```bash
# Run unit tests
make test

# Run tests with the Go race detector enabled
make test-race

# Run go vet
make vet
```

---

## License

GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE) for details.
