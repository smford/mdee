package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mermaid "github.com/smford/golang-mermaid"
	"gopkg.in/yaml.v3"
)

// DefaultConfigFile returns the canonical path to ~/.mdeerc in the user's home directory.
func DefaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".mdeerc")
}

// ExpandPath expands leading ~ to the user's home directory.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// fileConfig mirrors configuration keys supported in ~/.mdeerc (YAML/JSON format).
type fileConfig struct {
	Width              *int               `yaml:"width" json:"width"`
	Theme              *string            `yaml:"theme" json:"theme"`
	TableStyle         *string            `yaml:"table-style" json:"table-style"`
	TableStyleSnake    *string            `yaml:"table_style" json:"table_style"`
	Images             *string            `yaml:"images" json:"images"`
	ImageMode          *string            `yaml:"image-mode" json:"image-mode"`
	ImageModeSnake     *string            `yaml:"image_mode" json:"image_mode"`
	ImageProtocol      *string            `yaml:"image-protocol" json:"image-protocol"`
	ImageProtocolSnake *string            `yaml:"image_protocol" json:"image_protocol"`
	ImageWidth         *string            `yaml:"image-width" json:"image-width"`
	ImageWidthSnake    *string            `yaml:"image_width" json:"image_width"`
	ImageHeight        *string            `yaml:"image-height" json:"image-height"`
	ImageHeightSnake   *string            `yaml:"image_height" json:"image_height"`
	LineNumbers        *bool              `yaml:"line-numbers" json:"line-numbers"`
	LineNumbersSnake   *bool              `yaml:"line_numbers" json:"line_numbers"`
	Hyperlinks         *bool              `yaml:"hyperlinks" json:"hyperlinks"`
	Pager              *bool              `yaml:"pager" json:"pager"`
	Plain              *bool              `yaml:"plain" json:"plain"`
	Debug              *bool              `yaml:"debug" json:"debug"`

	// Flat Mermaid options
	MermaidMode            *string  `yaml:"mermaid-mode" json:"mermaid-mode"`
	MermaidModeSnake       *string  `yaml:"mermaid_mode" json:"mermaid_mode"`
	MermaidTheme           *string  `yaml:"mermaid-theme" json:"mermaid-theme"`
	MermaidThemeSnake      *string  `yaml:"mermaid_theme" json:"mermaid_theme"`
	MermaidWidth           *string  `yaml:"mermaid-width" json:"mermaid-width"`
	MermaidWidthSnake      *string  `yaml:"mermaid_width" json:"mermaid_width"`
	MermaidBg              *string  `yaml:"mermaid-bg" json:"mermaid-bg"`
	MermaidBgSnake         *string  `yaml:"mermaid_bg" json:"mermaid_bg"`
	MermaidBackground      *string  `yaml:"mermaid-background" json:"mermaid-background"`
	MermaidBackgroundSnake *string  `yaml:"mermaid_background" json:"mermaid_background"`
	MermaidScale           *float64 `yaml:"mermaid-scale" json:"mermaid-scale"`
	MermaidScaleSnake      *float64 `yaml:"mermaid_scale" json:"mermaid_scale"`

	// Nested Image Section
	ImageSection *imageSection `yaml:"image" json:"image"`
}

type imageSection struct {
	Mode     *string `yaml:"mode" json:"mode"`
	Protocol *string `yaml:"protocol" json:"protocol"`
	Width    *string `yaml:"width" json:"width"`
	Height   *string `yaml:"height" json:"height"`
}

type mermaidSection struct {
	Mode       *string  `yaml:"mode" json:"mode"`
	Theme      *string  `yaml:"theme" json:"theme"`
	Width      *string  `yaml:"width" json:"width"`
	Bg         *string  `yaml:"bg" json:"bg"`
	Background *string  `yaml:"background" json:"background"`
	Scale      *float64 `yaml:"scale" json:"scale"`
}

func (fc *fileConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawFields fileConfig
	var raw struct {
		rawFields  `yaml:",inline"`
		MermaidRaw yaml.Node `yaml:"mermaid"`
	}

	if err := value.Decode(&raw); err != nil {
		return err
	}

	*fc = fileConfig(raw.rawFields)

	switch raw.MermaidRaw.Kind {
	case yaml.ScalarNode:
		var mode string
		if err := raw.MermaidRaw.Decode(&mode); err == nil && mode != "" {
			fc.MermaidMode = &mode
		}
	case yaml.MappingNode:
		var ms mermaidSection
		if err := raw.MermaidRaw.Decode(&ms); err == nil {
			if ms.Mode != nil {
				fc.MermaidMode = ms.Mode
			}
			if ms.Theme != nil {
				fc.MermaidTheme = ms.Theme
			}
			if ms.Width != nil {
				fc.MermaidWidth = ms.Width
			}
			if ms.Bg != nil {
				fc.MermaidBg = ms.Bg
			} else if ms.Background != nil {
				fc.MermaidBg = ms.Background
			}
			if ms.Scale != nil {
				fc.MermaidScale = ms.Scale
			}
		}
	}

	return nil
}

