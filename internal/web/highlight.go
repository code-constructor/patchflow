package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

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

func highlightDiffJSON(path, diff string) string {
	result := map[string]map[int]string{"old": {}, "new": {}}
	oldLine, newLine := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		if match := hunkHeader.FindStringSubmatch(line); match != nil {
			oldLine = parseNumber(match[1])
			newLine = parseNumber(match[2])
			continue
		}
		if oldLine == 0 || newLine == 0 || line == "" {
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

func highlightLine(path, source string) template.HTML {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	formatter := html.New(html.WithClasses(false), html.PreventSurroundingPre(true))
	var output bytes.Buffer
	if err := formatter.Format(&output, styles.Get("github"), iterator); err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	return template.HTML(output.String())
}

func parseNumber(value string) int {
	result := 0
	for _, digit := range value {
		result = result*10 + int(digit-'0')
	}
	return result
}
