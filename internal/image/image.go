package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rivo/uniseg"
	mermaid "github.com/smford/golang-mermaid"
	_ "golang.org/x/image/webp"
)

const (
	// MaxImageSize limits downloaded image payload to 25MB to prevent memory exhaustion.
	MaxImageSize = 25 * 1024 * 1024
	// DefaultTimeout defines HTTP fetch timeout for remote images.
	DefaultTimeout = 10 * time.Second
)

// Info contains metadata about an image.
type Info struct {
	Width    int
	Height   int
	Format   string
	ByteSize int
}

type cachedRemoteImage struct {
	data     []byte
	filename string
}

var remoteCache sync.Map

// Fetch loads an image from a local file, HTTP/HTTPS URL, or base64 data URI.
// If src is a relative path, it will be resolved relative to basePath.
func Fetch(ctx context.Context, src string, basePath string, client *http.Client) ([]byte, string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, "", errors.New("empty image source")
	}

	// 1. Data URI: data:image/png;base64,...
	if strings.HasPrefix(src, "data:") {
		return parseDataURI(src)
	}

	// 2. Protocol-relative URL: //example.com/image.png
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}

	// 3. HTTP/HTTPS URL
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return fetchRemote(ctx, src, client)
	}

	// 4. Remote basePath resolution: if basePath is an HTTP/HTTPS URL and src is relative
	if (strings.HasPrefix(basePath, "http://") || strings.HasPrefix(basePath, "https://")) &&
		!filepath.IsAbs(src) && !strings.HasPrefix(src, "/") {
		if baseURL, err := url.Parse(basePath); err == nil {
			if relURL, err := url.Parse(src); err == nil {
				resolved := baseURL.ResolveReference(relURL).String()
				return fetchRemote(ctx, resolved, client)
			}
		}
	}

	// 5. Local filesystem
	return fetchLocal(src, basePath)
}

func parseDataURI(dataURI string) ([]byte, string, error) {
	commaIdx := strings.Index(dataURI, ",")
	if commaIdx == -1 {
		return nil, "", errors.New("invalid data URI format")
	}

	meta := dataURI[:commaIdx]
	dataStr := dataURI[commaIdx+1:]

	var filename string
	if strings.Contains(meta, "image/png") {
		filename = "image.png"
	} else if strings.Contains(meta, "image/jpeg") {
		filename = "image.jpg"
	} else if strings.Contains(meta, "image/gif") {
		filename = "image.gif"
	} else if strings.Contains(meta, "image/webp") {
		filename = "image.webp"
	} else if strings.Contains(meta, "image/svg") {
		filename = "image.svg"
	} else {
		filename = "image.bin"
	}

	if strings.Contains(meta, ";base64") {
		data, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode base64 data URI: %w", err)
		}
		return data, filename, nil
	}

	// URL-encoded or raw data
	unescaped, err := url.QueryUnescape(dataStr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unescape data URI: %w", err)
	}
	return []byte(unescaped), filename, nil
}

func fetchRemote(ctx context.Context, rawURL string, client *http.Client) ([]byte, string, error) {
	if val, ok := remoteCache.Load(rawURL); ok {
		entry := val.(cachedRemoteImage)
		return entry.data, entry.filename, nil
	}

	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("invalid request: %w", err)
	}

	// SRE Best Practice: Set descriptive User-Agent so remote CDNs don't block request
	req.Header.Set("User-Agent", "md-terminal-viewer/1.0 (Darwin/macOS; iTerm2)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("http error %d: %s", resp.StatusCode, resp.Status)
	}

	// Enforce max size limit to prevent memory exhaustion
	limitReader := io.LimitReader(resp.Body, MaxImageSize+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, "", fmt.Errorf("reading response body: %w", err)
	}
	if len(data) > MaxImageSize {
		return nil, "", fmt.Errorf("image exceeds maximum allowed size of %d bytes", MaxImageSize)
	}

	// Extract filename from URL path
	filename := "image.png"
	if parsed, err := url.Parse(rawURL); err == nil {
		base := filepath.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			filename = base
		}
	}

	remoteCache.Store(rawURL, cachedRemoteImage{data: data, filename: filename})
	return data, filename, nil
}

