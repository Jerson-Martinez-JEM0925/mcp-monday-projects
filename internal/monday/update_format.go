package monday

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	fenceStartPattern = regexp.MustCompile("^\\s*```([A-Za-z0-9_-]*)\\s*$")
	headingPattern    = regexp.MustCompile("^\\s*(#{1,6})\\s+(.+?)\\s*#*\\s*$")
	unorderedPattern  = regexp.MustCompile("^\\s*[-*+]\\s+(.+)$")
	orderedPattern    = regexp.MustCompile("^\\s*\\d+[.)]\\s+(.+)$")
	inlineCodePattern = regexp.MustCompile("`([^`\\n]+)`")
	linkPattern       = regexp.MustCompile(`\[([^]\n]+)\]\(([^)\s]+)(?:\s+\"([^\"]*)\")?\)`)
	boldPattern       = regexp.MustCompile(`\*\*([^*\n]+)\*\*|__([^_\n]+)__`)
	strikePattern     = regexp.MustCompile(`~~([^~\n]+)~~`)
	italicPattern     = regexp.MustCompile(`\*([^*\n]+)\*`)
	tableSeparator    = regexp.MustCompile(`^:?-{3,}:?$`)
)

type updateReplacement struct {
	token string
	value string
}

// FormatUpdateBody converts the Markdown-like input accepted by the other
// clients into the HTML subset that Monday update bodies render reliably.
// Raw HTML is escaped rather than passed through, so tags and placeholders
// remain visible instead of being removed by Monday sanitization.
func FormatUpdateBody(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")

	var out strings.Builder
	var paragraph []string
	var code []string
	var codeLanguage string
	inCode := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		appendBlock(&out, "<p>"+renderInline(strings.Join(paragraph, "\n"))+"</p>")
		paragraph = nil
	}
	flushCode := func() {
		if !inCode {
			return
		}
		class := ""
		if codeLanguage != "" {
			class = ` class="language-` + html.EscapeString(codeLanguage) + `"`
		}
		appendBlock(&out, "<pre><code"+class+">"+html.EscapeString(strings.Join(code, "\n"))+"</code></pre>")
		code = nil
		codeLanguage = ""
		inCode = false
	}

	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], "\r")

		if inCode {
			if strings.TrimSpace(line) == "```" {
				flushCode()
				i++
				continue
			}
			code = append(code, line)
			i++
			continue
		}

		if match := fenceStartPattern.FindStringSubmatch(line); match != nil {
			flushParagraph()
			inCode = true
			codeLanguage = match[1]
			i++
			continue
		}
		if strings.TrimSpace(line) == "" {
			flushParagraph()
			i++
			continue
		}
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			flushParagraph()
			level := len(match[1])
			appendBlock(&out, "<h"+string(rune('0'+level))+">"+renderInline(strings.TrimSpace(match[2]))+"</h"+string(rune('0'+level))+">")
			i++
			continue
		}
		if i+1 < len(lines) && strings.Contains(line, "|") && isTableSeparatorRow(lines[i+1]) {
			flushParagraph()
			block, next := renderTable(lines, i)
			appendBlock(&out, block)
			i = next
			continue
		}
		if isHorizontalRule(line) {
			flushParagraph()
			appendBlock(&out, "<hr>")
			i++
			continue
		}
		if unorderedPattern.MatchString(line) {
			flushParagraph()
			block, next := renderList(lines, i, false)
			appendBlock(&out, block)
			i = next
			continue
		}
		if orderedPattern.MatchString(line) {
			flushParagraph()
			block, next := renderList(lines, i, true)
			appendBlock(&out, block)
			i = next
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			flushParagraph()
			var quote []string
			for i < len(lines) {
				candidate := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(candidate, ">") {
					break
				}
				quote = append(quote, strings.TrimSpace(strings.TrimPrefix(candidate, ">")))
				i++
			}
			appendBlock(&out, "<blockquote>"+renderInline(strings.Join(quote, "\n"))+"</blockquote>")
			continue
		}

		paragraph = append(paragraph, line)
		i++
	}

	if inCode {
		flushCode()
	}
	flushParagraph()
	return strings.TrimSpace(out.String())
}

func appendBlock(out *strings.Builder, block string) {
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
	out.WriteString(block)
}

