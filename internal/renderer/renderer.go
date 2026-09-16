package renderer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	mermaid "github.com/smford/golang-mermaid"
	"github.com/smford/mdee/internal/config"
	"github.com/smford/mdee/internal/image"
	"github.com/smford/mdee/internal/table"
	"github.com/smford/mdee/internal/term"
	"github.com/smford/mdee/internal/theme"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Renderer converts Markdown AST into terminal-optimized output.
type Renderer struct {
	opts       config.Options
	theme      *theme.Theme
	termInfo   term.Info
	httpClient *http.Client
	mdParser   goldmark.Markdown
}

// New creates a configured terminal Markdown renderer.
func New(opts config.Options) *Renderer {
	th := theme.GetTheme(opts.Theme, opts.Plain)
	termInfo := term.Detect()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	return &Renderer{
		opts:       opts,
		theme:      th,
		termInfo:   termInfo,
		httpClient: &http.Client{Timeout: image.DefaultTimeout},
		mdParser:   md,
	}
}

// Render processes markdown source bytes and returns the styled terminal string.
func (r *Renderer) Render(ctx context.Context, source []byte) (string, error) {
	// Strip carriage returns for consistent multi-platform processing
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))

	// Check and handle YAML frontmatter if present
	var frontmatterCard string
	source, frontmatterCard = r.extractFrontmatter(source)

	reader := text.NewReader(source)
	doc := r.mdParser.Parser().Parse(reader)

	var sb strings.Builder
	if frontmatterCard != "" {
		sb.WriteString(frontmatterCard)
		sb.WriteString("\n\n")
	}

	state := &renderState{
		ctx:       ctx,
		source:    source,
		renderer:  r,
		listStack: make([]*listInfo, 0),
	}

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		rendered := state.renderBlock(child)
		if rendered != "" {
			sb.WriteString(rendered)
			sb.WriteString("\n\n")
		}
	}

	result := strings.TrimRight(sb.String(), "\n") + "\n"
	return result, nil
}

type listInfo struct {
	isOrdered bool
	index     int
}

type renderState struct {
	ctx        context.Context
	source     []byte
	renderer   *Renderer
	listStack  []*listInfo
	quoteDepth int
}

func (s *renderState) width() int {
	if s.renderer.opts.Width > 0 {
		return s.renderer.opts.Width
	}
	if s.renderer.termInfo.Width > 0 {
		return s.renderer.termInfo.Width
	}
	return 80
}

func (s *renderState) renderBlock(node gast.Node) string {
	switch n := node.(type) {
	case *gast.Heading:
		return s.renderHeading(n)
	case *gast.Paragraph:
		return s.renderParagraph(n)
	case *gast.Blockquote:
		return s.renderBlockquote(n)
	case *gast.List:
		return s.renderList(n)
	case *gast.ThematicBreak:
		return s.renderThematicBreak()
	case *gast.FencedCodeBlock:
		return s.renderFencedCodeBlock(n)
	case *gast.CodeBlock:
		return s.renderIndentedCodeBlock(n)
	case *extast.Table:
		return s.renderTable(n)
	case *gast.HTMLBlock:
		return s.renderHTMLBlock(n)
	default:
		// Fallback block rendering
		return s.renderInlines(node)
	}
}

func (s *renderState) renderHeading(h *gast.Heading) string {
	content := s.renderInlines(h)
	th := s.renderer.theme

	var prefix string
	var styledText string
	w := s.width()

	switch h.Level {
	case 1:
		prefix = "# "
		styledText = th.H1.Render(prefix + content)
		if !s.renderer.opts.Plain {
			underlineLen := min(w, table.VisualWidth(prefix+content))
			underline := th.HorizontalRule.Render(strings.Repeat("═", underlineLen))
			return styledText + "\n" + underline
		}
		return styledText
	case 2:
		prefix = "## "
		styledText = th.H2.Render(prefix + content)
		if !s.renderer.opts.Plain {
			underlineLen := min(w, table.VisualWidth(prefix+content))
			underline := th.HorizontalRule.Render(strings.Repeat("─", underlineLen))
			return styledText + "\n" + underline
		}
		return styledText
	case 3:
		return th.H3.Render("### " + content)
	case 4:
		return th.H4.Render("#### " + content)
	case 5:
		return th.H5.Render("##### " + content)
	case 6:
		fallthrough
	default:
		return th.H6.Render("###### " + content)
	}
}

