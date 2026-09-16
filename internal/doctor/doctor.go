package doctor

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	mermaid "github.com/smford/golang-mermaid"
	"github.com/smford/mdee/internal/config"
	imagePkg "github.com/smford/mdee/internal/image"
	"github.com/smford/mdee/internal/table"
	"github.com/smford/mdee/internal/term"
)

// RunDiagnostics generates a full SRE terminal health and capability report.
func RunDiagnostics(configPath ...string) string {
	info := term.Detect()

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("╭──────────────────────────────────────────────────────────────╮") + "\n")
	sb.WriteString(titleStyle.Render("│          mdee - SRE Terminal Diagnostic & Capabilities       │") + "\n")
	sb.WriteString(titleStyle.Render("╰──────────────────────────────────────────────────────────────╯") + "\n\n")

	// 1. Environment & Runtime Table
	envTable := table.New([]string{"Diagnostic Check", "Value", "Status"})
	envTable.Border = table.BorderRounded
	envTable.MaxWidth = 80

	envTable.AddRow("OS / Architecture", fmt.Sprintf("%s / %s", runtime.GOOS, runtime.GOARCH), okStyle.Render("PASS"))
	envTable.AddRow("Go Runtime Version", runtime.Version(), okStyle.Render("PASS"))
	envTable.AddRow("Standard Output TTY", fmt.Sprintf("%t", info.IsTTY), formatBoolStatus(info.IsTTY, okStyle, warnStyle))
	envTable.AddRow("Terminal Dimensions", fmt.Sprintf("%d cols x %d rows", info.Width, info.Height), okStyle.Render("INFO"))

	// Terminal identification
	termProg := info.TermProgram
	if termProg == "" {
		termProg = "(unset)"
	}
	envTable.AddRow("TERM_PROGRAM", termProg, okStyle.Render("INFO"))
	envTable.AddRow("TERM", info.TermName, okStyle.Render("INFO"))

	// iTerm2 detection
	itermStatus := okStyle.Render("DETECTED (macOS iTerm2)")
	if !info.IsITerm2 {
		itermStatus = warnStyle.Render("NOT DETECTED (Fallback mode active)")
	}
	envTable.AddRow("iTerm2 Detection", fmt.Sprintf("%t", info.IsITerm2), itermStatus)

	// Tmux
	tmuxStatus := infoStyle.Render("No")
	if info.IsTmux {
		tmuxStatus = warnStyle.Render("Active (DCS Passthrough required)")
	}
	envTable.AddRow("Inside tmux Session", fmt.Sprintf("%t", info.IsTmux), tmuxStatus)

	// Capabilities
	envTable.AddRow("OSC 1337 (Inline Images)", fmt.Sprintf("%t", info.HasOSC1337), formatBoolStatus(info.HasOSC1337, okStyle, warnStyle))
	envTable.AddRow("OSC 8 (Terminal Links)", fmt.Sprintf("%t", info.HasOSC8), formatBoolStatus(info.HasOSC8, okStyle, warnStyle))
	envTable.AddRow("TrueColor (24-bit)", fmt.Sprintf("%t", info.HasTrueColor), formatBoolStatus(info.HasTrueColor, okStyle, warnStyle))

	// Mermaid Engine Capabilities
	mermaidProto := mermaid.DetectGraphicsProtocol(nil, true)
	var mermaidStatus string
	switch mermaidProto {
	case mermaid.ProtocolKitty:
		mermaidStatus = okStyle.Render("Kitty Graphics (APC)")
	case mermaid.ProtocolITerm2:
		mermaidStatus = okStyle.Render("iTerm2 Graphics (OSC 1337)")
	case mermaid.ProtocolSixel:
		mermaidStatus = okStyle.Render("DEC Sixel Bitmap (DCS)")
	default:
		mermaidStatus = infoStyle.Render("ANSI / Unicode Fallback")
	}
	envTable.AddRow("Mermaid Protocol", mermaidProto.String(), mermaidStatus)

	hasMmdc := false
	mmdcStatus := infoStyle.Render("Not Found (Remote / Text fallback)")
	if _, err := exec.LookPath("mmdc"); err == nil {
		hasMmdc = true
		mmdcStatus = okStyle.Render("INSTALLED (Local CLI)")
	}
	envTable.AddRow("Mermaid CLI (mmdc)", fmt.Sprintf("%t", hasMmdc), mmdcStatus)
	envTable.AddRow("Mermaid Text Engine", "mmaid-go", okStyle.Render("READY (Pure-Go)"))

	// Config file check
	cfgFile := config.DefaultConfigFile()
	if len(configPath) > 0 && configPath[0] != "" {
		cfgFile = configPath[0]
	}
	cfgDisplay := "~/.mdeerc"
	if cfgFile != config.DefaultConfigFile() {
		cfgDisplay = cfgFile
	}
	var cfgStatus string
	if cfgFile != "" {
		_, loaded, err := config.LoadConfigFile(cfgFile, false)
		if err != nil {
			cfgStatus = warnStyle.Render("SYNTAX ERROR")
		} else if loaded {
			cfgStatus = okStyle.Render("LOADED")
		} else {
			cfgStatus = infoStyle.Render("Not Found (Run 'mdee init')")
		}
	} else {
		cfgStatus = infoStyle.Render("Not Configured")
	}
	envTable.AddRow("Config File (~/.mdeerc)", cfgDisplay, cfgStatus)

	sb.WriteString(envTable.Render())
	sb.WriteString("\n\n")

	// 2. Interactive Protocol Verification Test
	sb.WriteString(titleStyle.Render("── Protocol Verification Test ──────────────────────────────────") + "\n\n")

	// Test link
	testLink := term.FormatHyperlink("https://github.com/smford/mdee", "Click here to test OSC 8 GitHub link", true)
	sb.WriteString(fmt.Sprintf("  • OSC 8 Hyperlink: %s\n", testLink))

	// Test inline image (mini 32x32 color swatch)
	testPNG := generateSwatchPNG()
	opts := imagePkg.ITerm2Options{
		Width:               "16",
		Height:              "auto",
		PreserveAspectRatio: true,
		InTmux:              info.IsTmux,
	}

	if info.HasOSC1337 {
		imgSeq := imagePkg.FormatITerm2(testPNG, "doctor_test.png", opts)
		sb.WriteString("  • OSC 1337 Inline Image Test (32x32 color gradient swatch):\n\n")
		sb.WriteString("    " + imgSeq + "\n\n")
		sb.WriteString("    (If you see a square gradient above, iTerm2 inline graphics are working!)\n\n")
	} else {
		sb.WriteString("  • OSC 1337 Inline Image Test:\n")
		sb.WriteString("    [Skipped: current terminal does not report iTerm2 OSC 1337 support]\n")
		sb.WriteString("    (Use --images=always to force transmission if using a compatible emulator)\n\n")
	}

	// Test Mermaid diagram rendering
	p := mermaid.New(
		mermaid.WithMode(mermaid.ModeAuto),
		mermaid.WithOffline(true),
		mermaid.WithColumns(50),
	)
	mermaidRes, err := p.Render(context.Background(), "flowchart LR\n    Terminal --> Mermaid")
	if err == nil && mermaidRes != nil {
		sb.WriteString(fmt.Sprintf("  • Mermaid Diagram Rendering Test (%s mode, %s protocol):\n\n",
			mermaidRes.Mode, mermaidRes.Protocol))
		lines := strings.Split(strings.TrimRight(mermaidRes.Output, "\n"), "\n")
		for _, line := range lines {
			sb.WriteString("    " + line + "\n")
		}
		sb.WriteString("\n")
	}

	// 3. SRE Recommendations
	if info.IsTmux {
		sb.WriteString(titleStyle.Render("── SRE Recommendations for tmux + iTerm2 ────────────────────────") + "\n\n")
		sb.WriteString("  1. To allow iTerm2 images inside tmux, ensure your ~/.tmux.conf has:\n")
		sb.WriteString(infoStyle.Render("     set -g allow-passthrough on") + "\n")
		sb.WriteString("  2. Reload tmux config with:\n")
		sb.WriteString(infoStyle.Render("     tmux source-file ~/.tmux.conf") + "\n\n")
	}

	return sb.String()
}

func formatBoolStatus(val bool, ok, warn lipgloss.Style) string {
	if val {
		return ok.Render("ENABLED")
	}
	return warn.Render("DISABLED")
}

func generateSwatchPNG() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r := uint8((x * 255) / size)
			g := uint8((y * 255) / size)
			b := uint8(220)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

type stringsBuilder struct {
	bytes.Buffer
}

func (b *stringsBuilder) WriteString(s string) {
	_, _ = b.Buffer.WriteString(s)
}

func (b *stringsBuilder) String() string {
	return b.Buffer.String()
}
