package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	mermaid "github.com/smford/golang-mermaid"
)

// Helper to generate a 2x2 PNG in memory
func generateTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{B: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestFetch_Local(t *testing.T) {
	pngData := generateTestPNG(t)
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, pngData, 0600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ctx := context.Background()

	// 1. Absolute path
	got, name, err := Fetch(ctx, imgPath, "", nil)
	if err != nil {
		t.Fatalf("Fetch absolute failed: %v", err)
	}
	if !bytes.Equal(got, pngData) {
		t.Errorf("expected fetched data to match source")
	}
	if name != "test.png" {
		t.Errorf("expected filename test.png, got %s", name)
	}

	// 2. Relative path with basePath
	gotRel, _, err := Fetch(ctx, "test.png", tmpDir, nil)
	if err != nil {
		t.Fatalf("Fetch relative failed: %v", err)
	}
	if !bytes.Equal(gotRel, pngData) {
		t.Errorf("expected fetched relative data to match")
	}

	// 3. Nonexistent file
	_, _, err = Fetch(ctx, "missing.png", tmpDir, nil)
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestFetch_DataURI(t *testing.T) {
	pngData := generateTestPNG(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)
	dataURI := "data:image/png;base64," + b64

	ctx := context.Background()
	got, name, err := Fetch(ctx, dataURI, "", nil)
	if err != nil {
		t.Fatalf("Fetch data URI failed: %v", err)
	}
	if !bytes.Equal(got, pngData) {
		t.Errorf("expected decoded data to match")
	}
	if name != "image.png" {
		t.Errorf("expected name image.png, got %s", name)
	}
}

func TestFetch_Remote(t *testing.T) {
	pngData := generateTestPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logo.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	ctx := context.Background()

	// Success case
	got, name, err := Fetch(ctx, server.URL+"/logo.png", "", server.Client())
	if err != nil {
		t.Fatalf("Fetch remote failed: %v", err)
	}
	if !bytes.Equal(got, pngData) {
		t.Errorf("expected fetched remote bytes to match")
	}
	if name != "logo.png" {
		t.Errorf("expected name logo.png, got %s", name)
	}

	// 404 case
	_, _, err = Fetch(ctx, server.URL+"/notfound.png", "", server.Client())
	if err == nil {
		t.Errorf("expected 404 error, got nil")
	}
}

func TestGetDimensions(t *testing.T) {
	pngData := generateTestPNG(t)
	info, err := GetDimensions(pngData)
	if err != nil {
		t.Fatalf("GetDimensions failed: %v", err)
	}
	if info.Width != 2 || info.Height != 2 {
		t.Errorf("expected 2x2, got %dx%d", info.Width, info.Height)
	}
	if info.Format != "png" {
		t.Errorf("expected format png, got %s", info.Format)
	}
}

func TestFormatITerm2(t *testing.T) {
	pngData := []byte("fake-png-payload")
	opts := ITerm2Options{
		Width:               "auto",
		Height:              "auto",
		PreserveAspectRatio: true,
		InTmux:              false,
	}

	seq := FormatITerm2(pngData, "diagram.png", opts)
	if !strings.HasPrefix(seq, "\x1b]1337;File=") {
		t.Errorf("expected OSC 1337 prefix, got %q", seq)
	}
	if !strings.HasSuffix(seq, "\a") {
		t.Errorf("expected BEL termination, got %q", seq)
	}
	if !strings.Contains(seq, "inline=1") {
		t.Errorf("expected inline=1")
	}

	// Test tmux DCS wrapper
	opts.InTmux = true
	tmuxSeq := FormatITerm2(pngData, "diagram.png", opts)
	if !strings.HasPrefix(tmuxSeq, "\x1bPtmux;") {
		t.Errorf("expected tmux DCS prefix, got %q", tmuxSeq)
	}
	if !strings.HasSuffix(tmuxSeq, "\x1b\\") {
		t.Errorf("expected tmux DCS suffix, got %q", tmuxSeq)
	}
}

func TestFormatFallback(t *testing.T) {
	tests := []struct {
		name     string
		alt      string
		src      string
		errMsg   string
		maxWidth int
	}{
		{
			name:     "standard fallback without error",
			alt:      "Architecture Diagram",
			src:      "https://example.com/arch.png",
			errMsg:   "",
			maxWidth: 80,
		},
		{
			name:     "error fallback with long path and error message",
			alt:      "Missing Asset Test",
			src:      "./assets/does-not-exist-for-testing.png",
			errMsg:   "file not found: stat /Users/asc/git/md/assets/does-not-exist-for-testing.png: no such file or directory",
			maxWidth: 80,
		},
		{
			name:     "constrained terminal width",
			alt:      "Very Long Alternative Description For An Important Image In The System",
			src:      "/var/log/systems/very/deep/nested/path/to/an/image/file/that/might/wrap.png",
			errMsg:   "permission denied: access restricted to admin role",
			maxWidth: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := FormatFallback(tt.alt, tt.src, tt.errMsg, tt.maxWidth)

			lines := strings.Split(out, "\n")
			if len(lines) < 3 {
				t.Fatalf("expected at least 3 lines, got %d", len(lines))
			}

			// Verify rounded corners
			if !strings.HasPrefix(lines[0], "╭") || !strings.HasSuffix(lines[0], "╮") {
				t.Errorf("top border must start with ╭ and end with ╮, got: %q", lines[0])
			}
			lastLine := lines[len(lines)-1]
			if !strings.HasPrefix(lastLine, "╰") || !strings.HasSuffix(lastLine, "╯") {
				t.Errorf("bottom border must start with ╰ and end with ╯, got: %q", lastLine)
			}

			// Verify all middle lines have vertical borders │ ... │
			for i := 1; i < len(lines)-1; i++ {
				line := lines[i]
				if !strings.HasPrefix(line, "│ ") || !strings.HasSuffix(line, " │") {
					t.Errorf("line %d does not have │ ... │ borders: %q", i, line)
				}
			}

			// Verify visual width symmetry: every line must have the exact same visual width
			topWidth := uniseg.StringWidth(lines[0])
			for i, line := range lines {
				w := uniseg.StringWidth(line)
				if w != topWidth {
					t.Errorf("line %d visual width %d does not match top border width %d: %q", i, w, topWidth, line)
				}
			}

			// Check content presence
			if !strings.Contains(out, tt.src) && !strings.Contains(out, "...") {
				// note: long src might be wrapped across lines, check fields
				words := strings.Fields(tt.src)
				for _, word := range words {
					if !strings.Contains(out, word) {
						t.Errorf("expected output to contain %q", word)
					}
				}
			}
			if tt.errMsg != "" && !strings.Contains(out, "Image Error") {
				t.Errorf("expected output to contain Image Error, got: %s", out)
			}
		})
	}
}

func TestEnsurePNG(t *testing.T) {
	pngData := generateTestPNG(t)
	got, err := EnsurePNG(pngData)
	if err != nil {
		t.Fatalf("EnsurePNG failed on valid PNG: %v", err)
	}
	if !bytes.Equal(got, pngData) {
		t.Errorf("EnsurePNG should return identical slice for PNG")
	}

	// Test with JPEG
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 200, G: 50, B: 50, A: 255})
	var jpgBuf bytes.Buffer
	if err := png.Encode(&jpgBuf, img); err != nil {
		t.Fatalf("failed encoding image: %v", err)
	}
	converted, err := EnsurePNG(jpgBuf.Bytes())
	if err != nil {
		t.Fatalf("EnsurePNG failed on image: %v", err)
	}
	if len(converted) == 0 {
		t.Errorf("expected non-empty converted PNG")
	}
}

