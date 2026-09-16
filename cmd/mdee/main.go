package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/smford/mdee/internal/config"
	"github.com/smford/mdee/internal/doctor"
	"github.com/smford/mdee/internal/renderer"
	"github.com/smford/mdee/internal/term"
	"github.com/spf13/cobra"
)

var (
	// Build metadata populated via ldflags at build time
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// SRE Protection: Handle broken pipes gracefully when piping to head/grep
	handleBrokenPipe()

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleBrokenPipe() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

func newRootCmd() *cobra.Command {
	opts := config.DefaultOptions()
	var noPager bool
	var noHyperlinks bool

	cmd := &cobra.Command{
		Use:     "mdee [flags] [file | URL ...]",
		Version: version,
		Short:   "High-fidelity Markdown terminal viewer optimized for macOS iTerm2",
		Long: `mdee is a production-grade terminal Markdown viewer written in Go.
Specifically engineered for modern terminals (macOS iTerm2, Kitty, Ghostty, WezTerm),
it delivers accurate inline graphics using iTerm2 OSC 1337 and Kitty protocols,
graphical Mermaid diagram rendering with automated ANSI/ASCII fallback,
word-wrapped and auto-aligned tables with Unicode box borders, syntax-highlighted
code blocks, and OSC 8 clickable hyperlinks.`,
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if noPager {
				opts.Pager = false
			}
			if noHyperlinks {
				opts.Hyperlinks = false
			}

			switch strings.ToLower(strings.TrimSpace(opts.MermaidMode)) {
			case "auto", "image", "graphical", "ansi", "unicode", "text", "ascii", "raw", "code", "":
				// valid
			default:
				return fmt.Errorf("invalid mermaid mode %q (supported: auto, image, ansi, unicode, ascii, raw)", opts.MermaidMode)
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			return runViewer(ctx, opts, args)
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&opts.Width, "width", "w", 0, "Explicit terminal width in columns (0 = auto-detect)")
	flags.StringVarP(&opts.Theme, "theme", "t", "dark", "Theme: dark, light, dracula, monokai, solarized-dark, solarized-light, plain")
	flags.StringVarP(&opts.TableStyle, "table-style", "s", "rounded", "Table border style: rounded, box, double, ascii, markdown, minimal")
	flags.StringVarP(&opts.ImageMode, "images", "i", "auto", "Inline image mode: auto, always, never")
	flags.StringVar(&opts.ImageWidth, "image-width", "auto", "Image width constraint: auto, 100%, 80, 400px")
	flags.StringVar(&opts.ImageHeight, "image-height", "auto", "Image height constraint: auto, 20, 300px")
	flags.StringVarP(&opts.MermaidMode, "mermaid", "m", "auto", "Mermaid render mode: auto, image, ansi, unicode, ascii, raw")
	flags.StringVar(&opts.MermaidMode, "mermaid-mode", "auto", "Alias for --mermaid")
	flags.StringVar(&opts.MermaidTheme, "mermaid-theme", "", "Mermaid diagram theme: dark, default, slate, blueprint, neon, neutral, forest")
	flags.StringVar(&opts.MermaidWidth, "mermaid-width", "auto", "Mermaid image display width: auto, 100%, 80, 800px")
	flags.StringVar(&opts.MermaidBg, "mermaid-bg", "auto", "Mermaid image background: auto, dark, light, transparent, or hex #RRGGBB")
	flags.StringVar(&opts.MermaidBg, "mermaid-background", "auto", "Alias for --mermaid-bg")
	flags.Float64Var(&opts.MermaidScale, "mermaid-scale", 2.0, "Mermaid image scale factor (1.0 - 4.0)")
	flags.BoolVarP(&opts.LineNumbers, "line-numbers", "n", false, "Show line numbers in code blocks")
	flags.BoolVar(&opts.Hyperlinks, "hyperlinks", true, "Enable OSC 8 terminal hyperlinks")
	flags.BoolVar(&noHyperlinks, "no-hyperlinks", false, "Disable OSC 8 terminal hyperlinks")
	flags.BoolVar(&opts.Pager, "pager", false, "Enable pager for output longer than terminal screen")
	flags.BoolVar(&noPager, "no-pager", false, "Disable pager output")
	flags.BoolVar(&opts.Plain, "plain", false, "Output plain text without ANSI escape sequences or colors")
	flags.BoolVar(&opts.Debug, "debug", false, "Print debug diagnostic logs to stderr")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose terminal environment, iTerm2 detection, and protocol support",
		Run: func(cmd *cobra.Command, args []string) {
			report := doctor.RunDiagnostics()
			fmt.Print(report)
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print application version and build metadata",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("mdee version %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}

func runViewer(ctx context.Context, opts config.Options, args []string) error {
	termInfo := term.Detect()

	// If no args or "-" provided, read from os.Stdin
	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		opts.BasePath = "."
		return renderAndOutput(ctx, opts, termInfo, data)
	}

	// Process multiple files or URLs
	var combinedOutput strings.Builder
	for idx, target := range args {
		data, basePath, err := readSource(ctx, target)
		if err != nil {
			return fmt.Errorf("failed reading target %q: %w", target, err)
		}

		targetOpts := opts
		targetOpts.BasePath = basePath

		r := renderer.New(targetOpts)
		rendered, err := r.Render(ctx, data)
		if err != nil {
			return fmt.Errorf("failed rendering %q: %w", target, err)
		}

		if idx > 0 {
			combinedOutput.WriteString("\n\n")
		}
		combinedOutput.WriteString(rendered)
	}

	return outputContent(opts, termInfo, combinedOutput.String())
}

func readSource(ctx context.Context, target string) ([]byte, string, error) {
	// Remote URL
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("User-Agent", "md-cli/1.0")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("HTTP error %s", resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}
		return data, ".", nil
	}

	// Local file
	cleanPath := filepath.Clean(target)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, "", err
	}
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return data, filepath.Dir(cleanPath), nil
	}
	return data, filepath.Dir(absPath), nil
}

func renderAndOutput(ctx context.Context, opts config.Options, termInfo term.Info, data []byte) error {
	r := renderer.New(opts)
	rendered, err := r.Render(ctx, data)
	if err != nil {
		return err
	}
	return outputContent(opts, termInfo, rendered)
}

func outputContent(opts config.Options, termInfo term.Info, content string) error {
	// If output is directed to a TTY and pager is enabled and content exceeds screen height
	lineCount := strings.Count(content, "\n")
	if termInfo.IsTTY && opts.Pager && lineCount > termInfo.Height && !opts.Plain {
		err := term.RunPager(content)
		if err != nil && !errors.Is(err, syscall.EPIPE) {
			// If pager fails, fall back to stdout
			_, _ = fmt.Print(content)
		}
		return nil
	}

	_, err := fmt.Print(content)
	if err != nil && errors.Is(err, syscall.EPIPE) {
		return nil
	}
	return err
}
