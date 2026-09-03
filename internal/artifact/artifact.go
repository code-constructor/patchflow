package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/code-constructor/patchflow/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	yaml "go.yaml.in/yaml/v3"
)

// Review is the complete versioned document stored in review.yaml.
type Review struct {
	SchemaVersion int          `json:"schema_version" yaml:"schema_version"`
	ID            string       `json:"id" yaml:"id"`
	Repository    Repository   `json:"repository" yaml:"repository"`
	Source        Source       `json:"source" yaml:"source"`
	CreatedAt     string       `json:"created_at" yaml:"created_at"`
	UpdatedAt     string       `json:"updated_at" yaml:"updated_at"`
	Status        string       `json:"status" yaml:"status"`
	Change        Change       `json:"change" yaml:"change"`
	OverviewPath  string       `json:"overview_path" yaml:"overview_path"`
	Steps         []Step       `json:"steps" yaml:"steps"`
	Annotations   []Annotation `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Decisions     []Decision   `json:"decisions,omitempty" yaml:"decisions,omitempty"`
}

// Repository identifies the reviewed project for display purposes.
type Repository struct {
	Name string `json:"name" yaml:"name"`
}

// Source records both requested refs and the immutable commits they resolved to.
type Source struct {
	BaseRef   string `json:"base_ref" yaml:"base_ref"`
	TargetRef string `json:"target_ref" yaml:"target_ref"`
	BaseSHA   string `json:"base_sha" yaml:"base_sha"`
	TargetSHA string `json:"target_sha" yaml:"target_sha"`
}

// Change summarizes the comparison and inventories every changed path.
type Change struct {
	Title   string        `json:"title" yaml:"title"`
	Summary string        `json:"summary" yaml:"summary"`
	Files   []ChangedFile `json:"files" yaml:"files"`
}

// ChangedFile records one path and its Git change status.
type ChangedFile struct {
	Path         string `json:"path" yaml:"path"`
	Status       string `json:"status" yaml:"status"`
	PreviousPath string `json:"previous_path,omitempty" yaml:"previous_path,omitempty"`
}

// Step is one ordered review chapter with its evidence blocks.
type Step struct {
	ID             string   `json:"id" yaml:"id"`
	Title          string   `json:"title" yaml:"title"`
	Priority       string   `json:"priority" yaml:"priority"`
	Rationale      string   `json:"rationale" yaml:"rationale"`
	ReviewQuestion string   `json:"review_question,omitempty" yaml:"review_question,omitempty"`
	Attention      []string `json:"attention,omitempty" yaml:"attention,omitempty"`
	Files          []string `json:"files" yaml:"files"`
	Blocks         []Block  `json:"blocks,omitempty" yaml:"blocks,omitempty"`
}

// Block is one typed narrative building block in a v2 chapter.
type Block struct {
	ID        string `json:"id" yaml:"id"`
	Type      string `json:"type" yaml:"type"`
	Body      string `json:"body,omitempty" yaml:"body,omitempty"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	View      string `json:"view,omitempty" yaml:"view,omitempty"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
	Kind      string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Alt       string `json:"alt,omitempty" yaml:"alt,omitempty"`
	Caption   string `json:"caption,omitempty" yaml:"caption,omitempty"`
	Focus     *Focus `json:"focus,omitempty" yaml:"focus,omitempty"`
	Collapsed bool   `json:"collapsed,omitempty" yaml:"collapsed,omitempty"`
}

// Focus narrows a diff block to an important line range on one side.
type Focus struct {
	Side      string `json:"side" yaml:"side"`
	StartLine int    `json:"start_line" yaml:"start_line"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
}

// Annotation preserves a legacy v1 review comment.
type Annotation struct {
	ID        string `json:"id" yaml:"id"`
	Scope     string `json:"scope" yaml:"scope"`
	Body      string `json:"body" yaml:"body"`
	FilePath  string `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	Side      string `json:"side,omitempty" yaml:"side,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
	CreatedAt string `json:"created_at" yaml:"created_at"`
}

