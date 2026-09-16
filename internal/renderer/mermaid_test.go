package renderer

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	mermaid "github.com/smford/golang-mermaid"
	"github.com/smford/mdee/internal/config"
)

func createTestPNG(w, h int, transparent bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if transparent && (x < 5 || y < 5) {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 0})
			} else {
				img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestResolveMermaidBgColor(t *testing.T) {
	tests := []struct {
		bg              string
		theme           string
		shouldComposite bool
		expectedR       uint8
		expectedG       uint8
		expectedB       uint8
		wantErr         bool
	}{
		{"transparent", "dark", false, 0, 0, 0, false},
		{"none", "dark", false, 0, 0, 0, false},
		{"dark", "light", true, 0x1e, 0x1e, 0x2e, false},
		{"light", "dark", true, 0xff, 0xff, 0xff, false},
		{"white", "dark", true, 0xff, 0xff, 0xff, false},
		{"dracula", "dark", true, 0x28, 0x2a, 0x36, false},
		{"slate", "dark", true, 0x22, 0x27, 0x2e, false},
		{"auto", "dark", true, 0x1e, 0x1e, 0x2e, false},
		{"auto", "light", true, 0xff, 0xff, 0xff, false},
		{"auto", "dracula", true, 0x28, 0x2a, 0x36, false},
		{"#123456", "dark", true, 0x12, 0x34, 0x56, false},
		{"#fff", "dark", true, 0xff, 0xff, 0xff, false},
		{"#invalid", "dark", false, 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.bg+"_"+tt.theme, func(t *testing.T) {
			c, shouldComp, err := resolveMermaidBgColor(tt.bg, tt.theme)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for bg %q", tt.bg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if shouldComp != tt.shouldComposite {
				t.Errorf("expected shouldComposite=%v, got %v", tt.shouldComposite, shouldComp)
			}
			if tt.shouldComposite {
				if c.R != tt.expectedR || c.G != tt.expectedG || c.B != tt.expectedB {
					t.Errorf("expected color (%d,%d,%d), got (%d,%d,%d)",
						tt.expectedR, tt.expectedG, tt.expectedB, c.R, c.G, c.B)
				}
				if c.A != 255 {
					t.Errorf("expected opaque alpha (255), got %d", c.A)
				}
			}
		})
	}
}

func TestProcessMermaidImage_CompositingAndScaling(t *testing.T) {
	rawPNG := createTestPNG(100, 50, true)

	opts := config.DefaultOptions()
	opts.MermaidBg = "dark"
	opts.MermaidScale = 2.0
	opts.MermaidWidth = "80"

	out, err := processMermaidImage(rawPNG, mermaid.ProtocolITerm2, opts, 100, false)
	if err != nil {
		t.Fatalf("processMermaidImage failed: %v", err)
	}

	if !strings.HasPrefix(out, "\x1b]1337;File=") {
		t.Errorf("expected iTerm2 escape sequence, got: %s", out[:50])
	}
	if !strings.Contains(out, "width=80") {
		t.Errorf("expected width=80 in escape sequence: %s", out[:100])
	}
	if !strings.Contains(out, "preserveAspectRatio=1") {
		t.Errorf("expected preserveAspectRatio=1 in escape sequence: %s", out[:100])
	}
}

func TestProcessMermaidImage_ResponsiveWidth(t *testing.T) {
	rawPNG := createTestPNG(80, 40, false)

	opts := config.DefaultOptions()
	opts.MermaidWidth = "auto"

	// When terminal width is 100, targetCols = min(96, 85) = 85
	out, err := processMermaidImage(rawPNG, mermaid.ProtocolITerm2, opts, 100, false)
	if err != nil {
		t.Fatalf("processMermaidImage failed: %v", err)
	}
	if !strings.Contains(out, "width=85") {
		t.Errorf("expected responsive width=85 for 100-col terminal, got: %s", out[:100])
	}

	// For Kitty protocol, should format as width=85cell
	kittyOut, err := processMermaidImage(rawPNG, mermaid.ProtocolKitty, opts, 100, false)
	if err != nil {
		t.Fatalf("processMermaidImage for Kitty failed: %v", err)
	}
	if !strings.HasPrefix(kittyOut, "\x1b_G") {
		t.Errorf("expected Kitty APC sequence, got: %s", kittyOut[:50])
	}
	if !strings.Contains(kittyOut, "c=85") {
		t.Errorf("expected c=85 in Kitty control header: %s", kittyOut[:100])
	}
}

func TestProcessMermaidImage_TransparentOption(t *testing.T) {
	rawPNG := createTestPNG(60, 30, true)

	opts := config.DefaultOptions()
	opts.MermaidBg = "transparent"
	opts.MermaidWidth = "100%"

	out, err := processMermaidImage(rawPNG, mermaid.ProtocolITerm2, opts, 80, false)
	if err != nil {
		t.Fatalf("processMermaidImage failed: %v", err)
	}
	if !strings.Contains(out, "width=100%") {
		t.Errorf("expected width=100%% in escape sequence: %s", out[:100])
	}
}
