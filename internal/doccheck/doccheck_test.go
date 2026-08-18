package doccheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEveryGoFunctionHasDocumentation enforces named Doc Comments across production and test code.
func TestEveryGoFunctionHasDocumentation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var failures []string
	files := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		parsed, parseErr := parser.ParseFile(files, path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			position := files.Position(function.Pos())
			if function.Doc == nil || strings.TrimSpace(function.Doc.Text()) == "" {
				failures = append(failures, fmt.Sprintf("%s:%d: %s has no Doc Comment", path, position.Line, function.Name.Name))
				continue
			}
			if first := strings.Fields(function.Doc.Text()); len(first) == 0 || first[0] != function.Name.Name {
				failures = append(failures, fmt.Sprintf("%s:%d: Doc Comment for %s must start with its name", path, position.Line, function.Name.Name))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("Go documentation convention failed:\n%s", strings.Join(failures, "\n"))
	}
}
