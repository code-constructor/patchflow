package web

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
)

// ReviewNavigationView describes the global views available within one review.
type ReviewNavigationView struct {
	ReviewID, ActiveView, PlanPath, FilesPath, MemoryKey, ReviewPath string
	FileCount                                                        int
}

// FilesView contains the complete changed-file tree and optional selected diff.
type FilesView struct {
	BasePath, ReviewID, Title, Summary, FileLabel string
	FileCount                                     int
	Tree                                          []*FileTreeNode
	Selected                                      *SelectedFileView
}

// SelectedFileView combines one changed-file record with its rendered patch.
type SelectedFileView struct {
	Path, PreviousPath, Status, StatusLabel string
	Diff                                    BlockView
}

// FileTreeNode represents a directory or changed path in the review file tree.
type FileTreeNode struct {
	Name, Path, URL, Status, StatusCode string
	Directory, Active                   bool
	Children                            []*FileTreeNode
}

// files renders the classic changed-file view for one immutable review comparison.
func (a *App) files(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	reviewID, selectedPath, ok := parseFilesPath(r.URL.Path)
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
	basePath := requestRepositoryBasePath(r, repository)
	view := FilesView{
		BasePath:  basePath,
		ReviewID:  reviewID,
		Title:     stored.Review.Change.Title,
		Summary:   stored.Review.Change.Summary,
		FileCount: len(stored.Review.Change.Files),
		FileLabel: changedFilesLabel(len(stored.Review.Change.Files)),
		Tree:      buildFileTree(basePath, stored, selectedPath),
	}
	alert := r.URL.Query().Get("alert")
	if selectedPath != "" {
		selected, found := findChangedFile(stored.Review.Change.Files, selectedPath)
		if !found {
			http.NotFound(w, r)
			return
		}
		view.Selected, alert = buildSelectedFileView(repository, store, stored, basePath, selected, alert)
	}
	a.render(w, "files", Page{
		Title:            "Files changed · " + view.Title + " · Patchflow",
		BasePath:         basePath,
		RepositoryName:   repository.Name(),
		GitHub:           githubLinkView(repository, stored.Review.Source.TargetRef),
		ReviewNavigation: reviewNavigationView(basePath, stored, "files"),
		Files:            &view,
		Notice:           r.URL.Query().Get("notice"),
		Alert:            alert,
	}, http.StatusOK)
}

// changedFilesLabel formats a human-readable changed-path count.
func changedFilesLabel(count int) string {
	if count == 1 {
		return "1 changed file"
	}
	return fmt.Sprintf("%d changed files", count)
}

// buildSelectedFileView resolves one exact-SHA patch and any discussion attached in the plan.
func buildSelectedFileView(repository *gitrepo.Repository, store *patchreview.Store, stored *patchreview.Stored, basePath string, file artifact.ChangedFile, alert string) (*SelectedFileView, string) {
	block, commentable := reviewDiffBlock(stored.Review, file.Path)
	blockIDs := reviewBlockIDsForPath(stored.Review, file.Path)
	if !commentable {
		block = artifact.Block{ID: fileViewID(file.Path), Type: "diff", Path: file.Path, View: "split"}
	}
	block.Collapsed = false
	block.Focus = nil
	diff := buildBlock(repository, store, stored, map[string]artifact.ChangedFile{file.Path: file}, block)
	diff.ReferencePath = reviewFilePath(basePath, stored.Review.ID, file.Path)
	diff.ReferenceLabel = "file " + file.Path
	diff.Label = file.Path
	diff.Commentable = commentable && stored.Review.SchemaVersion == 2
	if len(blockIDs) > 0 && stored.Review.SchemaVersion == 2 {
		discussion, err := store.ReadDiscussion(stored)
		if err != nil {
			alert = err.Error()
			discussion = &artifact.Discussion{Threads: []artifact.Thread{}}
		}
		reviewerName, err := repository.UserName()
		if err != nil {
			reviewerName = "Reviewer"
		}
		diff.ReviewerName = reviewerName
		diff.Threads, diff.ThreadAnchors = buildThreadViewsForBlocks(basePath, stored.Review.ID, blockIDs, reviewerName, discussion, "", "")
		if diff.Commentable {
			diff.ThreadAction = basePath + "/reviews/" + stored.Review.ID + "/blocks/" + block.ID + "/threads"
		}
	}
	return &SelectedFileView{Path: file.Path, PreviousPath: file.PreviousPath, Status: file.Status, StatusLabel: humanize(file.Status), Diff: diff}, alert
}

// reviewNavigationView creates stable tab destinations for one stored review.
func reviewNavigationView(basePath string, stored *patchreview.Stored, activeView string) *ReviewNavigationView {
	reviewPath := basePath + "/reviews/" + stored.Review.ID
	filesPath := reviewPath + "/files"
	if len(stored.Review.Change.Files) > 0 {
		filesPath = reviewFilePath(basePath, stored.Review.ID, stored.Review.Change.Files[0].Path)
	}
	return &ReviewNavigationView{
		ReviewID: stored.Review.ID, ActiveView: activeView, PlanPath: reviewPath,
		FilesPath: filesPath, MemoryKey: repositoryScopeFromPath(basePath) + ":" + stored.Review.ID,
		ReviewPath: reviewPath, FileCount: len(stored.Review.Change.Files),
	}
}

