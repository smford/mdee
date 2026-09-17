package term

import (
	"os"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Run("iTerm2 detection", func(t *testing.T) {
		os.Setenv("TERM_PROGRAM", "iTerm.app")
		defer os.Unsetenv("TERM_PROGRAM")

		info := Detect()
		if !info.IsITerm2 {
			t.Errorf("expected IsITerm2 to be true when TERM_PROGRAM=iTerm.app")
		}
		if !info.HasOSC1337 {
			t.Errorf("expected HasOSC1337 to be true for iTerm2")
		}
		if !info.HasOSC8 {
			t.Errorf("expected HasOSC8 to be true for iTerm2")
		}
		if info.Width <= 0 {
			t.Errorf("expected positive width, got %d", info.Width)
		}
	})

	t.Run("Kitty detection", func(t *testing.T) {
		os.Setenv("TERM", "xterm-kitty")
		os.Setenv("KITTY_WINDOW_ID", "1")
		defer os.Unsetenv("TERM")
		defer os.Unsetenv("KITTY_WINDOW_ID")

		info := Detect()
		if !info.IsKitty {
			t.Errorf("expected IsKitty to be true")
		}
		if !info.HasKittyGraphics {
			t.Errorf("expected HasKittyGraphics to be true")
		}
	})

	t.Run("Ghostty detection", func(t *testing.T) {
		os.Setenv("TERM_PROGRAM", "ghostty")
		defer os.Unsetenv("TERM_PROGRAM")

		info := Detect()
		if !info.IsGhostty {
			t.Errorf("expected IsGhostty to be true")
		}
		if !info.HasKittyGraphics {
			t.Errorf("expected HasKittyGraphics to be true for Ghostty")
		}
	})

	t.Run("WezTerm detection", func(t *testing.T) {
		os.Setenv("TERM_PROGRAM", "WezTerm")
		defer os.Unsetenv("TERM_PROGRAM")

		info := Detect()
		if !info.IsWezTerm {
			t.Errorf("expected IsWezTerm to be true")
		}
		if !info.HasOSC1337 {
			t.Errorf("expected HasOSC1337 to be true for WezTerm")
		}
	})

	t.Run("Foot sixel detection", func(t *testing.T) {
		os.Setenv("TERM", "foot")
		defer os.Unsetenv("TERM")

		info := Detect()
		if !info.IsFoot {
			t.Errorf("expected IsFoot to be true")
		}
		if !info.HasSixel {
			t.Errorf("expected HasSixel to be true for Foot")
		}
	})
}

func TestFormatHyperlink(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		text    string
		enable  bool
		want    string
	}{
		{
			name:   "disabled",
			url:    "https://example.com",
			text:   "Example",
			enable: false,
			want:   "Example",
		},
		{
			name:   "enabled",
			url:    "https://example.com",
			text:   "Example",
			enable: true,
			want:   "\x1b]8;;https://example.com\aExample\x1b]8;;\a",
		},
		{
			name:   "empty url",
			url:    "",
			text:   "Just Text",
			enable: true,
			want:   "Just Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatHyperlink(tt.url, tt.text, tt.enable)
			if got != tt.want {
				t.Errorf("FormatHyperlink() = %q, want %q", got, tt.want)
			}
		})
	}
}