func (s *renderState) renderParagraph(p *gast.Paragraph) string {
	// If the paragraph contains only an image, render the image block directly
	if p.ChildCount() == 1 && p.FirstChild().Kind() == gast.KindImage {
		imgNode := p.FirstChild().(*gast.Image)
		return s.renderImage(imgNode)
	}

	content := s.renderInlines(p)
	if s.renderer.opts.Plain {
		return content
	}

	return s.renderer.theme.Paragraph.Render(content)
}

func (s *renderState) renderBlockquote(b *gast.Blockquote) string {
	s.quoteDepth++
	defer func() { s.quoteDepth-- }()

	var sb strings.Builder
	for child := b.FirstChild(); child != nil; child = child.NextSibling() {
		rendered := s.renderBlock(child)
		if rendered != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(rendered)
		}
	}

	lines := strings.Split(sb.String(), "\n")
	var result strings.Builder
	border := s.renderer.theme.BlockquoteBorder.Render("│ ")
	if s.renderer.opts.Plain {
		border = "> "
	}

	for i, line := range lines {
		result.WriteString(border)
		result.WriteString(s.renderer.theme.Blockquote.Render(line))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (s *renderState) renderList(l *gast.List) string {
	s.listStack = append(s.listStack, &listInfo{
		isOrdered: l.IsOrdered(),
		index:     l.Start,
	})
	defer func() {
		s.listStack = s.listStack[:len(s.listStack)-1]
	}()

	var sb strings.Builder
	for child := l.FirstChild(); child != nil; child = child.NextSibling() {
		if li, ok := child.(*gast.ListItem); ok {
			rendered := s.renderListItem(li)
			if rendered != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(rendered)
			}
		}
	}

	return sb.String()
}

