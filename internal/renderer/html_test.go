package renderer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	mermaid "github.com/smford/golang-mermaid"
	"github.com/smford/mdee/internal/config"
	"github.com/smford/mdee/internal/table"
)

func TestRenderer_HTMLImage(t *testing.T) {
	t.Run("standalone html img with kitty protocol", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolKitty
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" alt="Kitty HTML Image" width="400" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "\x1b_G") {
			t.Errorf("expected Kitty APC sequence in output: %q", out)
		}
		if !strings.Contains(out, "Kitty HTML Image") {
			t.Errorf("expected caption in output: %q", out)
		}
	})

	t.Run("standalone html img with iterm2 protocol", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" alt="iTerm2 HTML Image" width="300" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected iTerm2 OSC 1337 sequence in output: %q", out)
		}
		if !strings.Contains(out, "iTerm2 HTML Image") {
			t.Errorf("expected caption in output: %q", out)
		}
	})

	t.Run("standalone html img with sixel protocol", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolSixel
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" alt="Sixel HTML Image" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "\x1bPq") {
			t.Errorf("expected Sixel DCS sequence in output: %q", out)
		}
	})

	t.Run("html img in center paragraph strips container tags", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<p align="center">
  <img src="../../testdata/sample.png" alt="Centered Architecture" />
</p>`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if strings.Contains(out, "<p align=\"center\">") || strings.Contains(out, "</p>") {
			t.Errorf("container tags should be stripped from image output: %q", out)
		}
		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected inline image sequence: %q", out)
		}
		if !strings.Contains(out, "Centered Architecture") {
			t.Errorf("expected caption in output: %q", out)
		}
	})

	t.Run("html img wrapped in div and anchor link", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<div align="center">
  <a href="https://example.com/project">
    <img src="../../testdata/sample.png" alt="Linked App Banner" />
  </a>
</div>`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if strings.Contains(out, "<div") || strings.Contains(out, "<a") || strings.Contains(out, "</div>") {
			t.Errorf("HTML wrapper tags should be stripped from image output: %q", out)
		}
		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected inline image sequence: %q", out)
		}
		if !strings.Contains(out, "Linked App Banner") {
			t.Errorf("expected caption: %q", out)
		}
	})

	t.Run("relative path resolved against opts.BasePath", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		// Point BasePath to testdata
		absTestdata, err := filepath.Abs("../../testdata")
		if err != nil {
			t.Fatalf("failed to get abs path: %v", err)
		}
		opts.BasePath = absTestdata
		r := New(opts)

		// Reference sample.png without leading path
		doc := `<img src="sample.png" alt="Basepath Image" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected successful image rendering using BasePath: %q", out)
		}
	})

	t.Run("local path with query params and fragments stripped", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<img src="../../testdata/sample.png?raw=true#version1" alt="Query Param Image" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected image to load despite ?raw=true query param: %q", out)
		}
	})

	t.Run("inline html img with surrounding text", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `Here is the diagram: <img src="../../testdata/sample.png" alt="Inline Diagram"> in text.`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "Here is the diagram:") {
			t.Errorf("expected leading text: %q", out)
		}
		if !strings.Contains(out, "\x1b]1337;File=") {
			t.Errorf("expected image escape sequence: %q", out)
		}
		if !strings.Contains(out, "in text.") {
			t.Errorf("expected trailing text: %q", out)
		}
	})

	t.Run("figure with figcaption fallback for alt text", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<figure>
  <img src="../../testdata/sample.png" />
  <figcaption>Figure 1: Telemetry Dashboard</figcaption>
</figure>`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "Figure 1: Telemetry Dashboard") {
			t.Errorf("expected figcaption to provide image caption: %q", out)
		}
		if strings.Contains(out, "<figure>") || strings.Contains(out, "<figcaption>") {
			t.Errorf("HTML wrapper tags should be stripped: %q", out)
		}
	})

	t.Run("html comment containing img is not rendered", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		r := New(opts)

		doc := `<!-- <img src="nonexistent_comment.png" alt="Hidden"> -->`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if strings.Contains(out, "Image Error") || strings.Contains(out, "╭───") {
			t.Errorf("commented out img must not generate error card: %q", out)
		}
		if !strings.Contains(out, "nonexistent_comment.png") {
			t.Errorf("comment text should be preserved: %q", out)
		}
	})

	t.Run("images never mode outputs fallback card for html img", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "never"
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" alt="Fallback Card Test" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "Fallback Card Test") || !strings.Contains(out, "sample.png") {
			t.Errorf("expected fallback card in output: %q", out)
		}
	})

	t.Run("broken image outputs diagnostic error card", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		r := New(opts)

		doc := `<img src="missing_file_xyz_123.png" alt="Missing Asset" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if !strings.Contains(out, "Image Error") || !strings.Contains(out, "Missing Asset") {
			t.Errorf("expected error fallback card: %q", out)
		}
		if !strings.Contains(out, "file not found") {
			t.Errorf("expected file not found reason: %q", out)
		}
	})

	t.Run("plain mode outputs clean text without ansi escape sequences", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.Plain = true
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" alt="Plain Image" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		if strings.Contains(out, "\x1b[") {
			t.Errorf("plain mode should not have ANSI escape codes: %q", out)
		}
		clean := table.StripANSI(out)
		if !strings.Contains(clean, "Plain Image") {
			t.Errorf("expected image text in plain output: %q", out)
		}
	})

	t.Run("style attribute width and height parsing", func(t *testing.T) {
		opts := config.DefaultOptions()
		opts.ImageMode = "always"
		opts.ImageProtocol = mermaid.ProtocolITerm2
		r := New(opts)

		doc := `<img src="../../testdata/sample.png" style="width: 400px; height: 250px" alt="Styled Image" />`
		out, err := r.Render(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}

		// iTerm2 protocol should contain width=400px and height=250px
		if !strings.Contains(out, "width=400px") || !strings.Contains(out, "height=250px") {
			t.Errorf("expected style width/height passed to iTerm2 sequence: %q", out)
		}
	})
}
