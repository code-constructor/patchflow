package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// highlightCode turns a source excerpt into independently numbered, highlighted lines.
func highlightCode(path, source string, startLine int) []CodeLine {
	lines := strings.SplitAfter(source, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	result := make([]CodeLine, 0, len(lines))
	for index, line := range lines {
		result = append(result, CodeLine{Number: startLine + index, HTML: highlightLine(path, strings.TrimSuffix(line, "\n"))})
	}
	return result
}

// highlightDiffJSON builds the old/new line maps consumed by the diff controller.
func highlightDiffJSON(path, diff string) string {
	result := map[string]map[int]string{"old": {}, "new": {}}
	oldLine, newLine := 0, 0
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		if match := hunkHeader.FindStringSubmatch(line); match != nil {
			oldLine = parseNumber(match[1])
			newLine = parseNumber(match[2])
			inHunk = true
			continue
		}
		if !inHunk || line == "" {
			continue
		}
		content := line[1:]
		highlighted := string(highlightLine(path, content))
		switch line[0] {
		case ' ':
			result["old"][oldLine] = highlighted
			result["new"][newLine] = highlighted
			oldLine++
			newLine++
		case '-':
			result["old"][oldLine] = highlighted
			oldLine++
		case '+':
			result["new"][newLine] = highlighted
			newLine++
		}
	}
	encoded, _ := json.Marshal(result)
	return string(encoded)
}

// highlightLine lexes one source line and returns escaped HTML for GitHub-like light surfaces.
func highlightLine(path, source string) template.HTML {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	iterator = withoutErrorTokens(iterator)
	formatter := html.New(html.WithClasses(false), html.PreventSurroundingPre(true))
	var output bytes.Buffer
	if err := formatter.Format(&output, styles.Get("github"), iterator); err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	return template.HTML(output.String())
}

// withoutErrorTokens downgrades lexer error tokens to plain text because a
// single diff line lacks the surrounding context a stateful grammar expects.
func withoutErrorTokens(iterator chroma.Iterator) chroma.Iterator {
	tokens := iterator.Tokens()
	for index := range tokens {
		if tokens[index].Type == chroma.Error {
			tokens[index].Type = chroma.Text
		}
	}
	return chroma.Literator(tokens...)
}

// parseNumber converts an ASCII decimal line number without exposing parse errors.
func parseNumber(value string) int {
	result := 0
	for _, digit := range value {
		result = result*10 + int(digit-'0')
	}
	return result
}
