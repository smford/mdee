package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	mermaid "github.com/smford/golang-mermaid"
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
	var (
		noPager          bool
		noHyperlinks     bool
		configFile       string
		initConfig       bool
		imageProtocolStr string
	)

	cmd := &cobra.Command{
		Use:     "mdee [flags] [file | URL ...]",
		Version: version,
		Short:   "High-fidelity Markdown terminal viewer for Kitty, Ghostty, WezTerm, iTerm2, and modern terminals",
		Long: `mdee is a production-grade terminal Markdown viewer written in Go.
Specifically engineered for modern terminals (Kitty, Ghostty, WezTerm, iTerm2, Foot, mlterm, mintty, Linux & macOS),
it delivers accurate inline graphics using Kitty (\033_G), iTerm2 OSC 1337, and DEC Sixel bitmap protocols,
graphical Mermaid diagram rendering with automated ANSI/ASCII fallback,
word-wrapped and auto-aligned tables with Unicode box borders, syntax-highlighted
code blocks, and OSC 8 clickable hyperlinks.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()

			// Fast path: generate default configuration file if --init-config requested
			if initConfig {
				target := configFile
				if target == "" {
					target = config.DefaultConfigFile()
				}
				if target == "" {
					return errors.New("unable to determine user home directory for ~/.mdeerc; specify --config <path>")
				}
				path, err := config.WriteDefaultConfigFile(target, false)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created default configuration file: %s\n", path)
				return nil
			}

			// Load configuration file (~/.mdeerc or custom --config)
			cfgPath := configFile
			explicitConfig := flags.Changed("config")
			if cfgPath == "" {
				cfgPath = config.DefaultConfigFile()
			}

			fileOpts, loaded, err := config.LoadConfigFile(cfgPath, explicitConfig)
			if err != nil {
				return err
			}

			if loaded {
				if !flags.Changed("width") {
					opts.Width = fileOpts.Width
				}
				if !flags.Changed("theme") {
					opts.Theme = fileOpts.Theme
				}
				if !flags.Changed("table-style") {
					opts.TableStyle = fileOpts.TableStyle
				}
				if !flags.Changed("images") {
					opts.ImageMode = fileOpts.ImageMode
				}
				if !flags.Changed("image-width") {
					opts.ImageWidth = fileOpts.ImageWidth
				}
				if !flags.Changed("image-height") {
					opts.ImageHeight = fileOpts.ImageHeight
				}
				if !flags.Changed("image-protocol") {
					opts.ImageProtocol = fileOpts.ImageProtocol
				}
				if !flags.Changed("mermaid") && !flags.Changed("mermaid-mode") {
					opts.MermaidMode = fileOpts.MermaidMode
				}
				if !flags.Changed("mermaid-theme") {
					opts.MermaidTheme = fileOpts.MermaidTheme
				}
				if !flags.Changed("mermaid-width") {
					opts.MermaidWidth = fileOpts.MermaidWidth
				}
				if !flags.Changed("mermaid-bg") && !flags.Changed("mermaid-background") {
					opts.MermaidBg = fileOpts.MermaidBg
				}
				if !flags.Changed("mermaid-scale") {
					opts.MermaidScale = fileOpts.MermaidScale
				}
				if !flags.Changed("line-numbers") {
					opts.LineNumbers = fileOpts.LineNumbers
				}
				if !flags.Changed("hyperlinks") && !flags.Changed("no-hyperlinks") {
					opts.Hyperlinks = fileOpts.Hyperlinks
				}
				if !flags.Changed("pager") && !flags.Changed("no-pager") {
					opts.Pager = fileOpts.Pager
				}
				if !flags.Changed("plain") {
					opts.Plain = fileOpts.Plain
				}
				if !flags.Changed("debug") {
					opts.Debug = fileOpts.Debug
				}
				opts.ConfigFile = fileOpts.ConfigFile
			}

			if flags.Changed("image-protocol") {
				switch strings.ToLower(strings.TrimSpace(imageProtocolStr)) {
				case "auto", "":
					opts.ImageProtocol = mermaid.ProtocolAuto
				case "kitty":
					opts.ImageProtocol = mermaid.ProtocolKitty
				case "iterm2", "osc1337", "iterm":
					opts.ImageProtocol = mermaid.ProtocolITerm2
				case "sixel":
					opts.ImageProtocol = mermaid.ProtocolSixel
				case "none", "off", "disabled":
					opts.ImageProtocol = mermaid.ProtocolNone
				default:
					return fmt.Errorf("invalid image protocol %q (supported: auto, kitty, iterm2, sixel, none)", imageProtocolStr)
				}
			}

			if flags.Changed("no-pager") {
				opts.Pager = false
			}
			if flags.Changed("no-hyperlinks") {
				opts.Hyperlinks = false
			}

			if opts.Debug && opts.ConfigFile != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG] Configuration loaded from: %s\n", opts.ConfigFile)
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
	flags.StringVarP(&configFile, "config", "c", "", "Path to configuration file (default: ~/.mdeerc)")
	flags.IntVarP(&opts.Width, "width", "w", 0, "Explicit terminal width in columns (0 = auto-detect)")
	flags.StringVarP(&opts.Theme, "theme", "t", "dark", "Theme: dark, light, dracula, monokai, solarized-dark, solarized-light, plain")
	flags.StringVarP(&opts.TableStyle, "table-style", "s", "rounded", "Table border style: rounded, box, double, ascii, markdown, minimal")
	flags.StringVarP(&opts.ImageMode, "images", "i", "auto", "Inline image mode: auto, always, never")
	flags.StringVar(&opts.ImageWidth, "image-width", "auto", "Image width constraint: auto, 100%, 80, 400px")
	flags.StringVar(&opts.ImageHeight, "image-height", "auto", "Image height constraint: auto, 20, 300px")
	flags.StringVar(&imageProtocolStr, "image-protocol", "auto", "Image graphics protocol: auto, kitty, iterm2, sixel, none")
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
	flags.BoolVar(&initConfig, "init-config", false, "Generate default ~/.mdeerc configuration file and exit")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newInitCmd())

	return cmd
}

func newInitCmd() *cobra.Command {
	var (
		force       bool
		output      string
		printStdout bool
	)

	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"init-config", "config-init"},
		Short:   "Generate a default ~/.mdeerc configuration file",
		Long: `Generate a default configuration file with recommended settings and detailed comments.
By default, writes to ~/.mdeerc. If ~/.mdeerc already exists, use --force to overwrite it.
Use --stdout to print the configuration template to the terminal instead of writing to disk.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printStdout {
				_, err := fmt.Fprint(cmd.OutOrStdout(), config.DefaultConfigTemplate())
				return err
			}

			target := output
			if target == "" {
				target = config.DefaultConfigFile()
			}
			if target == "" {
				return errors.New("unable to determine user home directory for ~/.mdeerc; specify --output <path>")
			}

			path, err := config.WriteDefaultConfigFile(target, force)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created default configuration file: %s\n", path)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Overwrite existing configuration file if it exists")
	flags.StringVarP(&output, "output", "o", "", "Destination file path (default: ~/.mdeerc)")
	flags.BoolVar(&printStdout, "stdout", false, "Print default configuration to stdout instead of writing to file")

	return cmd
}

func newDoctorCmd() *cobra.Command {
	var configFile string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose terminal environment, emulator detection, graphics protocol support, and configuration",
		Run: func(cmd *cobra.Command, args []string) {
			report := doctor.RunDiagnostics(configFile)
			fmt.Print(report)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file (default: ~/.mdeerc)")
	return cmd
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
		base := target
		if parsed, err := url.Parse(target); err == nil {
			parsed.Path = filepath.Dir(parsed.Path)
			if !strings.HasSuffix(parsed.Path, "/") {
				parsed.Path += "/"
			}
			base = parsed.String()
		}
		return data, base, nil
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