func (s *renderState) renderListItem(li *gast.ListItem) string {
	depth := len(s.listStack) - 1
	info := s.listStack[depth]
	indent := strings.Repeat("  ", depth)

	var bullet string
	if info.isOrdered {
		bullet = fmt.Sprintf("%d. ", info.index)
		info.index++
	} else {
		switch depth % 3 {
		case 0:
			bullet = "• "
		case 1:
			bullet = "◦ "
		default:
			bullet = "▪ "
		}
	}

	// Render children
	var sb strings.Builder
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == gast.KindList {
			sb.WriteString("\n")
			sb.WriteString(s.renderList(child.(*gast.List)))
		} else {
			childText := s.renderBlock(child)
			if sb.Len() > 0 && childText != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(childText)
		}
	}

	fullText := sb.String()
	lines := strings.Split(fullText, "\n")
	var result strings.Builder

	bulletStyled := s.renderer.theme.ListBullet.Render(bullet)
	bulletWidth := table.VisualWidth(bullet)
	pad := indent + strings.Repeat(" ", bulletWidth)

	for i, line := range lines {
		if i == 0 {
			result.WriteString(indent + bulletStyled + line)
		} else {
			result.WriteString(pad + line)
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (s *renderState) renderThematicBreak() string {
	w := s.width()
	if w > 80 {
		w = 80
	}
	rule := strings.Repeat("─", w)
	if s.renderer.opts.Plain {
		rule = strings.Repeat("-", w)
	}
	return s.renderer.theme.HorizontalRule.Render(rule)
}

func (s *renderState) renderFencedCodeBlock(cb *gast.FencedCodeBlock) string {
	var codeBuf bytes.Buffer
	lines := cb.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		codeBuf.Write(line.Value(s.source))
	}

	lang := string(cb.Language(s.source))
	trimmedLang := strings.TrimSpace(lang)
	if strings.EqualFold(trimmedLang, "mermaid") {
		return s.renderMermaid(codeBuf.String())
	}

	return s.highlightCode(codeBuf.String(), lang)
}

func (s *renderState) renderMermaid(code string) string {
	source := strings.TrimSpace(code)
	if source == "" {
		return ""
	}

	modeStr := strings.ToLower(strings.TrimSpace(s.renderer.opts.MermaidMode))
	if modeStr == "raw" || modeStr == "code" {
		return s.highlightCode(code, "mermaid")
	}

	// Determine render mode
	var targetMode mermaid.RenderMode
	switch modeStr {
	case "ascii":
		targetMode = mermaid.ModeASCII
	case "ansi", "unicode", "text":
		targetMode = mermaid.ModeUnicode
	case "image", "graphical":
		targetMode = mermaid.ModeImage
	case "auto", "":
		if s.renderer.opts.Plain {
			targetMode = mermaid.ModeASCII
		} else if s.renderer.opts.ImageMode == "never" {
			targetMode = mermaid.ModeUnicode
		} else if s.renderer.opts.ImageMode == "always" {
			targetMode = mermaid.ModeImage
		} else {
			targetMode = mermaid.ModeAuto
		}
	default:
		targetMode = mermaid.ModeAuto
	}

	cols := s.width()
	if cols <= 0 {
		cols = 80
	}

	forceTTY := s.renderer.termInfo.IsTTY || s.renderer.opts.ImageMode == "always" || modeStr == "image" || modeStr == "graphical"

	opts := []mermaid.Option{
		mermaid.WithMode(targetMode),
		mermaid.WithColumns(cols),
		mermaid.WithCache(true),
		mermaid.WithForceTTY(forceTTY),
	}

	if s.renderer.opts.ImageWidth != "" && s.renderer.opts.ImageWidth != "auto" {
		opts = append(opts, mermaid.WithWidth(s.renderer.opts.ImageWidth))
	}
	if s.renderer.opts.ImageHeight != "" && s.renderer.opts.ImageHeight != "auto" {
		opts = append(opts, mermaid.WithHeight(s.renderer.opts.ImageHeight))
	}
	if s.renderer.opts.MermaidScale > 0 {
		opts = append(opts, mermaid.WithScale(s.renderer.opts.MermaidScale))
	}

	// Theme resolution
	if s.renderer.opts.MermaidTheme != "" {
		opts = append(opts, mermaid.WithTheme(s.renderer.opts.MermaidTheme))
	} else if !s.renderer.opts.Plain {
		switch strings.ToLower(s.renderer.opts.Theme) {
		case "light", "solarized-light":
			opts = append(opts, mermaid.WithTheme("default"))
		default:
			opts = append(opts, mermaid.WithTheme("dark"))
		}
	}

	// Observability fallback hook
	opts = append(opts, mermaid.WithOnFallback(func(reason string, err error) {
		if s.renderer.opts.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Mermaid fallback triggered: %s (err: %v)\n", reason, err)
		}
	}))

	printer := mermaid.New(opts...)
	res, err := printer.Render(s.ctx, source)
	if err != nil {
		if s.renderer.opts.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Mermaid render failed: %v, falling back to code block\n", err)
		}
		return s.highlightCode(code, "mermaid")
	}

	if s.renderer.opts.Debug && res != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Mermaid rendered: mode=%s protocol=%s fallback=%v duration=%v\n",
			res.Mode, res.Protocol, res.FallbackOccurred, res.Duration)
	}

	if res == nil || res.Output == "" {
		return s.highlightCode(code, "mermaid")
	}

	if res.Mode == mermaid.ModeImage && len(res.ImageData) > 0 {
		enhanced, err := processMermaidImage(
			res.ImageData,
			res.Protocol,
			s.renderer.opts,
			cols,
			s.renderer.termInfo.IsTmux,
		)
		if err == nil && enhanced != "" {
			return strings.TrimRight(enhanced, "\n")
		} else if s.renderer.opts.Debug && err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Mermaid image enhancement failed: %v\n", err)
		}
	}

	return strings.TrimRight(res.Output, "\n")
}

func (s *renderState) renderIndentedCodeBlock(cb *gast.CodeBlock) string {
	var codeBuf bytes.Buffer
	lines := cb.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		codeBuf.Write(line.Value(s.source))
	}

	return s.highlightCode(codeBuf.String(), "")
}

