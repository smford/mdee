package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Width            *int               `yaml:"width" json:"width"`
	Theme            *string            `yaml:"theme" json:"theme"`
	TableStyle       *string            `yaml:"table-style" json:"table-style"`
	TableStyleSnake  *string            `yaml:"table_style" json:"table_style"`
	Images           *string            `yaml:"images" json:"images"`
	ImageMode        *string            `yaml:"image-mode" json:"image-mode"`
	ImageModeSnake   *string            `yaml:"image_mode" json:"image_mode"`
	ImageWidth       *string            `yaml:"image-width" json:"image-width"`
	ImageWidthSnake  *string            `yaml:"image_width" json:"image_width"`
	ImageHeight      *string            `yaml:"image-height" json:"image-height"`
	ImageHeightSnake *string            `yaml:"image_height" json:"image_height"`
	LineNumbers      *bool              `yaml:"line-numbers" json:"line-numbers"`
	LineNumbersSnake *bool              `yaml:"line_numbers" json:"line_numbers"`
	Hyperlinks       *bool              `yaml:"hyperlinks" json:"hyperlinks"`
	Pager            *bool              `yaml:"pager" json:"pager"`
	Plain            *bool              `yaml:"plain" json:"plain"`
	Debug            *bool              `yaml:"debug" json:"debug"`

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
	Mode   *string `yaml:"mode" json:"mode"`
	Width  *string `yaml:"width" json:"width"`
	Height *string `yaml:"height" json:"height"`
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

	if fc.ImageSection != nil {
		if fc.ImageSection.Mode != nil {
			opts.ImageMode = strings.TrimSpace(*fc.ImageSection.Mode)
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