func TestFormatTerminal(t *testing.T) {
	pngData := generateTestPNG(t)

	// 1. Kitty protocol
	kittySeq, err := FormatTerminal(pngData, "test.png", mermaid.ProtocolKitty, TerminalOptions{
		Width:  "60",
		Height: "20",
	})
	if err != nil {
		t.Fatalf("FormatTerminal for Kitty failed: %v", err)
	}
	if !strings.Contains(kittySeq, "\033_G") {
		t.Errorf("expected Kitty APC escape sequence, got: %s", kittySeq)
	}

	// 2. iTerm2 protocol
	itermSeq, err := FormatTerminal(pngData, "test.png", mermaid.ProtocolITerm2, TerminalOptions{
		Width: "auto",
	})
	if err != nil {
		t.Fatalf("FormatTerminal for iTerm2 failed: %v", err)
	}
	if !strings.Contains(itermSeq, "\x1b]1337;File=") {
		t.Errorf("expected iTerm2 OSC 1337 escape sequence, got: %s", itermSeq)
	}

	// 3. Sixel protocol
	sixelSeq, err := FormatTerminal(pngData, "test.png", mermaid.ProtocolSixel, TerminalOptions{})
	if err != nil {
		t.Fatalf("FormatTerminal for Sixel failed: %v", err)
	}
	if !strings.Contains(sixelSeq, "\033Pq") {
		t.Errorf("expected Sixel DCS escape sequence, got: %s", sixelSeq)
	}

	// 4. Unsupported / None protocol returns error
	_, err = FormatTerminal(pngData, "test.png", mermaid.ProtocolNone, TerminalOptions{})
	if err == nil {
		t.Errorf("expected error for ProtocolNone")
	}
}

