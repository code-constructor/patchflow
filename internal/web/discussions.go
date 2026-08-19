package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
)

// DiscussionsView presents every review thread in narrative plan order.
type DiscussionsView struct {
	ReviewID, Title, Summary, ThreadLabel string
	ThreadCount                           int
	Chapters                              []DiscussionChapterView
}

// DiscussionChapterView groups annotated blocks beneath their review chapter.
type DiscussionChapterView struct {
	ID, Title, URL, ThreadLabel string
	ThreadCount                 int
	Blocks                      []DiscussionBlockView
}

// DiscussionBlockView groups every thread anchored to one stable review block.
type DiscussionBlockView struct {
	ID, Label, Type, Path, URL, ThreadLabel string
	ThreadCount                             int
	Threads                                 []ThreadView
}

// discussions renders the complete conversation index for one immutable review.
func (a *App) discussions(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	reviewID, ok := parseDiscussionsPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	stored, err := store.Find(reviewID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	discussion, discussionErr := store.ReadDiscussion(stored)
	if discussionErr != nil {
		discussion = &artifact.Discussion{Threads: []artifact.Thread{}}
	}
	reviewerName, reviewerErr := repository.UserName()
	if reviewerErr != nil {
		reviewerName = "Reviewer"
	}
	basePath := requestRepositoryBasePath(r, repository)
	view := buildDiscussionsView(basePath, stored, discussion, reviewerName)
	alert := r.URL.Query().Get("alert")
	if discussionErr != nil {
		alert = discussionErr.Error()
	}
	a.render(w, "discussions", Page{
		Title: "Discussions · " + view.Title + " · Patchflow", BasePath: basePath,
		RepositoryName: repository.Name(), GitHub: githubLinkView(repository, stored.Review.Source.TargetRef),
		ReviewNavigation: reviewNavigationView(basePath, stored, "discussions"), Discussions: &view,
		Notice: r.URL.Query().Get("notice"), Alert: alert,
	}, http.StatusOK)
}

// buildDiscussionsView groups persisted threads by their chapter and block order.
func buildDiscussionsView(basePath string, stored *patchreview.Stored, discussion *artifact.Discussion, reviewerName string) DiscussionsView {
	view := DiscussionsView{ReviewID: stored.Review.ID, Title: stored.Review.Change.Title, Summary: stored.Review.Change.Summary}
	threadsByBlock := map[string][]artifact.Thread{}
	for _, thread := range discussion.Threads {
		threadsByBlock[thread.Target.BlockID] = append(threadsByBlock[thread.Target.BlockID], thread)
	}
	for _, step := range stored.Review.Steps {
		chapter := DiscussionChapterView{ID: step.ID, Title: step.Title, URL: basePath + "/reviews/" + stored.Review.ID + "/steps/" + step.ID}
		blocks := step.Blocks
		if stored.Review.SchemaVersion == 1 {
			blocks = legacyBlocks(step)
		}
		for _, block := range blocks {
			threads := threadsByBlock[block.ID]
			if len(threads) == 0 {
				continue
			}
			blockView := DiscussionBlockView{
				ID: block.ID, Label: blockLabel(block), Type: block.Type, Path: block.Path,
				URL:         basePath + "/reviews/" + stored.Review.ID + "/blocks/" + block.ID,
				ThreadCount: len(threads), ThreadLabel: discussionCountLabel(len(threads)),
			}
			for _, thread := range threads {
				blockView.Threads = append(blockView.Threads, buildThreadView(basePath, stored.Review.ID, reviewerName, thread, "", ""))
			}
			chapter.ThreadCount += len(threads)
			chapter.Blocks = append(chapter.Blocks, blockView)
		}
		if chapter.ThreadCount == 0 {
			continue
		}
		chapter.ThreadLabel = discussionCountLabel(chapter.ThreadCount)
		view.ThreadCount += chapter.ThreadCount
		view.Chapters = append(view.Chapters, chapter)
	}
	view.ThreadLabel = discussionCountLabel(view.ThreadCount)
	return view
}

// discussionCountLabel formats a stable human-readable thread count.
func discussionCountLabel(count int) string {
	if count == 1 {
		return "1 discussion"
	}
	return fmt.Sprintf("%d discussions", count)
}

// parseDiscussionsPath extracts the review ID from one exact discussions route.
func parseDiscussionsPath(requestPath string) (string, bool) {
	parts := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "reviews" || parts[1] == "" || parts[2] != "discussions" {
		return "", false
	}
	return parts[1], true
}

// matchDiscussionsPath recognizes the complete conversation index route.
func matchDiscussionsPath(requestPath string) bool {
	_, ok := parseDiscussionsPath(requestPath)
	return ok
}
