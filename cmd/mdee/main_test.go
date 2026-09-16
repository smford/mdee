package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smford/mdee/internal/config"
	"github.com/smford/mdee/internal/table"
	"github.com/smford/mdee/internal/term"
)

func TestCLIVersion(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestCLIDoctor(t *testing.T) {
	cmd := newDoctorCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}
}

func TestRenderAndOutput(t *testing.T) {
	opts := config.DefaultOptions()
	opts.Plain = true
	info := term.Info{IsTTY: false, Width: 80, Height: 24}

	doc := "# Test Header\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"
	var buf bytes.Buffer

	// Temporarily redirect stdout during render test
	ctx := context.Background()
	err := renderAndOutput(ctx, opts, info, []byte(doc))
	if err != nil {
		t.Fatalf("renderAndOutput error: %v", err)
	}
	_ = buf
}

func TestTableAlignmentInCLI(t *testing.T) {
	opts := config.DefaultOptions()
	opts.Width = 60
	opts.TableStyle = "box"

	doc := `
| Left | Center | Right |
| :--- | :---: | ---: |
| L | C | R |
`
	ctx := context.Background()
	opts.Plain = true
	var buf strings.Builder
	r := table.New([]string{"Left", "Center", "Right"})
	r.SetAlignments([]table.Alignment{table.AlignLeft, table.AlignCenter, table.AlignRight})
	r.AddRow("L", "C", "R")
	r.Border = table.BorderBox
	r.MaxWidth = 60
	rendered := r.Render()
	buf.WriteString(rendered)

	if !strings.Contains(buf.String(), "┌") || !strings.Contains(buf.String(), "┘") {
		t.Errorf("expected box borders in rendered table")
	}
	_ = ctx
	_ = doc
}

func TestCLIMermaidFlags(t *testing.T) {
	t.Run("valid mermaid modes", func(t *testing.T) {
		modes := []string{"auto", "image", "ansi", "unicode", "ascii", "raw"}
		for _, m := range modes {
			cmd := newRootCmd()
			cmd.SetArgs([]string{"--mermaid", m, "--version"})
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			if err := cmd.Execute(); err != nil {
				t.Errorf("expected mode %q to be accepted, got err: %v", m, err)
			}
		}
	})

	t.Run("invalid mermaid mode returns error", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--mermaid", "invalid-mode", "testdata/demo.md"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err == nil {
			t.Errorf("expected error for invalid mermaid mode")
		}
		if !strings.Contains(err.Error(), "invalid mermaid mode") {
			t.Errorf("expected 'invalid mermaid mode' in error message: %v", err)
		}
	})

	t.Run("mermaid theme and scale flags", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--mermaid-theme", "slate", "--mermaid-scale", "2.0", "--version"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := cmd.Execute(); err != nil {
			t.Errorf("expected theme and scale flags to be accepted, got: %v", err)
		}
	})

	t.Run("mermaid width and background flags", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--mermaid-width", "80", "--mermaid-bg", "#1e1e2e", "--version"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := cmd.Execute(); err != nil {
			t.Errorf("expected width and bg flags to be accepted, got: %v", err)
		}

		cmdAlias := newRootCmd()
		cmdAlias.SetArgs([]string{"--mermaid-background", "dark", "--version"})
		var bufAlias bytes.Buffer
		cmdAlias.SetOut(&bufAlias)
		if err := cmdAlias.Execute(); err != nil {
			t.Errorf("expected mermaid-background alias to be accepted, got: %v", err)
		}
	})
}

func TestCLIConfigFile(t *testing.T) {
	t.Run("explicit config file loads settings and can be overridden by flags", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, ".mdeerc")
		content := `
theme: monokai
table-style: box
width: 75
line-numbers: true
mermaid:
  mode: ascii
`
		if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write test config file: %v", err)
		}

		mdPath := filepath.Join(tmpDir, "test.md")
		if err := os.WriteFile(mdPath, []byte("# Hello\n\nContent\n"), 0600); err != nil {
			t.Fatalf("failed to write test markdown file: %v", err)
		}

		// Test executing with --config and overriding theme
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", cfgPath, "--theme", "light", "--plain", mdPath})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("expected successful execution with --config, got: %v", err)
		}
	})

	t.Run("non-existent explicit config returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		mdPath := filepath.Join(tmpDir, "dummy.md")
		_ = os.WriteFile(mdPath, []byte("# Dummy\n"), 0600)
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", "/nonexistent/path/.mdeerc", mdPath})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err == nil {
			t.Errorf("expected error for non-existent config file")
		}
		if !strings.Contains(err.Error(), "config file not found") {
			t.Errorf("expected 'config file not found' in error, got: %v", err)
		}
	})

	t.Run("doctor with config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, ".mdeerc")
		if err := os.WriteFile(cfgPath, []byte("theme: dracula\n"), 0600); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		cmd := newDoctorCmd()
		cmd.SetArgs([]string{"--config", cfgPath})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("expected doctor with --config to succeed, got: %v", err)
		}
	})
}
