package review

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
)

type category struct {
	id             string
	title          string
	priority       string
	rationale      string
	reviewQuestion string
	takeaway       string
	attention      []string
	collapseDiffs  bool
	pattern        *regexp.Regexp
}

var categories = []category{
	{
		id:             "security",
		title:          "Inspect security-sensitive behavior",
		priority:       "critical",
		rationale:      "Authentication, authorization, session, permission, or credential paths can change the trust boundary.",
		reviewQuestion: "Does the change preserve the intended trust boundary and failure behavior?",
		takeaway:       "The reviewer can explain who is trusted, what is validated, and how access fails safely.",
		attention:      []string{"security", "design"},
		pattern:        regexp.MustCompile(`(?i)(auth|session|permission|policy|policies|security|credential|password|token)`),
	},
	{
		id:             "domain",
		title:          "Understand domain behavior",
		priority:       "high",
		rationale:      "Domain models and services define the behavior that supporting layers depend on.",
		reviewQuestion: "Does the domain behavior express the intended rule in the right owner?",
		takeaway:       "The reviewer can state the changed rule and the component responsible for enforcing it.",
		attention:      []string{"behavior", "design"},
		pattern:        regexp.MustCompile(`^(app/(models|services)|lib|domain)/`),
	},
	{
		id:             "data",
		title:          "Follow data and configuration changes",
		priority:       "high",
		rationale:      "Persistence and configuration changes shape the state available to the application.",
		reviewQuestion: "Can the new state be introduced, read, and changed without losing compatibility or data?",
		takeaway:       "The reviewer understands the state transition, compatibility boundary, and recovery path.",
		attention:      []string{"data", "reliability"},
		pattern:        regexp.MustCompile(`(?i)(^(db/|config/)|(schema|migration))`),
	},
	{
		id:             "interfaces",
		title:          "Trace entry points and interfaces",
		priority:       "high",
		rationale:      "Routes, controllers, and public interfaces show how the change enters and leaves the system.",
		reviewQuestion: "Does the external contract expose the intended behavior without leaking internal policy?",
		takeaway:       "The reviewer can follow input validation through the public result.",
		attention:      []string{"behavior", "design"},
		pattern:        regexp.MustCompile(`^(app/controllers|config/routes|app/(graphql|channels)|api)/?`),
	},
	{
		id:             "execution",
		title:          "Trace background and command execution",
		priority:       "medium",
		rationale:      "Jobs, commands, and executables reveal asynchronous or operational effects.",
		reviewQuestion: "Does execution start, fail, and stop predictably under real operating conditions?",
		takeaway:       "The reviewer understands execution ownership, failure propagation, and operational effects.",
		attention:      []string{"operations", "reliability"},
		pattern:        regexp.MustCompile(`^(app/jobs|bin|script|lib/tasks)/`),
	},
	{
		id:             "presentation",
		title:          "Review presentation changes",
		priority:       "low",
		rationale:      "Views, styles, and browser behavior are easiest to assess after their supporting behavior is understood.",
		reviewQuestion: "Does the interface communicate the new behavior clearly and remain usable at its boundaries?",
		takeaway:       "The reviewer can connect the visible experience to the supporting behavior.",
		attention:      []string{"ux", "behavior"},
		pattern:        regexp.MustCompile(`^(app/views|app/helpers|app/assets|app/javascript|public)/`),
	},
	{
		id:             "tests",
		title:          "Verify the intended behavior",
		priority:       "medium",
		rationale:      "Tests document expected behavior and expose missing cases after the implementation path is understood.",
		reviewQuestion: "Would these tests fail for the important regressions the change could introduce?",
		takeaway:       "The evidence covers the intended behavior and its meaningful edge cases.",
		attention:      []string{"verification"},
		pattern:        regexp.MustCompile(`(^|/)(test|spec)(/|_)|_test\.go$`),
	},
	{
		id:             "generated",
		title:          "Scan generated and vendored files",
		priority:       "low",
		rationale:      "Generated, minified, and vendored files are usually better verified through their source or generation process than line by line.",
		reviewQuestion: "Can this mechanical output be trusted from its source and generation process?",
		takeaway:       "The reviewer has verified provenance and unexpected scope without reading repetitive output line by line.",
		attention:      []string{"mechanical"},
		collapseDiffs:  true,
		pattern:        regexp.MustCompile(`(?i)(^(vendor|node_modules)/|\.min\.(css|js)$|\.lock$)`),
	},
	{
		id:             "supporting",
		title:          "Review supporting changes",
		priority:       "low",
		rationale:      "These files support the change but do not match a more important execution or domain path.",
		reviewQuestion: "Do the remaining changes support the intended behavior without adding unrelated scope?",
		takeaway:       "Every remaining path is accounted for and consistent with the change's purpose.",
		attention:      []string{"documentation"},
		pattern:        regexp.MustCompile(`.*`),
	},
}

var nonIDCharacters = regexp.MustCompile(`[^a-z0-9]+`)

// GeneratePlan groups every changed file into an ordered structural review baseline.
func GeneratePlan(files []artifact.ChangedFile) []artifact.Step {
	remaining := make([]string, len(files))
	for i, file := range files {
		remaining[i] = file.Path
	}
	var result []artifact.Step
	for _, rule := range categories {
		var matched, unmatched []string
		for _, path := range remaining {
			if rule.pattern.MatchString(path) {
				matched = append(matched, path)
			} else {
				unmatched = append(unmatched, path)
			}
		}
		remaining = unmatched
		if len(matched) == 0 {
			continue
		}
		sort.Strings(matched)
		blocks := []artifact.Block{{ID: rule.id + "-intro", Type: "prose", Body: rule.rationale}}
		for _, path := range matched {
			blocks = append(blocks, artifact.Block{ID: diffBlockID(rule.id, path), Type: "diff", Path: path, View: "split", Collapsed: rule.collapseDiffs})
		}
		blocks = append(blocks, artifact.Block{ID: rule.id + "-takeaway", Type: "takeaway", Body: rule.takeaway})
		result = append(result, artifact.Step{ID: rule.id, Title: rule.title, Priority: rule.priority, Rationale: rule.rationale, ReviewQuestion: rule.reviewQuestion, Attention: rule.attention, Files: matched, Blocks: blocks})
	}
	return result
}

// diffBlockID derives a readable stable ID from the category and evidence path.
func diffBlockID(categoryID, path string) string {
	slug := strings.Trim(nonIDCharacters.ReplaceAllString(strings.ToLower(path), "-"), "-")
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], "-")
	}
	digest := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%s-%s-%x", categoryID, slug, digest[:4])
}