// Decision preserves a legacy v1 review outcome.
type Decision struct {
	ID        string `json:"id" yaml:"id"`
	Summary   string `json:"summary" yaml:"summary"`
	Status    string `json:"status" yaml:"status"`
	Rationale string `json:"rationale,omitempty" yaml:"rationale,omitempty"`
	CreatedAt string `json:"created_at" yaml:"created_at"`
}

// ValidationErrors collects user-correctable artifact contract failures.
type ValidationErrors struct {
	Errors []string `json:"errors"`
}

// Error combines all validation failures into the standard error representation.
func (e *ValidationErrors) Error() string {
	return strings.Join(e.Errors, "\n")
}

// Validator parses v1 artifacts and validates v2 artifacts against the embedded contract.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator compiles the embedded v2 JSON Schema for repeated artifact checks.
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

// Parse decodes YAML, applies the versioned schema, and checks cross-field invariants.
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
	var review Review
	if err := json.Unmarshal(jsonDocument, &review); err != nil {
		return nil, fmt.Errorf("decode review: %w", err)
	}
	if review.SchemaVersion == 2 {
		if err := v.schema.Validate(document); err != nil {
			return nil, &ValidationErrors{Errors: []string{err.Error()}}
		}
	} else if review.SchemaVersion != 1 {
		return nil, &ValidationErrors{Errors: []string{"schema_version must be 1 or 2"}}
	}
	if semanticErrors := validateSemantics(&review); len(semanticErrors) > 0 {
		return nil, &ValidationErrors{Errors: semanticErrors}
	}
	return &review, nil
}

// IsValidationError reports whether err represents invalid artifact content.
func IsValidationError(err error) bool {
	var validationErrors *ValidationErrors
	return errors.As(err, &validationErrors)
}

// validateSemantics checks relationships and safety rules that JSON Schema cannot express.
func validateSemantics(review *Review) []string {
	var found []string
	if review.ID == "" || review.Repository.Name == "" || review.Change.Title == "" || review.Change.Summary == "" {
		found = append(found, "review metadata must contain non-empty text")
	}
	if review.OverviewPath != "overview.md" {
		found = append(found, "overview_path must be overview.md")
	}
	if len(review.Change.Files) == 0 || len(review.Steps) == 0 {
		found = append(found, "change.files and steps must be non-empty")
	}
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
			case "image":
				found = append(found, validatePath(block.Path, label+".path", true)...)
				if !strings.HasPrefix(block.Path, "assets/") {
					found = append(found, label+".path must be inside assets/")
				}
				extension := strings.ToLower(path.Ext(block.Path))
				if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".gif" && extension != ".webp" {
					found = append(found, label+".path must use PNG, JPEG, GIF, or WebP")
				}
			}
		}
		if review.SchemaVersion == 2 && len(step.Blocks) == 0 {
			found = append(found, fmt.Sprintf("steps[%d].blocks must be non-empty", stepIndex))
		}
	}

	for changedPath := range changedPaths {
		if !plannedPaths[changedPath] {
			found = append(found, fmt.Sprintf("every changed file must appear in a review step; missing: %s", changedPath))
		}
	}
	if review.SchemaVersion == 1 {
		for index, annotation := range review.Annotations {
			if annotation.ID == "" || strings.TrimSpace(annotation.Body) == "" {
				found = append(found, fmt.Sprintf("annotations[%d] requires id and body", index))
			}
			if annotation.Scope != "overview" && annotation.Scope != "file" && annotation.Scope != "line" {
				found = append(found, fmt.Sprintf("annotations[%d].scope is invalid", index))
			}
			if annotation.Scope != "overview" && !changedPaths[annotation.FilePath] {
				found = append(found, fmt.Sprintf("annotations[%d].file_path references unchanged path", index))
			}
			if annotation.Scope == "line" && (annotation.Side != "base" && annotation.Side != "target" || annotation.StartLine < 1 || annotation.EndLine > 0 && annotation.EndLine < annotation.StartLine) {
				found = append(found, fmt.Sprintf("annotations[%d] has an invalid line range", index))
			}
		}
	}
	return unique(found)
}

// validatePath rejects non-normalized paths and paths outside their permitted root.
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

// unique preserves the first occurrence of each validation message.
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
