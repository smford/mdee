package renderer

import (
	"io"
	"strings"

	gast "github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// HTMLImage represents parsed attributes from an <img> tag.
type HTMLImage struct {
	Src    string
	Alt    string
	Title  string
	Width  string
	Height string
}

func parseHTMLImageAttrs(attrs []html.Attribute) HTMLImage {
	var img HTMLImage
	for _, attr := range attrs {
		key := strings.ToLower(attr.Key)
		val := strings.TrimSpace(attr.Val)
		switch key {
		case "src":
			img.Src = val
		case "alt":
			img.Alt = val
		case "title":
			img.Title = val
		case "width":
			img.Width = val
		case "height":
			img.Height = val
		case "style":
			if img.Width == "" {
				img.Width = extractCSSDimension(val, "width")
			}
			if img.Height == "" {
				img.Height = extractCSSDimension(val, "height")
			}
		}
	}
	return img
}

func extractCSSDimension(style, property string) string {
	for _, rule := range strings.Split(style, ";") {
		rule = strings.TrimSpace(rule)
		if idx := strings.Index(rule, ":"); idx != -1 {
			prop := strings.TrimSpace(strings.ToLower(rule[:idx]))
			if prop == property {
				return strings.TrimSpace(rule[idx+1:])
			}
		}
	}
	return ""
}

type htmlTokenItem struct {
	tt      html.TokenType
	tok     html.Token
	raw     string
	isImg   bool
	imgInfo HTMLImage
}

func parseHTMLTokens(rawHTML string) ([]htmlTokenItem, bool) {
	z := html.NewTokenizer(strings.NewReader(rawHTML))
	var items []htmlTokenItem
	hasImg := false
	inPre := false

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				break
			}
			break
		}
		tok := z.Token()
		raw := string(z.Raw())

		// Track <pre> and <code> to prevent processing code examples as real images
		if (tt == html.StartTagToken || tt == html.SelfClosingTagToken) && (tok.Data == "pre" || tok.Data == "code") {
			inPre = true
		}
		if tt == html.EndTagToken && (tok.Data == "pre" || tok.Data == "code") {
			inPre = false
		}

		item := htmlTokenItem{
			tt:  tt,
			tok: tok,
			raw: raw,
		}

		if !inPre && (tt == html.StartTagToken || tt == html.SelfClosingTagToken) && strings.EqualFold(tok.Data, "img") {
			item.isImg = true
			item.imgInfo = parseHTMLImageAttrs(tok.Attr)
			if item.imgInfo.Src != "" {
				hasImg = true
			}
		}

		items = append(items, item)
	}

	return items, hasImg
}

func isContainerTag(tag string) bool {
	switch strings.ToLower(tag) {
	case "p", "div", "center", "a", "picture", "figure", "figcaption",
		"span", "section", "article", "header", "footer", "b", "strong", "i", "em", "br":
		return true
	default:
		return false
	}
}

