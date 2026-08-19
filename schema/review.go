// Package schema exposes Patchflow's versioned artifact schemas to Go binaries.
package schema

import _ "embed"

// ReviewV2 is the canonical JSON Schema for Patchflow Review Artifact v2.
//
//go:embed patchflow-review-v2.schema.json
var ReviewV2 []byte

// CommentsV1 is the canonical JSON Schema for persisted Patchflow discussions.
//
//go:embed patchflow-comments-v1.schema.json
var CommentsV1 []byte
