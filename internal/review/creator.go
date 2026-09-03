package review

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/code-constructor/patchflow/internal/artifact"
	"github.com/code-constructor/patchflow/internal/gitrepo"
)

// Creator turns a committed Git comparison into a stored baseline review.
type Creator struct {
	Repository *gitrepo.Repository
	Store      *Store
	Now        func() time.Time
}

// Create resolves refs once, builds a complete baseline plan, and persists it.
func (c *Creator) Create(baseRef, targetRef string) (*Stored, error) {
	requestedBase, err := c.Repository.ResolveCommit(baseRef)
	if err != nil {
		return nil, err
	}
	targetSHA, err := c.Repository.ResolveCommit(targetRef)
	if err != nil {
		return nil, err
	}
	baseSHA, err := c.Repository.MergeBase(requestedBase, targetSHA)
	if err != nil {
		return nil, err
	}
	files, err := c.Repository.ChangedFiles(baseSHA, targetSHA)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("the selected commits do not contain reviewable changes")
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	timestamp := now().UTC()
	steps := GeneratePlan(files)
	reviewID := fmt.Sprintf("%s-%s-%s", timestamp.Format("20060102-150405"), targetSHA[:8], randomSuffix())
	value := &artifact.Review{SchemaVersion: 2, ID: reviewID, Repository: artifact.Repository{Name: c.Repository.Name()}, Source: artifact.Source{BaseRef: baseRef, TargetRef: targetRef, BaseSHA: baseSHA, TargetSHA: targetSHA}, CreatedAt: timestamp.Format(time.RFC3339), UpdatedAt: timestamp.Format(time.RFC3339), Status: "draft", Change: artifact.Change{Title: fmt.Sprintf("Review %s against %s", targetRef, baseRef), Summary: fmt.Sprintf("%d changed %s grouped into %d review %s.", len(files), plural(len(files), "file", "files"), len(steps), plural(len(steps), "step", "steps")), Files: files}, OverviewPath: "overview.md", Steps: steps}
	return c.Store.Create(value, overview(value))
}

// overview renders the human-readable introduction beside a baseline artifact.
func overview(value *artifact.Review) string {
	lines := []string{
		"# " + value.Change.Title,
		"",
		"## Purpose and outcome",
		"",
		value.Change.Summary,
		"",
		"## Main decision and review focus",
		"",
		"This baseline plan was generated from repository structure. Ask a Coding Agent to replace this paragraph with the change's purpose, central design decision, and the feedback that matters most before relying on it for a final review.",
		"",
		"## Evidence",
		"",
		fmt.Sprintf("The artifact binds %d changed %s to exact base and target commits. Record relevant tests, checks, benchmarks, screenshots, or manual verification here.", len(value.Change.Files), plural(len(value.Change.Files), "file", "files")),
		"",
		"## Scope, risks, and uncertainty",
		"",
		"Repository structure cannot reveal author intent, non-goals, tradeoffs, or operational risk. A Coding Agent should make those boundaries explicit and distinguish unresolved questions from established behavior.",
		"",
		"## Reading path",
		"",
	}
	for index, step := range value.Steps {
		lines = append(lines, fmt.Sprintf("%d. **%s** — %s", index+1, step.Title, step.Rationale))
	}
	return strings.Join(append(lines, ""), "\n")
}

// randomSuffix adds collision resistance to timestamp-based review IDs.
func randomSuffix() string {
	var value [2]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "0000"
	}
	return fmt.Sprintf("%x", value)
}

// plural selects a singular or plural label for generated prose.
func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