// renderHTML parses HTML containing <img> tags, strips pure container wrappers,
// and renders the image(s) using the terminal graphics protocol or fallback.
func (s *renderState) renderHTML(rawHTML string, isBlock bool) string {
	if !strings.Contains(strings.ToLower(rawHTML), "<img") {
		return rawHTML
	}

	items, hasImg := parseHTMLTokens(rawHTML)
	if !hasImg {
		return rawHTML
	}

	// Check if the HTML snippet is a pure image container (images + container tags + whitespace)
	isPureImageContainer := true
	var figcaptionText strings.Builder
	inFigcaption := false

	for _, item := range items {
		if item.isImg || item.tt == html.CommentToken {
			continue
		}
		if item.tt == html.StartTagToken && strings.EqualFold(item.tok.Data, "figcaption") {
			inFigcaption = true
			continue
		}
		if item.tt == html.EndTagToken && strings.EqualFold(item.tok.Data, "figcaption") {
			inFigcaption = false
			continue
		}
		if inFigcaption && item.tt == html.TextToken {
			figcaptionText.WriteString(item.tok.Data)
			continue
		}
		if item.tt == html.TextToken {
			if strings.TrimSpace(item.tok.Data) != "" {
				isPureImageContainer = false
				break
			}
			continue
		}
		if (item.tt == html.StartTagToken || item.tt == html.EndTagToken || item.tt == html.SelfClosingTagToken) && isContainerTag(item.tok.Data) {
			continue
		}
		// Any other tag means it's a more complex HTML construct
		isPureImageContainer = false
		break
	}

	if isPureImageContainer {
		// Pure image container: find wrapping <a> href, apply figcaption, and render images
		var currentHref string
		var renderedImages []string

		for _, item := range items {
			if item.tt == html.StartTagToken && strings.EqualFold(item.tok.Data, "a") {
				for _, attr := range item.tok.Attr {
					if strings.EqualFold(attr.Key, "href") {
						currentHref = attr.Val
					}
				}
			} else if item.tt == html.EndTagToken && strings.EqualFold(item.tok.Data, "a") {
				currentHref = ""
			} else if item.isImg {
				alt := item.imgInfo.Alt
				if alt == "" && figcaptionText.Len() > 0 {
					alt = strings.TrimSpace(figcaptionText.String())
				}
				rendered := s.renderImageCommon(
					item.imgInfo.Src,
					alt,
					item.imgInfo.Title,
					item.imgInfo.Width,
					item.imgInfo.Height,
					currentHref,
				)
				if rendered != "" {
					renderedImages = append(renderedImages, rendered)
				}
			}
		}

		if len(renderedImages) == 0 {
			return ""
		}
		if isBlock {
			return strings.Join(renderedImages, "\n\n")
		}
		return strings.Join(renderedImages, " ")
	}

	// Mixed HTML content: iterate tokens and format in place
	var sb strings.Builder
	var currentHref string
	isBold := false
	isItalic := false

	for _, item := range items {
		if item.isImg {
			rendered := s.renderImageCommon(
				item.imgInfo.Src,
				item.imgInfo.Alt,
				item.imgInfo.Title,
				item.imgInfo.Width,
				item.imgInfo.Height,
				currentHref,
			)
			if rendered != "" {
				sb.WriteString(rendered)
			}
			continue
		}

		switch item.tt {
		case html.TextToken:
			text := item.tok.Data
			if currentHref != "" && s.renderer.opts.Hyperlinks && s.renderer.termInfo.HasOSC8 && !s.renderer.termInfo.IsTmux {
				styled := s.renderer.theme.Link.Render(text)
				sb.WriteString(styled)
			} else if isBold && !s.renderer.opts.Plain {
				sb.WriteString(s.renderer.theme.Bold.Render(text))
			} else if isItalic && !s.renderer.opts.Plain {
				sb.WriteString(s.renderer.theme.Italic.Render(text))
			} else {
				sb.WriteString(text)
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			tag := strings.ToLower(item.tok.Data)
			switch tag {
			case "a":
				for _, attr := range item.tok.Attr {
					if strings.EqualFold(attr.Key, "href") {
						currentHref = attr.Val
					}
				}
			case "b", "strong":
				isBold = true
			case "i", "em":
				isItalic = true
			case "br":
				sb.WriteString("\n")
			case "hr":
				sb.WriteString("\n" + s.renderThematicBreak() + "\n")
			case "p", "div":
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}

		case html.EndTagToken:
			tag := strings.ToLower(item.tok.Data)
			switch tag {
			case "a":
				currentHref = ""
			case "b", "strong":
				isBold = false
			case "i", "em":
				isItalic = false
			case "p", "div":
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// extractAdjacentRawHTML examines whether node or its adjacent RawHTML siblings
// contain an HTML <img> tag. If so, it merges the consecutive RawHTML nodes into
// a single string and returns it along with the next unmerged sibling.
func (s *renderState) extractAdjacentRawHTML(node gast.Node) (string, gast.Node) {
	if _, ok := node.(*gast.RawHTML); !ok {
		return "", nil
	}

	// Fast check: inspect this node and immediately adjacent siblings
	var buf strings.Builder
	hasImg := false
	curr := node

	for curr != nil {
		if r, isRaw := curr.(*gast.RawHTML); isRaw {
			for i := 0; i < r.Segments.Len(); i++ {
				seg := r.Segments.At(i)
				val := string(seg.Value(s.source))
				buf.WriteString(val)
				if strings.Contains(strings.ToLower(val), "<img") {
					hasImg = true
				}
			}
			curr = curr.NextSibling()
			continue
		}

		// Also permit pure whitespace text nodes between HTML tags
		if t, isText := curr.(*gast.Text); isText {
			val := string(t.Segment.Value(s.source))
			if strings.TrimSpace(val) == "" {
				buf.WriteString(val)
				curr = curr.NextSibling()
				continue
			}
		}

		break
	}

	if !hasImg {
		return "", nil
	}

	return buf.String(), curr
}
