# mdee: High-Fidelity Terminal Markdown Viewer

[![CI](https://github.com/smford/mdee/actions/workflows/ci.yml/badge.svg)](https://github.com/smford/mdee/actions/workflows/ci.yml)
[![Website](https://img.shields.io/badge/Website-smford.github.io%2Fmdee-6366f1?logo=google-chrome&logoColor=white)](https://smford.github.io/mdee/)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)](https://golang.org)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Platform: macOS & Linux](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux-blue?logo=linux&logoColor=white)](#terminal-compatibility)

`mdee` is a production-grade terminal Markdown viewer written in Go, specifically engineered for modern terminal emulators (Kitty, Ghostty, WezTerm, iTerm2, Foot, Linux & macOS).

Designed through a Senior Site Reliability Engineering lens, `mdee` solves common terminal Markdown rendering issues: **garbled wide tables**, **missing or broken images**, **pipe panics**, and **unreadable terminal wrapping**.

<p align="center">
  <img src="assets/screenshots/demo-overview.png" alt="mdee Terminal Markdown Viewer" width="850" />
</p>

---

## Terminal Compatibility

`mdee` provides first-class, multi-protocol graphics and layout compatibility with modern terminal emulators on macOS, Linux, and Windows:

| Terminal Emulator | Default Protocol | Display Output | Notes |
| :--- | :--- | :--- | :--- |
| **Kitty** | `kitty` | 🖼️ Native Inline Image | High-speed Kitty graphics (`\033_G`) with chunking |
| **Ghostty** | `kitty` | 🖼️ Native Inline Image | Native Kitty protocol preferred; OSC 1337 also supported |
| **WezTerm** | `iterm2` | 🖼️ Native Inline Image | Full OSC 1337 and Kitty protocol support |
| **iTerm2** | `iterm2` | 🖼️ Native Inline Image | Native OSC 1337 inline image protocol |
| **Foot** | `sixel` | 🖼️ Native Inline Image | High-performance DEC Sixel bitmap graphics |
| **mlterm** | `sixel` | 🖼️ Native Inline Image | DEC Sixel bitmap protocol |
| **mintty** | `iterm2` | 🖼️ Native Inline Image | Windows terminal with OSC 1337 and Sixel |
| **Apple Terminal** | `none` | 🔤 Unicode Box Art | Graceful fallback (no image protocol support) |
| **Alacritty** | `none` | 🔤 Unicode Box Art | Graceful fallback (no image protocol support) |
| **CI/CD Runners** | `none` | 🔤 Unicode Box Art / ASCII | Automated non-interactive TTY fallback |
| **Output piped to file / grep** | `none` | 📄 Clean Text | Safe TTY detection protects output files from binary graphics escape sequences |

---

## Key Features

### 1. Accurate Inline Image Rendering (Multi-Protocol & Native Graphics)
- **Native Multi-Protocol Graphics**: Transmits inline graphics using Kitty APC (`\033_G`), iTerm2 OSC 1337 (`\033]1337`), or DEC Sixel bitmap (`\033Pq`) protocols depending on terminal capabilities or user flag.
- **Protocol Override**: Explicitly choose your preferred image graphics protocol with `--image-protocol` (`auto`, `kitty`, `iterm2`, `sixel`, `none`).
- **Multi-Format Support**: Displays PNG, JPEG, GIF (including animated GIFs), WebP, TIFF, and SVG. Non-PNG images are automatically converted in-memory to PNG when transmitting via the Kitty protocol.
- **Smart Asset Resolution**: Resolves relative file paths (e.g. `![diagram](./assets/arch.png)`) relative to the Markdown document's location, not just current working directory.
- **HTML Image Tag Support**: Seamlessly parses and renders HTML images in both block and inline contexts (e.g. `<img src="..." width="400">`, `<p align="center"><img ...></p>`, or `<a><img ...></a>`), resolving relative filesystem paths and remote URLs, honoring width/height constraints, and stripping unnecessary HTML container noise.
- **Resilient Remote Fetching**: Streams `http://` and `https://` images with bounded HTTP timeouts (10s), in-memory caching, concurrent AST pre-fetching, and a 25MB safety buffer to protect system memory.
- **tmux Passthrough**: Transparently wraps image payloads in tmux DCS escape sequences (`\033Ptmux;...`) when running inside tmux sessions (`set -g allow-passthrough on`).
- **Graceful Degradation**: Automatically falls back to formatted diagnostic placeholders on unsupported terminals, when piping/redirecting, or when `--images=never` is selected.

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

### 7. Persistent Configuration (`~/.mdeerc`)
- **Automatic Discovery**: Automatically loads user preferences from `~/.mdeerc` in your home directory if present. If absent, proceeds silently with standard compiled defaults.
- **YAML & JSON Support**: Configure your preferred theme, borders, Mermaid modes, or image behavior using clean YAML or JSON format.
- **Hierarchical Precedence**: Deterministic precedence guarantees CLI flags override `~/.mdeerc` settings, which override compiled defaults.
- **Custom Config File**: Specify custom configuration files on demand with `-c, --config <path>`.

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

# Load preferences from an explicit configuration file
mdee --config ~/.config/mdee/dark.yaml report.md

# Override configuration file preferences with CLI flags
mdee -c ~/.mdeerc --theme dracula report.md
```

---

## CLI Options

| Flag | Shorthand | Default | Description |
| :--- | :--- | :--- | :--- |
| `--config` | `-c` | `""` | Path to configuration file (defaults to `~/.mdeerc` if present) |
| `--width` | `-w` | `0` | Explicit terminal width in columns (`0` = auto-detect) |
| `--theme` | `-t` | `"dark"` | Color theme: `dark`, `light`, `dracula`, `monokai`, `solarized-dark`, `solarized-light`, `plain` |
| `--table-style` | `-s` | `"rounded"` | Border style: `rounded`, `box`, `double`, `ascii`, `markdown`, `minimal` |
| `--images` | `-i` | `"auto"` | Inline image mode: `auto` (detect protocol), `always`, `never` |
| `--image-width` | | `"auto"` | Image width constraint: `auto`, `100%`, `80`, `400px` |
| `--image-height`| | `"auto"` | Image height constraint: `auto`, `20`, `300px` |
| `--image-protocol`| | `"auto"` | Image graphics protocol: `auto`, `kitty`, `iterm2`, `sixel`, `none` |
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
| `--init-config` | | `false` | Generate default `~/.mdeerc` configuration file and exit |

---

## Configuration File (`~/.mdeerc`)

`mdee` automatically searches for a configuration file named `~/.mdeerc` in your home directory upon launch. If found, it establishes your default preferences across all commands. If `~/.mdeerc` does not exist, `mdee` proceeds silently with compiled defaults without requiring any setup.

You can also point `mdee` to an explicit configuration file via `-c` / `--config`:

```bash
mdee -c ~/.config/mdee/work.yaml document.md
mdee --config ./project.mdeerc.json document.md
```

### Precedence Hierarchy

1. **CLI Flags** (e.g. `--theme dracula`, `--mermaid ansi`, `-w 80`) &mdash; highest priority, overrides configuration file and defaults.
2. **Configuration File** (`~/.mdeerc` or path supplied via `-c, --config`).
3. **Compiled Defaults** &mdash; baseline fallback.

### Supported Syntax & Formats

Both **YAML** and **JSON** are supported. Configuration keys accept kebab-case (`table-style`) or snake_case (`table_style`), and can be expressed either as flat keys or organized into nested sections:

```yaml
# ~/.mdeerc (YAML example)
theme: "dracula"
table-style: "rounded"
line-numbers: false
hyperlinks: true
pager: false

# Nested image settings
image:
  mode: "auto"
  width: "auto"
  height: "auto"

# Nested Mermaid settings (or scalar shortcut: mermaid: "unicode")
mermaid:
  mode: "auto"
  theme: "dark"
  width: "auto"
  bg: "#1e1e2e"
  scale: 2.0
```

A complete reference configuration is available at [`.mdeerc.example`](.mdeerc.example).

### Generating a Default Configuration File

To quickly generate a documented `~/.mdeerc` file pre-populated with standard defaults, run:

```bash
# Generate ~/.mdeerc with default values
mdee init

# Print template to stdout for previewing or piping
mdee init --stdout

# Write to a custom path
mdee init --output ~/.config/mdee/mdeerc.yaml

# Overwrite existing config file
mdee init --force
```

Alternatively, use the `--init-config` flag on the root command:

```bash
mdee --init-config
```

---

## Subcommands

### `mdee init`
Generates a fully documented default `~/.mdeerc` file to help you customize your settings:

```bash
$ mdee init
Created default configuration file: /Users/username/.mdeerc
```

Flags:
- `-f, --force`: Overwrite existing configuration file if it already exists.
- `-o, --output <path>`: Destination path (defaults to `~/.mdeerc`).
- `--stdout`: Output configuration template to standard output instead of writing to disk.

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
│ Terminal Emulator        │ iTerm2            │ DETECTED (iterm2 graphics) │
│ Inside tmux Session      │ false             │ No                      │
│ Active Graphics Protocol │ iterm2            │ iTerm2 Graphics (OSC 1337)│
│ Kitty Graphics (APC)     │ false             │ DISABLED                │
│ OSC 1337 (iTerm2 Graphics)│ true             │ ENABLED                 │
│ DEC Sixel Graphics (DCS) │ false             │ DISABLED                │
│ OSC 8 (Terminal Links)   │ true              │ ENABLED                 │
│ TrueColor (24-bit)       │ true              │ ENABLED                 │
│ Mermaid Protocol         │ iterm2            │ iTerm2 Graphics (OSC 1337)│
│ Mermaid CLI (mmdc)       │ false             │ Not Found (Remote / Fallback) │
│ Mermaid Text Engine      │ mmaid-go          │ READY (Pure-Go)         │
│ Config File (~/.mdeerc)  │ ~/.mdeerc         │ LOADED                  │
╰──────────────────────────┴───────────────────┴─────────────────────────╯

── Protocol Verification Test ──────────────────────────────────

  • OSC 8 Hyperlink: Click here to test OSC 8 GitHub link
  • Inline Image Test (iterm2 protocol, 32x32 color gradient swatch):
    [Inline graphic test rendered here]
  • Mermaid Diagram Rendering Test (image mode, iterm2 protocol):
    [Inline Mermaid diagram graphic rendered here]
```

<p align="center">
  <img src="assets/screenshots/doctor-diagnostics.png" alt="mdee doctor Terminal Diagnostics and Capabilities" width="850" />
</p>

---

## tmux Configuration for Inline Graphics (Kitty, iTerm2, Sixel)

If you run inside `tmux`, tmux blocks terminal escape sequences by default unless passthrough is enabled. To display images and diagrams seamlessly inside tmux:

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
│   ├── config/              # Runtime configuration, ~/.mdeerc loader, defaults, and types
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
