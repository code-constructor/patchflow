package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/traqx-ai/patchflow/schema"
	yaml "go.yaml.in/yaml/v3"
)

// Discussion is the versioned comments.yaml document stored beside one review.
type Discussion struct {
	SchemaVersion int      `json:"schema_version" yaml:"schema_version"`
	ReviewID      string   `json:"review_id" yaml:"review_id"`
	UpdatedAt     string   `json:"updated_at" yaml:"updated_at"`
	Threads       []Thread `json:"threads" yaml:"threads"`
}

// Thread groups an anchored opening comment and its ordered replies.
type Thread struct {
	ID       string       `json:"id" yaml:"id"`
	Target   ThreadTarget `json:"target" yaml:"target"`
	Resolved bool         `json:"resolved" yaml:"resolved"`
	Comments []Comment    `json:"comments" yaml:"comments"`
}

// ThreadTarget anchors a discussion to a block or immutable source lines.
type ThreadTarget struct {
	Type      string `json:"type" yaml:"type"`
	BlockID   string `json:"block_id" yaml:"block_id"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	Side      string `json:"side,omitempty" yaml:"side,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty" yaml:"commit_sha,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
}

// Comment is one uniquely addressable human or agent message in a thread.
type Comment struct {
	ID         string `json:"id" yaml:"id"`
	Author     string `json:"author" yaml:"author"`
	AuthorKind string `json:"author_kind" yaml:"author_kind"`
	Body       string `json:"body" yaml:"body"`
	CreatedAt  string `json:"created_at" yaml:"created_at"`
	UpdatedAt  string `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	ReplyTo    string `json:"reply_to,omitempty" yaml:"reply_to,omitempty"`
}

// DiscussionValidationErrors collects correctable comments artifact failures.
type DiscussionValidationErrors struct {
	Errors []string `json:"errors"`
}

// Error combines discussion validation failures for command-line and HTTP use.
func (e *DiscussionValidationErrors) Error() string {
	return strings.Join(e.Errors, "\n")
}

// DiscussionValidator validates the standalone comments artifact contract.
type DiscussionValidator struct {
	schema *jsonschema.Schema
}

// NewDiscussionValidator compiles the embedded comments schema for repeated use.
func NewDiscussionValidator() (*DiscussionValidator, error) {
	var document any
	if err := json.Unmarshal(schema.CommentsV1, &document); err != nil {
		return nil, fmt.Errorf("decode embedded comments schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const schemaURL = "https://patchflow.dev/schema/patchflow-comments-v1.schema.json"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		return nil, fmt.Errorf("load embedded comments schema: %w", err)
	}
	compiled, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile embedded comments schema: %w", err)
	}
	return &DiscussionValidator{schema: compiled}, nil
}

// Parse decodes comments YAML and checks both schema and reply relationships.
func (v *DiscussionValidator) Parse(source []byte) (*Discussion, error) {
	var yamlDocument any
	if err := yaml.NewDecoder(bytes.NewReader(source)).Decode(&yamlDocument); err != nil {
		return nil, &DiscussionValidationErrors{Errors: []string{fmt.Sprintf("comments.yaml is not valid YAML: %v", err)}}
	}
	jsonDocument, err := json.Marshal(yamlDocument)
	if err != nil {
		return nil, &DiscussionValidationErrors{Errors: []string{fmt.Sprintf("comments.yaml cannot be represented as JSON: %v", err)}}
	}
	var document any
	if err := json.Unmarshal(jsonDocument, &document); err != nil {
		return nil, err
	}
	if err := v.schema.Validate(document); err != nil {
		return nil, &DiscussionValidationErrors{Errors: []string{err.Error()}}
	}
	var discussion Discussion
	if err := json.Unmarshal(jsonDocument, &discussion); err != nil {
		return nil, fmt.Errorf("decode comments: %w", err)
	}
	if semanticErrors := validateDiscussionSemantics(&discussion); len(semanticErrors) > 0 {
		return nil, &DiscussionValidationErrors{Errors: semanticErrors}
	}
	return &discussion, nil
}

// IsDiscussionValidationError reports invalid persisted discussion content.
func IsDiscussionValidationError(err error) bool {
	var validationErrors *DiscussionValidationErrors
	return errors.As(err, &validationErrors)
}

// ValidateDiscussionForReview checks anchors that depend on the neighboring review artifact.
func ValidateDiscussionForReview(review *Review, discussion *Discussion) error {
	var found []string
	if discussion.ReviewID != review.ID {
		found = append(found, "comments.yaml review_id must match review.yaml id")
	}
	blocks := map[string]Block{}
	for _, step := range review.Steps {
		for _, block := range step.Blocks {
			blocks[block.ID] = block
		}
	}
	for index, thread := range discussion.Threads {
		label := fmt.Sprintf("threads[%d].target", index)
		block, exists := blocks[thread.Target.BlockID]
		if !exists {
			found = append(found, label+".block_id must reference an existing v2 block")
			continue
		}
		if thread.Target.Type != "code" {
			continue
		}
		if block.Type != "code" && block.Type != "diff" {
			found = append(found, label+" must reference a code or diff block")
			continue
		}
		if thread.Target.Path != block.Path {
			found = append(found, label+".path must match its block path")
		}
		expectedSHA := review.Source.TargetSHA
		if thread.Target.Side == "base" {
			expectedSHA = review.Source.BaseSHA
		}
		if thread.Target.CommitSHA != expectedSHA {
			found = append(found, label+".commit_sha must match the recorded "+thread.Target.Side+" commit")
		}
		if block.Type == "code" && (thread.Target.Side != block.Source || thread.Target.StartLine < block.StartLine || thread.Target.EndLine > block.EndLine) {
			found = append(found, label+" range must stay inside its code block")
		}
	}
	if len(found) > 0 {
		return &DiscussionValidationErrors{Errors: unique(found)}
	}
	return nil
}

// validateDiscussionSemantics enforces global IDs, line bounds, and reply order.
func validateDiscussionSemantics(discussion *Discussion) []string {
	var found []string
	ids := map[string]bool{}
	for threadIndex, thread := range discussion.Threads {
		threadLabel := fmt.Sprintf("threads[%d]", threadIndex)
		if ids[thread.ID] {
			found = append(found, "thread and comment IDs must be unique across the discussion")
		}
		ids[thread.ID] = true
		if thread.Target.Type == "code" {
			found = append(found, validatePath(thread.Target.Path, threadLabel+".target.path", false)...)
			if thread.Target.EndLine < thread.Target.StartLine {
				found = append(found, threadLabel+".target.end_line cannot be before start_line")
			}
			if thread.Target.EndLine-thread.Target.StartLine >= 500 {
				found = append(found, threadLabel+".target may contain at most 500 lines")
			}
		}
		priorComments := map[string]bool{}
		for commentIndex, comment := range thread.Comments {
			commentLabel := fmt.Sprintf("%s.comments[%d]", threadLabel, commentIndex)
			if ids[comment.ID] {
				found = append(found, "thread and comment IDs must be unique across the discussion")
			}
			ids[comment.ID] = true
			if commentIndex == 0 && comment.ReplyTo != "" {
				found = append(found, commentLabel+" cannot be a reply")
			}
			if commentIndex > 0 && !priorComments[comment.ReplyTo] {
				found = append(found, commentLabel+".reply_to must reference an earlier comment in the same thread")
			}
			priorComments[comment.ID] = true
		}
	}
	return unique(found)
}