// repositoryScopeFromPath returns the opaque repository key used in transient tab memory.
func repositoryScopeFromPath(basePath string) string {
	return strings.TrimPrefix(basePath, "/repositories/")
}

// parseFilesPath extracts a review ID and optional changed path from a file-view route.
func parseFilesPath(requestPath string) (string, string, bool) {
	parts := strings.SplitN(strings.Trim(requestPath, "/"), "/", 4)
	if len(parts) < 3 || parts[0] != "reviews" || parts[1] == "" || parts[2] != "files" {
		return "", "", false
	}
	if len(parts) == 3 {
		return parts[1], "", true
	}
	return parts[1], parts[3], parts[3] != ""
}

// matchFilesPath recognizes the file index and nested changed-file resources.
func matchFilesPath(requestPath string) bool {
	_, _, ok := parseFilesPath(requestPath)
	return ok
}

// reviewFilePath creates an encoded resource URL while preserving directory hierarchy.
func reviewFilePath(basePath, reviewID, filePath string) string {
	segments := strings.Split(filePath, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return basePath + "/reviews/" + reviewID + "/files/" + strings.Join(segments, "/")
}

// reviewDiffBlock returns the first planned diff block that owns discussions for a path.
func reviewDiffBlock(review *artifact.Review, filePath string) (artifact.Block, bool) {
	for _, step := range review.Steps {
		blocks := step.Blocks
		if review.SchemaVersion == 1 {
			blocks = legacyBlocks(step)
		}
		for _, block := range blocks {
			if block.Type == "diff" && block.Path == filePath {
				return block, true
			}
		}
	}
	return artifact.Block{}, false
}

// reviewBlockIDsForPath finds every narrative evidence block associated with a file.
func reviewBlockIDsForPath(review *artifact.Review, filePath string) map[string]bool {
	blockIDs := map[string]bool{}
	for _, step := range review.Steps {
		blocks := step.Blocks
		if review.SchemaVersion == 1 {
			blocks = legacyBlocks(step)
		}
		for _, block := range blocks {
			if (block.Type == "diff" || block.Type == "code") && block.Path == filePath {
				blockIDs[block.ID] = true
			}
		}
	}
	return blockIDs
}

// findChangedFile resolves only paths recorded in the immutable review artifact.
func findChangedFile(files []artifact.ChangedFile, filePath string) (artifact.ChangedFile, bool) {
	for _, file := range files {
		if file.Path == filePath {
			return file, true
		}
	}
	return artifact.ChangedFile{}, false
}

// buildFileTree groups changed paths into sorted directory nodes.
func buildFileTree(basePath string, stored *patchreview.Stored, activePath string) []*FileTreeNode {
	root := &FileTreeNode{Directory: true}
	for _, file := range stored.Review.Change.Files {
		parts := strings.Split(file.Path, "/")
		parent := root
		currentPath := ""
		for index, name := range parts {
			currentPath = strings.TrimPrefix(currentPath+"/"+name, "/")
			if index == len(parts)-1 {
				parent.Children = append(parent.Children, &FileTreeNode{
					Name: name, Path: file.Path, URL: reviewFilePath(basePath, stored.Review.ID, file.Path),
					Status: file.Status, StatusCode: fileStatusCode(file.Status), Active: file.Path == activePath,
				})
				continue
			}
			directory := childDirectory(parent, name)
			if directory == nil {
				directory = &FileTreeNode{Name: name, Path: currentPath, Directory: true}
				parent.Children = append(parent.Children, directory)
			}
			parent = directory
		}
	}
	finalizeFileTree(root)
	return root.Children
}

// childDirectory finds an existing directory beneath one mutable tree node.
func childDirectory(parent *FileTreeNode, name string) *FileTreeNode {
	for _, child := range parent.Children {
		if child.Directory && child.Name == name {
			return child
		}
	}
	return nil
}

// finalizeFileTree sorts directories first and marks ancestors of the selected file.
func finalizeFileTree(node *FileTreeNode) bool {
	active := node.Active
	for _, child := range node.Children {
		active = finalizeFileTree(child) || active
	}
	node.Active = active
	sort.Slice(node.Children, func(left, right int) bool {
		if node.Children[left].Directory != node.Children[right].Directory {
			return node.Children[left].Directory
		}
		return strings.ToLower(node.Children[left].Name) < strings.ToLower(node.Children[right].Name)
	})
	return active
}

// fileStatusCode maps an artifact status to a compact tree marker.
func fileStatusCode(status string) string {
	codes := map[string]string{"added": "A", "modified": "M", "deleted": "D", "renamed": "R", "copied": "C", "type_changed": "T", "unmerged": "U"}
	if code := codes[status]; code != "" {
		return code
	}
	return "?"
}

// fileViewID creates a safe synthetic DOM identity for an unplanned changed file.
func fileViewID(filePath string) string {
	digest := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("file-%x", digest[:8])
}