func fetchLocal(filePath, basePath string) ([]byte, string, error) {
	// Support tilde home directory expansion (~/...)
	targetPath := filePath
	if strings.HasPrefix(targetPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			targetPath = filepath.Join(home, targetPath[2:])
		}
	} else if !filepath.IsAbs(targetPath) && basePath != "" {
		targetPath = filepath.Join(basePath, targetPath)
	}

	cleanPath := filepath.Clean(targetPath)
	stat, err := os.Stat(cleanPath)
	if err != nil {
		// Attempt stripping query params and fragments (common in GitHub READMEs, e.g. diagram.png?raw=true)
		stripped := targetPath
		if qIdx := strings.IndexAny(stripped, "?#"); qIdx != -1 {
			stripped = stripped[:qIdx]
		}
		// Also attempt URL path unescaping (e.g. %20 -> space)
		if unescaped, uerr := url.PathUnescape(stripped); uerr == nil {
			stripped = unescaped
		}
		cleanStripped := filepath.Clean(stripped)
		if sStat, sErr := os.Stat(cleanStripped); sErr == nil {
			cleanPath = cleanStripped
			stat = sStat
			err = nil
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("file not found: %w", err)
	}
	if stat.IsDir() {
		return nil, "", fmt.Errorf("path is a directory: %s", cleanPath)
	}
	if stat.Size() > MaxImageSize {
		return nil, "", fmt.Errorf("image exceeds maximum allowed size (%d bytes)", MaxImageSize)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading image file: %w", err)
	}

	return data, filepath.Base(cleanPath), nil
}

// GetDimensions reads the intrinsic width, height, and format of the image bytes.
func GetDimensions(data []byte) (Info, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Info{ByteSize: len(data)}, err
	}
	return Info{
		Width:    cfg.Width,
		Height:   cfg.Height,
		Format:   format,
		ByteSize: len(data),
	}, nil
}

// ITerm2Options configures the inline image rendering for iTerm2.
type ITerm2Options struct {
	Width               string // e.g. "auto", "100%", "80", "400px"
	Height              string // e.g. "auto", "20", "300px"
	PreserveAspectRatio bool   // default true
	InTmux              bool   // wraps in tmux DCS passthrough
}

// FormatITerm2 generates the OSC 1337 escape sequence for displaying an inline image.
func FormatITerm2(data []byte, filename string, opts ITerm2Options) string {
	b64Data := base64.StdEncoding.EncodeToString(data)
	b64Name := base64.StdEncoding.EncodeToString([]byte(filename))

	width := opts.Width
	if width == "" {
		width = "auto"
	}
	height := opts.Height
	if height == "" {
		height = "auto"
	}

	aspect := "1"
	if !opts.PreserveAspectRatio {
		aspect = "0"
	}

	// Standard iTerm2 OSC 1337 escape sequence:
	// \033]1337;File=name=<b64>;size=<len>;inline=1;width=<w>;height=<h>;preserveAspectRatio=1:<payload>\a
	seq := fmt.Sprintf(
		"\x1b]1337;File=name=%s;size=%d;inline=1;preserveAspectRatio=%s;width=%s;height=%s:%s\a",
		b64Name,
		len(data),
		aspect,
		width,
		height,
		b64Data,
	)

	// If inside tmux, wrap in DCS passthrough
	if opts.InTmux {
		escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
		return "\x1bPtmux;" + escaped + "\x1b\\"
	}

	return seq
}

// TerminalOptions configures inline image rendering across terminal graphics protocols.
type TerminalOptions struct {
	Protocol            mermaid.GraphicsProtocol // Target protocol (auto, iterm2, kitty, sixel, none)
	Width               string                   // e.g. "auto", "100%", "80", "400px", "80cell"
	Height              string                   // e.g. "auto", "20", "300px", "30cell"
	PreserveAspectRatio bool                     // default true
	InTmux              bool                     // wraps in tmux DCS passthrough
}

// EnsurePNG checks if image data is already in PNG format, and if not, decodes and converts it to PNG.
func EnsurePNG(data []byte) ([]byte, error) {
	if len(data) >= 8 && string(data[0:8]) == "\x89PNG\r\n\x1a\n" {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed decoding image for PNG conversion: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed encoding image to PNG: %w", err)
	}
	return buf.Bytes(), nil
}

// FormatKitty formats image data using the Kitty graphics protocol (APC \033_G).
// Non-PNG images are automatically converted to PNG before encoding.
func FormatKitty(data []byte, opts TerminalOptions) (string, error) {
	pngBytes, err := EnsurePNG(data)
	if err != nil {
		return "", err
	}

	widthCols := 0
	if opts.Width != "" && opts.Width != "auto" {
		wClean := strings.TrimSuffix(opts.Width, "cell")
		if n, err := strconv.Atoi(wClean); err == nil && n > 0 {
			widthCols = n
		}
	}

	heightRows := 0
	if opts.Height != "" && opts.Height != "auto" {
		hClean := strings.TrimSuffix(opts.Height, "cell")
		if n, err := strconv.Atoi(hClean); err == nil && n > 0 {
			heightRows = n
		}
	}

	return mermaid.FormatKittyImage(pngBytes, widthCols, heightRows, opts.InTmux), nil
}

// FormatSixel formats image data using the DEC Sixel bitmap graphics protocol (DCS \033Pq).
func FormatSixel(data []byte, opts TerminalOptions) (string, error) {
	return mermaid.FormatSixelImage(data, opts.InTmux)
}

