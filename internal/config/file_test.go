package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigFile(t *testing.T) {
	path := DefaultConfigFile()
	if path == "" {
		t.Skip("User home directory not available")
	}
	if filepath.Base(path) != ".mdeerc" {
		t.Errorf("Expected base name .mdeerc, got %s", filepath.Base(path))
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("User home directory not available")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"~", home},
		{"~/.mdeerc", filepath.Join(home, ".mdeerc")},
		{"/var/log/test.log", "/var/log/test.log"},
		{"relative/path.yaml", "relative/path.yaml"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandPath(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestLoadConfigFile_NonExistent(t *testing.T) {
	// Explicit non-existent file should error
	_, _, err := LoadConfigFile(filepath.Join(t.TempDir(), "missing.yaml"), true)
	if err == nil {
		t.Errorf("Expected error for explicitly missing config file, got nil")
	}

	// Implicit non-existent file should return default options without error
	opts, loaded, err := LoadConfigFile(filepath.Join(t.TempDir(), ".mdeerc"), false)
	if err != nil {
		t.Errorf("Expected no error for implicit missing config file, got: %v", err)
	}
	if loaded {
		t.Errorf("Expected loaded=false for missing config file")
	}
	if opts.Theme != "dark" {
		t.Errorf("Expected default theme 'dark', got %q", opts.Theme)
	}
}

func TestApplyConfigData_FlatYAML(t *testing.T) {
	yamlContent := `
theme: dracula
table-style: box
width: 95
line-numbers: true
hyperlinks: false
pager: true
plain: true
debug: true
images: never
image-width: 80
image-height: 25
mermaid: unicode
mermaid-theme: blueprint
mermaid-width: 90
mermaid-bg: "#1e1e2e"
mermaid-scale: 2.5
`
	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(yamlContent)); err != nil {
		t.Fatalf("ApplyConfigData failed: %v", err)
	}

	if opts.Theme != "dracula" {
		t.Errorf("Expected theme 'dracula', got %q", opts.Theme)
	}
	if opts.TableStyle != "box" {
		t.Errorf("Expected table-style 'box', got %q", opts.TableStyle)
	}
	if opts.Width != 95 {
		t.Errorf("Expected width 95, got %d", opts.Width)
	}
	if !opts.LineNumbers {
		t.Errorf("Expected line-numbers=true")
	}
	if opts.Hyperlinks {
		t.Errorf("Expected hyperlinks=false")
	}
	if !opts.Pager {
		t.Errorf("Expected pager=true")
	}
	if !opts.Plain {
		t.Errorf("Expected plain=true")
	}
	if !opts.Debug {
		t.Errorf("Expected debug=true")
	}
	if opts.ImageMode != "never" {
		t.Errorf("Expected image mode 'never', got %q", opts.ImageMode)
	}
	if opts.ImageWidth != "80" {
		t.Errorf("Expected image width '80', got %q", opts.ImageWidth)
	}
	if opts.ImageHeight != "25" {
		t.Errorf("Expected image height '25', got %q", opts.ImageHeight)
	}
	if opts.MermaidMode != "unicode" {
		t.Errorf("Expected mermaid mode 'unicode', got %q", opts.MermaidMode)
	}
	if opts.MermaidTheme != "blueprint" {
		t.Errorf("Expected mermaid theme 'blueprint', got %q", opts.MermaidTheme)
	}
	if opts.MermaidWidth != "90" {
		t.Errorf("Expected mermaid width '90', got %q", opts.MermaidWidth)
	}
	if opts.MermaidBg != "#1e1e2e" {
		t.Errorf("Expected mermaid bg '#1e1e2e', got %q", opts.MermaidBg)
	}
	if opts.MermaidScale != 2.5 {
		t.Errorf("Expected mermaid scale 2.5, got %f", opts.MermaidScale)
	}
}

func TestApplyConfigData_SnakeCase(t *testing.T) {
	yamlContent := `
table_style: double
line_numbers: true
image_mode: always
image_width: 100%
image_height: 300px
mermaid_mode: ascii
mermaid_theme: slate
mermaid_width: 80
mermaid_bg: transparent
mermaid_scale: 1.5
`
	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(yamlContent)); err != nil {
		t.Fatalf("ApplyConfigData failed: %v", err)
	}

	if opts.TableStyle != "double" {
		t.Errorf("Expected table-style 'double', got %q", opts.TableStyle)
	}
	if !opts.LineNumbers {
		t.Errorf("Expected line-numbers=true")
	}
	if opts.ImageMode != "always" {
		t.Errorf("Expected image mode 'always', got %q", opts.ImageMode)
	}
	if opts.ImageWidth != "100%" {
		t.Errorf("Expected image width '100%%', got %q", opts.ImageWidth)
	}
	if opts.ImageHeight != "300px" {
		t.Errorf("Expected image height '300px', got %q", opts.ImageHeight)
	}
	if opts.MermaidMode != "ascii" {
		t.Errorf("Expected mermaid mode 'ascii', got %q", opts.MermaidMode)
	}
	if opts.MermaidTheme != "slate" {
		t.Errorf("Expected mermaid theme 'slate', got %q", opts.MermaidTheme)
	}
	if opts.MermaidWidth != "80" {
		t.Errorf("Expected mermaid width '80', got %q", opts.MermaidWidth)
	}
	if opts.MermaidBg != "transparent" {
		t.Errorf("Expected mermaid bg 'transparent', got %q", opts.MermaidBg)
	}
	if opts.MermaidScale != 1.5 {
		t.Errorf("Expected mermaid scale 1.5, got %f", opts.MermaidScale)
	}
}

func TestApplyConfigData_NestedSections(t *testing.T) {
	yamlContent := `
theme: monokai
image:
  mode: always
  width: 70
  height: 20
mermaid:
  mode: image
  theme: neutral
  width: 85
  bg: "#333333"
  scale: 3.0
`
	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(yamlContent)); err != nil {
		t.Fatalf("ApplyConfigData failed: %v", err)
	}

	if opts.Theme != "monokai" {
		t.Errorf("Expected theme 'monokai', got %q", opts.Theme)
	}
	if opts.ImageMode != "always" {
		t.Errorf("Expected image mode 'always', got %q", opts.ImageMode)
	}
	if opts.ImageWidth != "70" {
		t.Errorf("Expected image width '70', got %q", opts.ImageWidth)
	}
	if opts.ImageHeight != "20" {
		t.Errorf("Expected image height '20', got %q", opts.ImageHeight)
	}
	if opts.MermaidMode != "image" {
		t.Errorf("Expected mermaid mode 'image', got %q", opts.MermaidMode)
	}
	if opts.MermaidTheme != "neutral" {
		t.Errorf("Expected mermaid theme 'neutral', got %q", opts.MermaidTheme)
	}
	if opts.MermaidWidth != "85" {
		t.Errorf("Expected mermaid width '85', got %q", opts.MermaidWidth)
	}
	if opts.MermaidBg != "#333333" {
		t.Errorf("Expected mermaid bg '#333333', got %q", opts.MermaidBg)
	}
	if opts.MermaidScale != 3.0 {
		t.Errorf("Expected mermaid scale 3.0, got %f", opts.MermaidScale)
	}
}

