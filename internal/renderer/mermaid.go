package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"

	mermaid "github.com/smford/golang-mermaid"
	"github.com/smford/mdee/internal/config"
	"golang.org/x/image/draw"
)

// Default background colors for Mermaid diagram cards
var (
	defaultDarkBg  = color.RGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff} // Catppuccin Mocha / dark slate
	defaultLightBg = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // Pure white
	draculaBg      = color.RGBA{R: 0x28, G: 0x2a, B: 0x36, A: 0xff} // Dracula background
	slateBg        = color.RGBA{R: 0x22, G: 0x27, B: 0x2e, A: 0xff} // Slate / GitHub Dark Dimmed
)

// resolveMermaidBgColor resolves the requested background color into an RGBA color.
// Returns (color, shouldComposite, error).
func resolveMermaidBgColor(bgStr, theme string) (color.RGBA, bool, error) {
	clean := strings.ToLower(strings.TrimSpace(bgStr))
	if clean == "transparent" || clean == "none" {
		return color.RGBA{}, false, nil
	}

	// Direct hex color check (#RRGGBB or #RGB)
	if strings.HasPrefix(clean, "#") {
		c, ok := parseHexColor(clean)
		if ok {
			return c, true, nil
		}
		return color.RGBA{}, false, fmt.Errorf("invalid hex color %q", bgStr)
	}

	// Named colors
	switch clean {
	case "dark":
		return defaultDarkBg, true, nil
	case "dracula":
		return draculaBg, true, nil
	case "slate":
		return slateBg, true, nil
	case "light", "white":
		return defaultLightBg, true, nil
	case "auto", "":
		// Pick background based on mdee theme
		switch strings.ToLower(theme) {
		case "light", "solarized-light":
			return defaultLightBg, true, nil
		case "dracula":
			return draculaBg, true, nil
		default:
			return defaultDarkBg, true, nil
		}
	default:
		// Attempt hex parse without # prefix
		if c, ok := parseHexColor(clean); ok {
			return c, true, nil
		}
		// Fallback to dark
		return defaultDarkBg, true, nil
	}
}

func parseHexColor(s string) (color.RGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		r, err1 := strconv.ParseUint(string(s[0])+string(s[0]), 16, 8)
		g, err2 := strconv.ParseUint(string(s[1])+string(s[1]), 16, 8)
		b, err3 := strconv.ParseUint(string(s[2])+string(s[2]), 16, 8)
		if err1 == nil && err2 == nil && err3 == nil {
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
		}
	} else if len(s) == 6 {
		r, err1 := strconv.ParseUint(s[0:2], 16, 8)
		g, err2 := strconv.ParseUint(s[2:4], 16, 8)
		b, err3 := strconv.ParseUint(s[4:6], 16, 8)
		if err1 == nil && err2 == nil && err3 == nil {
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
		}
	}
	return color.RGBA{}, false
}

// processMermaidImage enhances a raw Mermaid PNG diagram by:
// 1. Compositing transparent pixels onto a solid, high-contrast canvas background
//    (eliminating terminal background color bleed or dark-on-dark unreadability).
// 2. Adding clean margin padding around the diagram nodes.
// 3. Resampling at HiDPI scale (CatmullRom filter) for razor-sharp typography.
// 4. Calculating a responsive, legible terminal column width (e.g. 70-85 cells)
//    so the image is prominently visible instead of a tiny thumbnail.
func processMermaidImage(
	rawPNG []byte,
	proto mermaid.GraphicsProtocol,
	opts config.Options,
	termWidth int,
	inTmux bool,
) (string, error) {
	if len(rawPNG) == 0 {
		return "", fmt.Errorf("empty PNG data")
	}

	finalPNG := rawPNG

	bgColor, shouldComposite, err := resolveMermaidBgColor(opts.MermaidBg, opts.Theme)
	if err == nil && shouldComposite {
		srcImg, _, decErr := image.Decode(bytes.NewReader(rawPNG))
		if decErr == nil {
			srcBounds := srcImg.Bounds()
			sw, sh := srcBounds.Dx(), srcBounds.Dy()

			pad := 20
			scale := opts.MermaidScale
			if scale <= 0 {
				scale = 2.0
			}
			if scale > 4.0 {
				scale = 4.0
			}

			targetW := int(float64(sw+pad*2) * scale)
			targetH := int(float64(sh+pad*2) * scale)

			dstImg := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
			draw.Draw(dstImg, dstImg.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

			destRect := image.Rect(
				int(float64(pad)*scale),
				int(float64(pad)*scale),
				int(float64(sw+pad)*scale),
				int(float64(sh+pad)*scale),
			)
			draw.CatmullRom.Scale(dstImg, destRect, srcImg, srcBounds, draw.Over, nil)

			var buf bytes.Buffer
			if encErr := png.Encode(&buf, dstImg); encErr == nil {
				finalPNG = buf.Bytes()
			}
		}
	}

	// Calculate display width constraint
	wStr := opts.MermaidWidth
	if wStr == "" || wStr == "auto" {
		wStr = opts.ImageWidth
	}
	if wStr == "" || wStr == "auto" {
		cols := termWidth
		if cols <= 0 {
			cols = 80
		}
		targetCols := cols - 4
		if targetCols > 85 {
			targetCols = 85
		}
		if targetCols < 45 {
			targetCols = cols
		}

		if proto == mermaid.ProtocolKitty {
			wStr = fmt.Sprintf("%dcell", targetCols)
		} else {
			// For iTerm2 OSC 1337, bare integer denotes character columns
			wStr = fmt.Sprintf("%d", targetCols)
		}
	} else {
		// If user specified a plain number for Kitty, append "cell"
		if proto == mermaid.ProtocolKitty && !strings.HasSuffix(wStr, "cell") && !strings.HasSuffix(wStr, "%") && !strings.HasSuffix(wStr, "px") {
			if _, err := strconv.Atoi(wStr); err == nil {
				wStr = wStr + "cell"
			}
		}
	}

	hStr := opts.ImageHeight
	if hStr == "" {
		hStr = "auto"
	}

	fmtOpts := mermaid.ImageFormatOptions{
		Inline:              true,
		Width:               wStr,
		Height:              hStr,
		PreserveAspectRatio: true,
		TmuxPassthrough:     inTmux,
	}

	return mermaid.FormatTerminalImage(proto, finalPNG, fmtOpts)
}
