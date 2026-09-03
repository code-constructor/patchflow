package review

import (
	"fmt"
	"strings"

	"github.com/code-constructor/patchflow/internal/artifact"
	"github.com/code-constructor/patchflow/internal/gitrepo"
)

// VerificationErrors collects repository-backed problems an agent can correct.
type VerificationErrors struct {
	Errors []string `json:"errors"`
}

// Error renders every failed repository assertion on its own line.
func (e *VerificationErrors) Error() string {
	return strings.Join(e.Errors, "\n")
}

// Verifier proves that a parsed review still points at readable Git evidence and assets.
type Verifier struct {
	Repository *gitrepo.Repository
	Store      *Store
}

// Verify checks immutable commits, the complete changed-file inventory, and block resources.
func (v *Verifier) Verify(stored *Stored) error {
	if v.Repository == nil || v.Store == nil || stored == nil || stored.Review == nil {
		return &VerificationErrors{Errors: []string{"review verification requires a repository, store, and parsed review"}}
	}
	review := stored.Review
	found := []string{}
	if review.Repository.Name != v.Repository.Name() {
		found = append(found, fmt.Sprintf("repository.name is %q but the selected repository is %q", review.Repository.Name, v.Repository.Name()))
	}
	baseReadable := verifyCommit(v.Repository, "source.base_sha", review.Source.BaseSHA, &found)
	targetReadable := verifyCommit(v.Repository, "source.target_sha", review.Source.TargetSHA, &found)
	if baseReadable && targetReadable {
		files, err := v.Repository.ChangedFiles(review.Source.BaseSHA, review.Source.TargetSHA)
		if err != nil {
			found = append(found, fmt.Sprintf("cannot read the recorded Git comparison: %v", err))
		} else {
			found = append(found, verifyChangedFiles(review.Change.Files, files)...)
		}
	}
	if _, err := v.Store.ReadOverview(stored); err != nil {
		found = append(found, "overview_path cannot be read: "+err.Error())
	}
	for stepIndex, step := range review.Steps {
		for blockIndex, block := range step.Blocks {
			label := fmt.Sprintf("steps[%d].blocks[%d] (%s)", stepIndex, blockIndex, block.ID)
			switch block.Type {
			case "code":
				sha, readable := review.Source.TargetSHA, targetReadable
				if block.Source == "base" {
					sha, readable = review.Source.BaseSHA, baseReadable
				}
				if !readable {
					continue
				}
				lineCount, err := v.Repository.FileLineCount(sha, block.Path)
				if err != nil {
					found = append(found, fmt.Sprintf("%s cannot read %s at the recorded %s commit: %v", label, block.Path, block.Source, err))
				} else if block.EndLine > lineCount {
					found = append(found, fmt.Sprintf("%s requests lines %d-%d but %s has %d lines at the recorded %s commit", label, block.StartLine, block.EndLine, block.Path, lineCount, block.Source))
				}
			case "diagram", "image":
				if _, err := v.Store.ReadAssetBytes(stored, block.Path); err != nil {
					found = append(found, fmt.Sprintf("%s cannot read asset %s: %v", label, block.Path, err))
				}
			}
		}
	}
	if len(found) > 0 {
		return &VerificationErrors{Errors: found}
	}
	return nil
}

// verifyCommit confirms that one full immutable identifier resolves to a commit object.
func verifyCommit(repository *gitrepo.Repository, label, sha string, found *[]string) bool {
	if _, err := repository.ResolveCommit(sha); err != nil {
		*found = append(*found, fmt.Sprintf("%s %q is not an available commit: %v", label, sha, err))
		return false
	}
	return true
}

// verifyChangedFiles compares the artifact inventory with Git without requiring an order.
func verifyChangedFiles(recorded, actual []artifact.ChangedFile) []string {
	found := []string{}
	actualByPath := make(map[string]artifact.ChangedFile, len(actual))
	for _, file := range actual {
		actualByPath[file.Path] = file
	}
	recordedByPath := make(map[string]artifact.ChangedFile, len(recorded))
	for _, file := range recorded {
		recordedByPath[file.Path] = file
		gitFile, ok := actualByPath[file.Path]
		if !ok {
			found = append(found, fmt.Sprintf("change.files contains %s, but that path is not changed between the recorded commits", file.Path))
			continue
		}
		if file.Status != gitFile.Status || file.PreviousPath != gitFile.PreviousPath {
			found = append(found, fmt.Sprintf("change.files entry for %s does not match Git: recorded status=%s previous_path=%q; actual status=%s previous_path=%q", file.Path, file.Status, file.PreviousPath, gitFile.Status, gitFile.PreviousPath))
		}
	}
	for _, file := range actual {
		if _, ok := recordedByPath[file.Path]; !ok {
			found = append(found, fmt.Sprintf("change.files is missing changed path %s", file.Path))
		}
	}
	return found
}