func TestApplyConfigData_JSON(t *testing.T) {
	jsonContent := `{"theme": "solarized-light", "width": 88, "line-numbers": true}`
	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(jsonContent)); err != nil {
		t.Fatalf("ApplyConfigData failed: %v", err)
	}

	if opts.Theme != "solarized-light" {
		t.Errorf("Expected theme 'solarized-light', got %q", opts.Theme)
	}
	if opts.Width != 88 {
		t.Errorf("Expected width 88, got %d", opts.Width)
	}
	if !opts.LineNumbers {
		t.Errorf("Expected line-numbers=true")
	}
}

func TestApplyConfigData_SyntaxError(t *testing.T) {
	badContent := `theme: [unclosed list`
	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(badContent)); err == nil {
		t.Errorf("Expected syntax error on bad YAML, got nil")
	}
}

func TestLoadConfigFile_RealFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, ".mdeerc")
	content := "theme: dracula\ntable-style: double\nline-numbers: true\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	opts, loaded, err := LoadConfigFile(cfgPath, true)
	if err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}
	if !loaded {
		t.Errorf("Expected loaded=true")
	}
	if opts.Theme != "dracula" {
		t.Errorf("Expected theme 'dracula', got %q", opts.Theme)
	}
	if opts.TableStyle != "double" {
		t.Errorf("Expected table-style 'double', got %q", opts.TableStyle)
	}
	if !opts.LineNumbers {
		t.Errorf("Expected line-numbers=true")
	}
	if opts.ConfigFile != cfgPath {
		t.Errorf("Expected ConfigFile=%q, got %q", cfgPath, opts.ConfigFile)
	}
}

func TestDefaultConfigTemplate_ParsesSuccessfully(t *testing.T) {
	tmpl := DefaultConfigTemplate()
	if len(tmpl) == 0 {
		t.Fatal("Expected non-empty default config template")
	}

	opts := DefaultOptions()
	if err := ApplyConfigData(&opts, []byte(tmpl)); err != nil {
		t.Fatalf("DefaultConfigTemplate failed to parse: %v", err)
	}

	if opts.Theme != "dark" {
		t.Errorf("Expected default theme 'dark', got %q", opts.Theme)
	}
	if opts.TableStyle != "rounded" {
		t.Errorf("Expected default table-style 'rounded', got %q", opts.TableStyle)
	}
	if opts.MermaidScale != 2.0 {
		t.Errorf("Expected default mermaid scale 2.0, got %f", opts.MermaidScale)
	}
	if !opts.Hyperlinks {
		t.Errorf("Expected default hyperlinks=true")
	}
}

func TestWriteDefaultConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "subdir", ".mdeerc")

	// 1. Initial creation succeeds and creates parent directory
	resolved, err := WriteDefaultConfigFile(targetPath, false)
	if err != nil {
		t.Fatalf("WriteDefaultConfigFile failed: %v", err)
	}
	if resolved != targetPath {
		t.Errorf("Expected resolved path %q, got %q", targetPath, resolved)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}
	if string(data) != DefaultConfigTemplate() {
		t.Errorf("File content does not match DefaultConfigTemplate()")
	}

	// 2. Writing again without force should error
	_, err = WriteDefaultConfigFile(targetPath, false)
	if err == nil {
		t.Fatal("Expected error when file already exists without force, got nil")
	}

	// 3. Writing again with force should succeed
	_, err = WriteDefaultConfigFile(targetPath, true)
	if err != nil {
		t.Fatalf("Expected success when force=true, got: %v", err)
	}
}
