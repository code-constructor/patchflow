package review

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/traqx-ai/patchflow/internal/artifact"
)

type category struct {
	id, title, priority, rationale string
	pattern                        *regexp.Regexp
}

var categories = []category{
	{"security", "Inspect security-sensitive behavior", "critical", "Authentication, authorization, session, permission, or credential paths can change the trust boundary.", regexp.MustCompile(`(?i)(auth|session|permission|policy|policies|security|credential|password|token)`)},
	{"domain", "Understand domain behavior", "high", "Domain models and services define the behavior that supporting layers depend on.", regexp.MustCompile(`^(app/(models|services)|lib|domain)/`)},
	{"data", "Follow data and configuration changes", "high", "Persistence and configuration changes shape the state available to the application.", regexp.MustCompile(`(?i)(^(db/|config/)|(schema|migration))`)},
	{"interfaces", "Trace entry points and interfaces", "high", "Routes, controllers, and public interfaces show how the change enters and leaves the system.", regexp.MustCompile(`^(app/controllers|config/routes|app/(graphql|channels)|api)/?`)},
	{"execution", "Trace background and command execution", "medium", "Jobs, commands, and executables reveal asynchronous or operational effects.", regexp.MustCompile(`^(app/jobs|bin|script|lib/tasks)/`)},
	{"presentation", "Review presentation changes", "low", "Views, styles, and browser behavior are easiest to assess after their supporting behavior is understood.", regexp.MustCompile(`^(app/views|app/helpers|app/assets|app/javascript|public)/`)},
	{"tests", "Verify the intended behavior", "medium", "Tests document expected behavior and expose missing cases after the implementation path is understood.", regexp.MustCompile(`^(test|spec)/`)},
	{"generated", "Scan generated and vendored files", "low", "Generated, minified, and vendored files are usually better verified through their source or generation process than line by line.", regexp.MustCompile(`(?i)(^(vendor|node_modules)/|\.min\.(css|js)$|\.lock$)`)},
	{"supporting", "Review supporting changes", "low", "These files support the change but do not match a more important execution or domain path.", regexp.MustCompile(`.*`)},
}

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
		for index, path := range matched {
			blocks = append(blocks, artifact.Block{ID: fmt.Sprintf("%s-diff-%d", rule.id, index+1), Type: "diff", Path: path, View: "split"})
		}
		result = append(result, artifact.Step{ID: rule.id, Title: rule.title, Priority: rule.priority, Rationale: rule.rationale, Files: matched, Blocks: blocks})
	}
	return result
}
