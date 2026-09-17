package config

import (
	mermaid "github.com/smford/golang-mermaid"
)

// Options defines runtime rendering configuration.
type Options struct {
	Width         int                      // Available width (0 means auto-detect terminal width)
	Theme         string                   // Color theme name (dark, light, dracula, monokai, solarized-dark, solarized-light, plain)
	TableStyle    string                   // Border style: rounded, box, double, ascii, markdown, minimal
	ImageMode     string                   // "auto", "always", "never"
	ImageProtocol mermaid.GraphicsProtocol // "auto", "kitty", "iterm2", "sixel", "none"
	ImageWidth    string                   // "auto", "100%", "80", "400px"
	ImageHeight   string                   // "auto", "20", "300px"
	LineNumbers bool   // Display line numbers in code blocks
	Hyperlinks  bool   // Enable OSC 8 clickable terminal hyperlinks
	Pager       bool   // Enable pager for long output in interactive TTY
	Plain       bool   // Plain text output with no ANSI escape sequences
	Debug       bool   // Output diagnostic / debug logs to stderr
	BasePath     string  // Directory of input markdown file for resolving relative assets
	MermaidMode  string  // "auto", "image", "ansi", "unicode", "ascii", "raw"
	MermaidTheme string  // Diagram theme: dark, default, slate, blueprint, neon, neutral, forest
	MermaidWidth string  // Diagram image width: "auto", "100%", "80", "800px" (default: "auto")
	MermaidBg    string  // Diagram image background: "auto", "dark", "light", "transparent", or "#RRGGBB" (default: "auto")
	MermaidScale float64 // Rasterization scale factor for Mermaid images (default: 2.0 for Retina/HiDPI)
	ConfigFile   string  // Path to loaded config file (e.g. ~/.mdeerc), or empty if none loaded
}

// DefaultOptions returns standard SRE defaults.
func DefaultOptions() Options {
	return Options{
		Width:        0,
		Theme:        "dark",
		TableStyle:    "rounded",
		ImageMode:     "auto",
		ImageProtocol: mermaid.ProtocolAuto,
		ImageWidth:    "auto",
		ImageHeight:  "auto",
		LineNumbers:  false,
		Hyperlinks:   true,
		Pager:        false,
		Plain:        false,
		Debug:        false,
		BasePath:     "",
		MermaidMode:  "auto",
		MermaidTheme: "",
		MermaidWidth: "auto",
		MermaidBg:    "auto",
		MermaidScale: 2.0,
		ConfigFile:   "",
	}
}