func (s *renderState) highlightCode(code, lang string) string {
	code = strings.ReplaceAll(code, "\t", "    ")
	code = strings.TrimRight(code, "\n")

	if s.renderer.opts.Plain {
		return code
	}

	// Use chroma for syntax highlighting
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	chromaTheme := s.renderer.theme.ChromaTheme
	style := styles.Get(chromaTheme)
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.TTY256
	if s.renderer.termInfo.HasTrueColor {
		formatter = formatters.TTY16m
	}

	var buf bytes.Buffer
	iter, err := lexer.Tokenise(nil, code)
	if err == nil {
		_ = formatter.Format(&buf, style, iter)
	} else {
		buf.WriteString(code)
	}

	highlightedLines := strings.Split(buf.String(), "\n")

	// Frame code block with title header
	termWidth := s.width()
	if termWidth <= 0 {
		termWidth = 80
	}

	w := termWidth
	if w > 100 {
		w = 100
	}

	maxLineWidth := 0
	for _, l := range highlightedLines {
		lw := table.VisualWidth(l)
		if lw > maxLineWidth {
			maxLineWidth = lw
		}
	}

	basePrefixWidth := 2 // "│ "
	if s.renderer.opts.LineNumbers {
		basePrefixWidth = 8 // "│ %3s │ "
	}
	minNeeded := maxLineWidth + basePrefixWidth + 2 // 2 for " │"
	if minNeeded > w && minNeeded <= termWidth {
		w = minNeeded
	}

	displayLang := lang
	if displayLang == "" {
		displayLang = "code"
	}

	title := fmt.Sprintf("╭─── %s ", displayLang)
	titleWidth := table.VisualWidth(title)
	headerDashes := w - titleWidth - 1
	if headerDashes < 2 {
		headerDashes = 2
		w = titleWidth + headerDashes + 1
	}
	header := title + strings.Repeat("─", headerDashes) + "╮"

	var sb strings.Builder
	sb.WriteString(s.renderer.theme.CodeBlockBorder.Render(header))
	sb.WriteString("\n")

	for idx, line := range highlightedLines {
		var linePrefix string
		var prefixWidth int
		if s.renderer.opts.LineNumbers {
			numStr := strconv.Itoa(idx + 1)
			linePrefix = fmt.Sprintf("│ %3s │ ", numStr)
			prefixWidth = table.VisualWidth(linePrefix)
		} else {
			linePrefix = "│ "
			prefixWidth = 2
		}

		lineWidth := table.VisualWidth(line)
		padding := w - prefixWidth - lineWidth - 2
		if padding < 0 {
			padding = 0
		}

		sb.WriteString(s.renderer.theme.CodeBlockBorder.Render(linePrefix))
		sb.WriteString(line)
		sb.WriteString(strings.Repeat(" ", padding))
		sb.WriteString(s.renderer.theme.CodeBlockBorder.Render(" │"))
		if idx < len(highlightedLines)-1 {
			sb.WriteString("\n")
		}
	}

	footerDashes := w - 2
	if footerDashes < 4 {
		footerDashes = 4
	}
	footer := "\n" + s.renderer.theme.CodeBlockBorder.Render("╰"+strings.Repeat("─", footerDashes)+"╯")
	sb.WriteString(footer)

	return sb.String()
}

func (s *renderState) renderTable(t *extast.Table) string {
	var headers []string
	var alignments []table.Alignment
	var rows [][]string

	// Extract alignments from Table if available
	for _, a := range t.Alignments {
		alignments = append(alignments, convertAlignment(a))
	}

	for child := t.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *extast.TableHeader:
			for cellChild := n.FirstChild(); cellChild != nil; cellChild = cellChild.NextSibling() {
				if tc, ok := cellChild.(*extast.TableCell); ok {
					headers = append(headers, s.renderInlines(tc))
					if len(alignments) < len(headers) {
						alignments = append(alignments, convertAlignment(tc.Alignment))
					}
				}
			}
		case *extast.TableRow:
			var row []string
			for cellChild := n.FirstChild(); cellChild != nil; cellChild = cellChild.NextSibling() {
				if tc, ok := cellChild.(*extast.TableCell); ok {
					row = append(row, s.renderInlines(tc))
				}
			}
			rows = append(rows, row)
		}
	}

	tbl := table.New(headers)
	tbl.SetAlignments(alignments)
	for _, row := range rows {
		tbl.AddRow(row...)
	}

	tbl.Border = table.StyleNameToBorders(s.renderer.opts.TableStyle)
	tbl.MaxWidth = s.width()

	if !s.renderer.opts.Plain {
		tbl.BorderStyle = s.renderer.theme.TableBorder
		tbl.HeaderStyle = s.renderer.theme.TableHeader
		tbl.CellStyle = s.renderer.theme.TableCell
	} else {
		tbl.Border = table.BorderASCII
	}

	return tbl.Render()
}