func TestFetch_AdvancedFeatures(t *testing.T) {
	pngData := generateTestPNG(t)
	tmpDir := t.TempDir()

	// 1. File with space in name
	spacedPath := filepath.Join(tmpDir, "my test image.png")
	if err := os.WriteFile(spacedPath, pngData, 0600); err != nil {
		t.Fatalf("failed writing spaced file: %v", err)
	}

	ctx := context.Background()

	// Fetch using URL encoding: my%20test%20image.png
	got, name, err := Fetch(ctx, "my%20test%20image.png", tmpDir, nil)
	if err != nil {
		t.Fatalf("Fetch url-encoded failed: %v", err)
	}
	if !bytes.Equal(got, pngData) || name != "my test image.png" {
		t.Errorf("expected unescaped file match, got name %s", name)
	}

	// Fetch with GitHub raw query param: my test image.png?raw=true#section
	gotQuery, _, err := Fetch(ctx, "my test image.png?raw=true#section", tmpDir, nil)
	if err != nil {
		t.Fatalf("Fetch with query param failed: %v", err)
	}
	if !bytes.Equal(gotQuery, pngData) {
		t.Errorf("expected query param stripped match")
	}

	// 2. Remote server tests for protocol-relative, remote basePath, and caching
	hits := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if strings.HasSuffix(r.URL.Path, "/logo.png") {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Protocol relative URL (prepends https:)
	protoRelURL := "//" + strings.TrimPrefix(server.URL, "https://") + "/logo.png"
	gotProto, _, err := Fetch(ctx, protoRelURL, "", server.Client())
	if err != nil {
		t.Fatalf("Fetch protocol-relative failed: %v", err)
	}
	if !bytes.Equal(gotProto, pngData) {
		t.Errorf("expected protocol-relative match")
	}

	// Relative URL resolved against remote basePath
	baseURL := server.URL + "/docs/"
	gotBase, _, err := Fetch(ctx, "logo.png", baseURL, server.Client())
	if err != nil {
		t.Fatalf("Fetch with remote basePath failed: %v", err)
	}
	if !bytes.Equal(gotBase, pngData) {
		t.Errorf("expected remote basePath match")
	}

	// In-memory cache verification: second fetch of same URL should not hit server
	initialHits := hits
	cachedURL := server.URL + "/logo.png"
	_, _, err = Fetch(ctx, cachedURL, "", server.Client())
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	hitsAfterFirst := hits
	_, _, err = Fetch(ctx, cachedURL, "", server.Client())
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if hits != hitsAfterFirst {
		t.Errorf("expected cached response without incrementing hits: initial=%d, afterFirst=%d, afterSecond=%d",
			initialHits, hitsAfterFirst, hits)
	}
}