// FormatTerminal formats an image according to the specified GraphicsProtocol.
func FormatTerminal(data []byte, filename string, proto mermaid.GraphicsProtocol, opts TerminalOptions) (string, error) {
	switch proto {
	case mermaid.ProtocolKitty:
		return FormatKitty(data, opts)
	case mermaid.ProtocolITerm2:
		itermOpts := ITerm2Options{
			Width:               opts.Width,
			Height:              opts.Height,
			PreserveAspectRatio: opts.PreserveAspectRatio,
			InTmux:              opts.InTmux,
		}
		return FormatITerm2(data, filename, itermOpts), nil
	case mermaid.ProtocolSixel:
		return FormatSixel(data, opts)
	default:
		return "", fmt.Errorf("unsupported or disabled graphics protocol: %v", proto)
	}
}

// formatFieldLines formats a prefixed field (e.g. "Source: ...") wrapping words to innerWidth,
// indenting continuation lines under the prefix.
func formatFieldLines(prefix string, val string, innerWidth int) []string {
	prefixWidth := uniseg.StringWidth(prefix)
	indent := strings.Repeat(" ", prefixWidth)
	words := strings.Fields(val)
	if len(words) == 0 {
		return []string{prefix}
	}

	var lines []string
	var cur strings.Builder
	cur.WriteString(prefix)
	curWidth := prefixWidth

	for _, word := range words {
		wLen := uniseg.StringWidth(word)
		space := 1
		if curWidth == prefixWidth {
			space = 0
		}

		if curWidth+space+wLen <= innerWidth {
			if space > 0 {
				cur.WriteByte(' ')
				curWidth++
			}
			cur.WriteString(word)
			curWidth += wLen
		} else {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(indent)
			curWidth = prefixWidth

			if wLen > innerWidth-prefixWidth {
				g := uniseg.NewGraphemes(word)
				for g.Next() {
					cluster := g.Str()
					cw := g.Width()
					if curWidth+cw > innerWidth {
						lines = append(lines, cur.String())
						cur.Reset()
						cur.WriteString(indent)
						curWidth = prefixWidth
					}
					cur.WriteString(cluster)
					curWidth += cw
				}
			} else {
				cur.WriteString(word)
				curWidth += wLen
			}
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// FormatFallback renders a clean, fully enclosed rounded Unicode card
// for terminals that don't support inline images or when an image fails to load.
func FormatFallback(altText, src string, errMsg string, maxWidth ...int) string {
	if altText == "" {
		altText = "Untitled Image"
	}

	termWidth := 80
	if len(maxWidth) > 0 && maxWidth[0] > 0 {
		termWidth = maxWidth[0]
	}
	if termWidth < 40 {
		termWidth = 40
	}

	var title string
	if errMsg != "" {
		title = fmt.Sprintf("╭─── ⚠️  [Image Error] %s ", altText)
	} else {
		title = fmt.Sprintf("╭─── 🖼️  [Image: %s] ", altText)
	}

	titleWidth := uniseg.StringWidth(title)

	// Determine card width
	w := 70
	if titleWidth+4 > w {
		w = titleWidth + 4
	}
	if w > termWidth {
		w = termWidth
	}
	if titleWidth+3 > w {
		excess := (titleWidth + 3) - w
		runes := []rune(altText)
		if len(runes) > excess+3 {
			altText = string(runes[:len(runes)-excess-3]) + "..."
			if errMsg != "" {
				title = fmt.Sprintf("╭─── ⚠️  [Image Error] %s ", altText)
			} else {
				title = fmt.Sprintf("╭─── 🖼️  [Image: %s] ", altText)
			}
			titleWidth = uniseg.StringWidth(title)
		}
	}

	headerDashes := w - titleWidth - 1
	if headerDashes < 2 {
		headerDashes = 2
		w = titleWidth + headerDashes + 1
	}
	topLine := title + strings.Repeat("─", headerDashes) + "╮"

	innerWidth := w - 4 // 2 chars for "│ " and 2 chars for " │"
	if innerWidth < 10 {
		innerWidth = 10
	}

	var contentLines []string
	contentLines = append(contentLines, formatFieldLines("Source: ", src, innerWidth)...)
	if errMsg != "" {
		contentLines = append(contentLines, formatFieldLines("Reason: ", errMsg, innerWidth)...)
	}

	var sb strings.Builder
	sb.WriteString(topLine)
	sb.WriteString("\n")

	for _, line := range contentLines {
		lw := uniseg.StringWidth(line)
		padding := innerWidth - lw
		if padding < 0 {
			padding = 0
		}
		sb.WriteString("│ ")
		sb.WriteString(line)
		sb.WriteString(strings.Repeat(" ", padding))
		sb.WriteString(" │\n")
	}

	footerDashes := w - 2
	if footerDashes < 4 {
		footerDashes = 4
	}
	bottomLine := "╰" + strings.Repeat("─", footerDashes) + "╯"
	sb.WriteString(bottomLine)

	return sb.String()
}