func convertAlignment(a extast.Alignment) table.Alignment {
	switch a {
	case extast.AlignLeft:
		return table.AlignLeft
	case extast.AlignCenter:
		return table.AlignCenter
	case extast.AlignRight:
		return table.AlignRight
	default:
		return table.AlignNone
	}
}

func (s *renderState) renderHTMLBlock(h *gast.HTMLBlock) string {
	var buf bytes.Buffer
	lines := h.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		buf.Write(line.Value(s.source))
	}
	return buf.String()
}

func (s *renderState) renderImage(img *gast.Image) string {
	dest := string(img.Destination)
	altText := s.extractText(img)
	title := string(img.Title)

	shouldRender := false
	switch s.renderer.opts.ImageMode {
	case "always":
		shouldRender = true
	case "never":
		shouldRender = false
	case "auto":
		fallthrough
	default:
		shouldRender = s.renderer.termInfo.IsITerm2 || s.renderer.termInfo.HasOSC1337
	}

	if shouldRender {
		data, filename, err := image.Fetch(s.ctx, dest, s.renderer.opts.BasePath, s.renderer.httpClient)
		if err != nil {
			return image.FormatFallback(altText, dest, err.Error(), s.width())
		}

		opts := image.ITerm2Options{
			Width:               s.renderer.opts.ImageWidth,
			Height:              s.renderer.opts.ImageHeight,
			PreserveAspectRatio: true,
			InTmux:              s.renderer.termInfo.IsTmux,
		}

		seq := image.FormatITerm2(data, filename, opts)

		caption := altText
		if caption == "" {
			caption = title
		}

		if caption != "" {
			styledCaption := s.renderer.theme.Italic.Render("🖼  " + caption)
			return seq + "\n" + styledCaption
		}

		return seq
	}

	return image.FormatFallback(altText, dest, "", s.width())
}

func (s *renderState) renderInlines(node gast.Node) string {
	var sb strings.Builder

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		sb.WriteString(s.renderInlineNode(child))
	}

	return sb.String()
}

func (s *renderState) renderInlineNode(node gast.Node) string {
	switch n := node.(type) {
	case *gast.Text:
		textVal := string(n.Segment.Value(s.source))
		if n.HardLineBreak() {
			textVal += "\n"
		} else if n.SoftLineBreak() {
			textVal += " "
		}
		return textVal

	case *gast.String:
		return string(n.Value)

	case *gast.CodeSpan:
		content := s.extractText(n)
		if s.renderer.opts.Plain {
			return "`" + content + "`"
		}
		return s.renderer.theme.CodeSpan.Render(content)

	case *gast.Emphasis:
		content := s.renderInlines(n)
		if s.renderer.opts.Plain {
			if n.Level == 2 {
				return "**" + content + "**"
			}
			return "*" + content + "*"
		}
		if n.Level == 2 {
			return s.renderer.theme.Bold.Render(content)
		}
		return s.renderer.theme.Italic.Render(content)

	case *extast.Strikethrough:
		content := s.renderInlines(n)
		if s.renderer.opts.Plain {
			return "~~" + content + "~~"
		}
		return s.renderer.theme.Strikethrough.Render(content)

	case *gast.Link:
		url := string(n.Destination)
		linkText := s.renderInlines(n)
		if linkText == "" {
			linkText = url
		}

		if s.renderer.opts.Plain {
			if linkText == url {
				return url
			}
			return fmt.Sprintf("%s (%s)", linkText, url)
		}

		useOSC8 := s.renderer.opts.Hyperlinks && s.renderer.termInfo.HasOSC8 && !s.renderer.termInfo.IsTmux
		linkStyle := s.renderer.theme.Link
		if strings.Contains(linkText, "\x1b") {
			linkStyle = linkStyle.Underline(false)
		}
		if useOSC8 {
			styledText := linkStyle.Render(linkText)
			return term.FormatHyperlink(url, styledText, true)
		}

		styledLink := linkStyle.Render(linkText)
		if linkText != url {
			return fmt.Sprintf("%s (%s)", styledLink, s.renderer.theme.LinkURL.Render(url))
		}
		return styledLink

	case *gast.AutoLink:
		url := string(n.URL(s.source))
		label := string(n.Label(s.source))
		if s.renderer.opts.Plain {
			return url
		}
		useOSC8 := s.renderer.opts.Hyperlinks && s.renderer.termInfo.HasOSC8 && !s.renderer.termInfo.IsTmux
		if useOSC8 {
			styledText := s.renderer.theme.Link.Render(label)
			return term.FormatHyperlink(url, styledText, true)
		}
		return s.renderer.theme.Link.Render(label)

	case *extast.TaskCheckBox:
		if n.IsChecked {
			if s.renderer.opts.Plain {
				return "[x] "
			}
			return s.renderer.theme.TaskDone.Render("☑ ")
		}
		if s.renderer.opts.Plain {
			return "[ ] "
		}
		return s.renderer.theme.TaskTodo.Render("☐ ")

	case *gast.Image:
		return s.renderImage(n)

	case *gast.RawHTML:
		var buf bytes.Buffer
		lines := n.Segments
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			buf.Write(line.Value(s.source))
		}
		return buf.String()

	default:
		return s.renderInlines(node)
	}
}