// LoadConfigFile attempts to load and parse configuration from the given path.
// If explicit is false and the file does not exist, it returns DefaultOptions(), false, nil.
// If explicit is true and the file does not exist, it returns an error.
// If the file exists and is parsed, it merges onto DefaultOptions() and returns opts, true, nil.
func LoadConfigFile(path string, explicit bool) (Options, bool, error) {
	opts := DefaultOptions()
	if path == "" {
		path = DefaultConfigFile()
	}
	if path == "" {
		return opts, false, nil
	}

	resolvedPath := ExpandPath(path)
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
			if explicit {
				return opts, false, fmt.Errorf("config file not found: %s", path)
			}
			return opts, false, nil
		}
		return opts, false, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	if err := ApplyConfigData(&opts, data); err != nil {
		return opts, false, fmt.Errorf("error parsing config file %s: %w", path, err)
	}

	opts.ConfigFile = resolvedPath
	return opts, true, nil
}

// ApplyConfigData unmarshals YAML/JSON bytes into opts.
func ApplyConfigData(opts *Options, data []byte) error {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return err
	}

	if fc.Width != nil {
		opts.Width = *fc.Width
	}
	if fc.Theme != nil {
		opts.Theme = strings.TrimSpace(*fc.Theme)
	}
	if fc.TableStyle != nil {
		opts.TableStyle = strings.TrimSpace(*fc.TableStyle)
	} else if fc.TableStyleSnake != nil {
		opts.TableStyle = strings.TrimSpace(*fc.TableStyleSnake)
	}

	// Images
	if fc.Images != nil {
		opts.ImageMode = strings.TrimSpace(*fc.Images)
	} else if fc.ImageMode != nil {
		opts.ImageMode = strings.TrimSpace(*fc.ImageMode)
	} else if fc.ImageModeSnake != nil {
		opts.ImageMode = strings.TrimSpace(*fc.ImageModeSnake)
	}

	if fc.ImageWidth != nil {
		opts.ImageWidth = strings.TrimSpace(*fc.ImageWidth)
	} else if fc.ImageWidthSnake != nil {
		opts.ImageWidth = strings.TrimSpace(*fc.ImageWidthSnake)
	}

	if fc.ImageHeight != nil {
		opts.ImageHeight = strings.TrimSpace(*fc.ImageHeight)
	} else if fc.ImageHeightSnake != nil {
		opts.ImageHeight = strings.TrimSpace(*fc.ImageHeightSnake)
	}

	if fc.ImageProtocol != nil {
		if p, err := mermaid.ParseGraphicsProtocol(*fc.ImageProtocol); err == nil {
			opts.ImageProtocol = p
		}
	} else if fc.ImageProtocolSnake != nil {
		if p, err := mermaid.ParseGraphicsProtocol(*fc.ImageProtocolSnake); err == nil {
			opts.ImageProtocol = p
		}
	}

	if fc.ImageSection != nil {
		if fc.ImageSection.Mode != nil {
			opts.ImageMode = strings.TrimSpace(*fc.ImageSection.Mode)
		}
		if fc.ImageSection.Protocol != nil {
			if p, err := mermaid.ParseGraphicsProtocol(*fc.ImageSection.Protocol); err == nil {
				opts.ImageProtocol = p
			}
		}
		if fc.ImageSection.Width != nil {
			opts.ImageWidth = strings.TrimSpace(*fc.ImageSection.Width)
		}
		if fc.ImageSection.Height != nil {
			opts.ImageHeight = strings.TrimSpace(*fc.ImageSection.Height)
		}
	}

	// Booleans
	if fc.LineNumbers != nil {
		opts.LineNumbers = *fc.LineNumbers
	} else if fc.LineNumbersSnake != nil {
		opts.LineNumbers = *fc.LineNumbersSnake
	}

	if fc.Hyperlinks != nil {
		opts.Hyperlinks = *fc.Hyperlinks
	}
	if fc.Pager != nil {
		opts.Pager = *fc.Pager
	}
	if fc.Plain != nil {
		opts.Plain = *fc.Plain
	}
	if fc.Debug != nil {
		opts.Debug = *fc.Debug
	}

	// Mermaid - flat options
	if fc.MermaidMode != nil {
		opts.MermaidMode = strings.TrimSpace(*fc.MermaidMode)
	} else if fc.MermaidModeSnake != nil {
		opts.MermaidMode = strings.TrimSpace(*fc.MermaidModeSnake)
	}

	if fc.MermaidTheme != nil {
		opts.MermaidTheme = strings.TrimSpace(*fc.MermaidTheme)
	} else if fc.MermaidThemeSnake != nil {
		opts.MermaidTheme = strings.TrimSpace(*fc.MermaidThemeSnake)
	}

	if fc.MermaidWidth != nil {
		opts.MermaidWidth = strings.TrimSpace(*fc.MermaidWidth)
	} else if fc.MermaidWidthSnake != nil {
		opts.MermaidWidth = strings.TrimSpace(*fc.MermaidWidthSnake)
	}

	if fc.MermaidBg != nil {
		opts.MermaidBg = strings.TrimSpace(*fc.MermaidBg)
	} else if fc.MermaidBgSnake != nil {
		opts.MermaidBg = strings.TrimSpace(*fc.MermaidBgSnake)
	} else if fc.MermaidBackground != nil {
		opts.MermaidBg = strings.TrimSpace(*fc.MermaidBackground)
	} else if fc.MermaidBackgroundSnake != nil {
		opts.MermaidBg = strings.TrimSpace(*fc.MermaidBackgroundSnake)
	}

	if fc.MermaidScale != nil {
		opts.MermaidScale = *fc.MermaidScale
	} else if fc.MermaidScaleSnake != nil {
		opts.MermaidScale = *fc.MermaidScaleSnake
	}

	return nil
}

