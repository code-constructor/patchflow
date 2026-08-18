package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/traqx-ai/patchflow/schema"
	yaml "go.yaml.in/yaml/v3"
)

type Review struct {
	SchemaVersion int        `json:"schema_version"`
	ID            string     `json:"id"`
	Repository    Repository `json:"repository"`
	Source        Source     `json:"source"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
	Status        string     `json:"status"`
	Change        Change     `json:"change"`
	OverviewPath  string     `json:"overview_path"`
	Steps         []Step     `json:"steps"`
}

type Repository struct {
	Name string `json:"name"`
}

type Source struct {
	BaseRef   string `json:"base_ref"`
	TargetRef string `json:"target_ref"`
	BaseSHA   string `json:"base_sha"`
	TargetSHA string `json:"target_sha"`
}

type Change struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Files   []ChangedFile `json:"files"`
}

type ChangedFile struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	PreviousPath string `json:"previous_path,omitempty"`
}

type Step struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Priority  string   `json:"priority"`
	Rationale string   `json:"rationale"`
	Files     []string `json:"files"`
	Blocks    []Block  `json:"blocks"`
}

type Block struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Body      string `json:"body,omitempty"`
	Path      string `json:"path,omitempty"`
	View      string `json:"view,omitempty"`
	Source    string `json:"source,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Focus     *Focus `json:"focus,omitempty"`
}

type Focus struct {
	Side      string `json:"side"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line,omitempty"`
}

type ValidationErrors struct {
	Errors []string `json:"errors"`
}

func (e *ValidationErrors) Error() string {
	return strings.Join(e.Errors, "\n")
}

type Validator struct {
	schema *jsonschema.Schema
}

func NewValidator() (*Validator, error) {
	var document any
	if err := json.Unmarshal(schema.ReviewV2, &document); err != nil {
		return nil, fmt.Errorf("decode embedded review schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const schemaURL = "https://patchflow.dev/schema/patchflow-review-v2.schema.json"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		return nil, fmt.Errorf("load embedded review schema: %w", err)
	}
	compiled, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile embedded review schema: %w", err)
	}
	return &Validator{schema: compiled}, nil
}

func (v *Validator) Parse(source []byte) (*Review, error) {
	var yamlDocument any
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	if err := decoder.Decode(&yamlDocument); err != nil {
		return nil, &ValidationErrors{Errors: []string{fmt.Sprintf("review.yaml is not valid YAML: %v", err)}}
	}

	jsonDocument, err := json.Marshal(yamlDocument)
	if err != nil {
		return nil, &ValidationErrors{Errors: []string{fmt.Sprintf("review.yaml cannot be represented as JSON: %v", err)}}
	}
	var document any
	if err := json.Unmarshal(jsonDocument, &document); err != nil {
		return nil, err
	}
	if err := v.schema.Validate(document); err != nil {
		return nil, &ValidationErrors{Errors: []string{err.Error()}}
	}

	var review Review
	if err := json.Unmarshal(jsonDocument, &review); err != nil {
		return nil, fmt.Errorf("decode validated review: %w", err)
	}
	if semanticErrors := validateSemantics(&review); len(semanticErrors) > 0 {
		return nil, &ValidationErrors{Errors: semanticErrors}
	}
	return &review, nil
}

func IsValidationError(err error) bool {
	var validationErrors *ValidationErrors
	return errors.As(err, &validationErrors)
}

func validateSemantics(review *Review) []string {
	var found []string
	changedPaths := make(map[string]bool, len(review.Change.Files))
	for index, file := range review.Change.Files {
		label := fmt.Sprintf("change.files[%d].path", index)
		found = append(found, validatePath(file.Path, label, false)...)
		if changedPaths[file.Path] {
			found = append(found, "change.files paths must be unique")
		}
		changedPaths[file.Path] = true
		if file.PreviousPath != "" {
			found = append(found, validatePath(file.PreviousPath, fmt.Sprintf("change.files[%d].previous_path", index), false)...)
		}
	}

	stepIDs := map[string]bool{}
	blockIDs := map[string]bool{}
	plannedPaths := map[string]bool{}
	for stepIndex, step := range review.Steps {
		if stepIDs[step.ID] {
			found = append(found, "step IDs must be unique")
		}
		stepIDs[step.ID] = true
		for fileIndex, filePath := range step.Files {
			label := fmt.Sprintf("steps[%d].files[%d]", stepIndex, fileIndex)
			found = append(found, validatePath(filePath, label, false)...)
			if !changedPaths[filePath] {
				found = append(found, fmt.Sprintf("%s references unchanged path %s", label, filePath))
			}
			plannedPaths[filePath] = true
		}
		for blockIndex, block := range step.Blocks {
			label := fmt.Sprintf("steps[%d].blocks[%d]", stepIndex, blockIndex)
			if blockIDs[block.ID] {
				found = append(found, "block IDs must be unique across the review")
			}
			blockIDs[block.ID] = true
			switch block.Type {
			case "diff":
				found = append(found, validatePath(block.Path, label+".path", false)...)
				if !changedPaths[block.Path] {
					found = append(found, fmt.Sprintf("%s.path references unchanged path %s", label, block.Path))
				}
				if block.Focus != nil && block.Focus.EndLine > 0 && block.Focus.EndLine < block.Focus.StartLine {
					found = append(found, label+".focus.end_line cannot be before start_line")
				}
			case "code":
				found = append(found, validatePath(block.Path, label+".path", false)...)
				if block.EndLine < block.StartLine {
					found = append(found, label+".end_line cannot be before start_line")
				}
				if block.EndLine-block.StartLine >= 500 {
					found = append(found, label+" may contain at most 500 lines")
				}
			case "diagram":
				found = append(found, validatePath(block.Path, label+".path", true)...)
				if !strings.HasPrefix(block.Path, "diagrams/") {
					found = append(found, label+".path must be inside diagrams/")
				}
			}
		}
	}

	for changedPath := range changedPaths {
		if !plannedPaths[changedPath] {
			found = append(found, fmt.Sprintf("every changed file must appear in a review step; missing: %s", changedPath))
		}
	}
	return unique(found)
}

func validatePath(value, label string, artifactRelative bool) []string {
	hasTraversal := false
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			hasTraversal = true
		}
	}
	invalid := value == "." || hasTraversal || strings.Contains(value, "\\") || strings.ContainsRune(value, '\x00') || path.IsAbs(value) || path.Clean(value) != value
	if !artifactRelative && (value == ".patchflow" || strings.HasPrefix(value, ".patchflow/")) {
		invalid = true
	}
	if invalid {
		return []string{label + " must be normalized, relative, and stay inside its root"}
	}
	return nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