func (s *renderState) extractText(node gast.Node) string {
	var sb strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch cn := child.(type) {
		case *gast.Text:
			sb.Write(cn.Segment.Value(s.source))
		case *gast.String:
			sb.Write(cn.Value)
		default:
			sb.WriteString(s.extractText(child))
		}
	}
	return sb.String()
}

func (r *Renderer) extractFrontmatter(source []byte) ([]byte, string) {
	str := string(source)
	if !strings.HasPrefix(str, "---\n") {
		return source, ""
	}

	rest := str[4:]
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		return source, ""
	}

	yamlContent := strings.TrimSpace(rest[:endIdx])
	remainingSource := []byte(rest[endIdx+5:])

	if r.opts.Plain {
		return remainingSource, ""
	}

	lines := strings.Split(yamlContent, "\n")
	maxLineWidth := 0
	for _, l := range lines {
		lw := table.VisualWidth(l)
		if lw > maxLineWidth {
			maxLineWidth = lw
		}
	}

	termWidth := r.opts.Width
	if termWidth <= 0 {
		termWidth = r.termInfo.Width
	}
	if termWidth <= 0 {
		termWidth = 80
	}

	w := 60
	minNeeded := maxLineWidth + 4
	if minNeeded > w {
		w = minNeeded
	}
	if w > termWidth {
		w = termWidth
	}
	if w > 100 {
		w = 100
	}

	title := "╭─── Metadata "
	titleWidth := table.VisualWidth(title)
	headerDashes := w - titleWidth - 1
	if headerDashes < 2 {
		headerDashes = 2
		w = titleWidth + headerDashes + 1
	}
	header := title + strings.Repeat("─", headerDashes) + "╮"

	var formatted strings.Builder
	formatted.WriteString(r.theme.CodeBlockBorder.Render(header))
	formatted.WriteString("\n")

	for _, l := range lines {
		lineWidth := table.VisualWidth(l)
		padding := w - 2 - lineWidth - 2
		if padding < 0 {
			padding = 0
		}
		formatted.WriteString(r.theme.CodeBlockBorder.Render("│ "))
		formatted.WriteString(r.theme.TableCell.Render(l))
		formatted.WriteString(strings.Repeat(" ", padding))
		formatted.WriteString(r.theme.CodeBlockBorder.Render(" │") + "\n")
	}

	footerDashes := w - 2
	if footerDashes < 4 {
		footerDashes = 4
	}
	footer := "╰" + strings.Repeat("─", footerDashes) + "╯"
	formatted.WriteString(r.theme.CodeBlockBorder.Render(footer))

	return remainingSource, formatted.String()
}