func isHorizontalRule(line string) bool {
	trimmed := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	return len(trimmed) >= 3 && (strings.Trim(trimmed, "-") == "" || strings.Trim(trimmed, "*") == "" || strings.Trim(trimmed, "_") == "")
}

func isTableSeparatorRow(line string) bool {
	cells := splitTableCells(line)
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		if !tableSeparator.MatchString(strings.TrimSpace(cell)) {
			return false
		}
	}
	return true
}

func renderTable(lines []string, start int) (string, int) {
	header := splitTableCells(lines[start])
	separator := splitTableCells(lines[start+1])
	alignments := make([]string, len(separator))
	for i, cell := range separator {
		cell = strings.TrimSpace(cell)
		switch {
		case strings.HasPrefix(cell, ":") && strings.HasSuffix(cell, ":"):
			alignments[i] = "center"
		case strings.HasPrefix(cell, ":"):
			alignments[i] = "left"
		case strings.HasSuffix(cell, ":"):
			alignments[i] = "right"
		}
	}

	var b strings.Builder
	b.WriteString("<table><thead><tr>")
	for i, cell := range header {
		b.WriteString(tableCell("th", cell, alignmentAt(alignments, i)))
	}
	b.WriteString("</tr></thead><tbody>")

	i := start + 2
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && strings.Contains(lines[i], "|") {
		b.WriteString("<tr>")
		for col, cell := range splitTableCells(lines[i]) {
			b.WriteString(tableCell("td", cell, alignmentAt(alignments, col)))
		}
		b.WriteString("</tr>")
		i++
	}
	b.WriteString("</tbody></table>")
	return b.String(), i
}

func tableCell(tag, value, alignment string) string {
	attr := ""
	if alignment != "" {
		attr = ` align="` + alignment + `"`
	}
	return "<" + tag + attr + ">" + renderInline(strings.TrimSpace(value)) + "</" + tag + ">"
}

func alignmentAt(alignments []string, index int) string {
	if index >= len(alignments) {
		return ""
	}
	return alignments[index]
}

func splitTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.ReplaceAll(strings.TrimSpace(parts[i]), `\|`, "|")
	}
	return parts
}

func renderList(lines []string, start int, ordered bool) (string, int) {
	pattern := unorderedPattern
	tag := "ul"
	if ordered {
		pattern = orderedPattern
		tag = "ol"
	}
	var b strings.Builder
	b.WriteString("<" + tag + ">")
	i := start
	for i < len(lines) {
		match := pattern.FindStringSubmatch(lines[i])
		if match == nil {
			break
		}
		b.WriteString("<li>" + renderInline(strings.TrimSpace(match[1])) + "</li>")
		i++
	}
	b.WriteString("</" + tag + ">")
	return b.String(), i
}

func renderInline(input string) string {
	replacements := make([]updateReplacement, 0, 4)
	sequence := 0
	protect := func(value string) string {
		token := "\x00MONDAY_REPLACEMENT_" + string(rune('A'+sequence)) + "\x00"
		sequence++
		replacements = append(replacements, updateReplacement{token: token, value: value})
		return token
	}

	input = inlineCodePattern.ReplaceAllStringFunc(input, func(match string) string {
		content := strings.Trim(match, "`")
		return protect("<code>" + html.EscapeString(content) + "</code>")
	})
	input = linkPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := linkPattern.FindStringSubmatch(match)
		if len(parts) < 3 || !safeLink(parts[2]) {
			return match
		}
		label := renderInline(parts[1])
		return protect(`<a href="` + html.EscapeString(parts[2]) + `">` + label + "</a>")
	})

	result := html.EscapeString(input)
	result = boldPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := boldPattern.FindStringSubmatch(match)
		if parts[1] != "" {
			return "<strong>" + parts[1] + "</strong>"
		}
		return "<strong>" + parts[2] + "</strong>"
	})
	result = strikePattern.ReplaceAllString(result, "<del>$1</del>")
	result = italicPattern.ReplaceAllString(result, "<em>$1</em>")
	result = strings.ReplaceAll(result, "\n", "<br>\n")
	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, html.EscapeString(replacement.token), replacement.value)
	}
	return result
}

func safeLink(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return parsed.Opaque != "" || parsed.Path != ""
	default:
		return false
	}
}