// DefaultConfigTemplate returns a fully annotated YAML configuration file with standard defaults.
func DefaultConfigTemplate() string {
	return `# ~/.mdeerc - Configuration file for mdee (Markdown Terminal Viewer)
#
# Place this file at ~/.mdeerc to configure personal default preferences.
# You can also point to a custom config file using the -c, --config flag:
#   mdee -c /path/to/custom-config.yaml document.md
#
# Precedence Order:
#   1. Explicit CLI flags (e.g. -t dracula, --mermaid ansi)
#   2. Configuration file (~/.mdeerc or -c <path>)
#   3. Built-in defaults
#
# Both YAML and JSON formats are supported.
# Both kebab-case (table-style) and snake_case (table_style) keys are supported.

# Terminal rendering width in columns (0 = auto-detect terminal columns)
width: 0

# Color theme: dark, light, dracula, monokai, solarized-dark, solarized-light, plain
theme: "dark"

# Table border style: rounded, box, double, ascii, markdown, minimal
table-style: "rounded"

# Show line numbers in code blocks (true or false)
line-numbers: false

# Enable clickable terminal hyperlinks (OSC 8)
hyperlinks: true

# Route output longer than screen height through $PAGER (less -R -F -X)
pager: false

# Output plain text with no ANSI styling, escape codes, or borders
plain: false

# Print SRE diagnostic debug traces to stderr
debug: false

# -----------------------------------------------------------------------------
# Image Options
# -----------------------------------------------------------------------------
# Can be specified as flat options:
#   images: "auto"           # auto, always, never
#   image-protocol: "auto"   # auto (detects Kitty, iTerm2, Sixel), kitty, iterm2, sixel, none
#   image-width: "auto"      # auto, 100%, 80, 400px
#   image-height: "auto"     # auto, 20, 300px
#
# Or as a nested section:
image:
  mode: "auto"
  protocol: "auto"
  width: "auto"
  height: "auto"

# -----------------------------------------------------------------------------
# Mermaid Diagram Options
# -----------------------------------------------------------------------------
# Can be specified as a scalar shortcut:
#   mermaid: "auto"       # auto, image, ansi, unicode, ascii, raw
#
# Or as flat options:
#   mermaid-mode: "auto"
#   mermaid-theme: "dark" # dark, default, slate, blueprint, neon, neutral, forest
#   mermaid-width: "auto" # auto (smart cell fit), 100%, 80, 800px
#   mermaid-bg: "auto"    # auto, dark, light, transparent, #RRGGBB
#   mermaid-scale: 2.0    # 1.0 - 4.0 (HiDPI / Retina scale factor)
#
# Or as a nested section:
mermaid:
  mode: "auto"
  theme: ""
  width: "auto"
  bg: "auto"
  scale: 2.0
`
}

// WriteDefaultConfigFile writes the default configuration template to targetPath.
// If targetPath is empty, it uses DefaultConfigFile().
// If the target file already exists and force is false, it returns an error.
// Creates any missing parent directories before writing.
func WriteDefaultConfigFile(targetPath string, force bool) (string, error) {
	if targetPath == "" {
		targetPath = DefaultConfigFile()
	}
	if targetPath == "" {
		return "", errors.New("unable to resolve configuration file path")
	}

	resolvedPath := ExpandPath(targetPath)

	if !force {
		if _, err := os.Stat(resolvedPath); err == nil {
			return resolvedPath, fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", resolvedPath)
		}
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return resolvedPath, fmt.Errorf("failed creating directory %s: %w", dir, err)
	}

	content := DefaultConfigTemplate()
	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return resolvedPath, fmt.Errorf("failed writing configuration file to %s: %w", resolvedPath, err)
	}

	return resolvedPath, nil
}

