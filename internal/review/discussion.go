package review

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/traqx-ai/patchflow/internal/artifact"
)

// DiscussionService creates durable comment threads and replies for one store.
type DiscussionService struct {
	Store *Store
	Now   func() time.Time
	NewID func(string) (string, error)
}

// NewThread contains reviewer input for one block or code-range discussion.
type NewThread struct {
	BlockID    string
	TargetType string
	Path       string
	Side       string
	StartLine  int
	EndLine    int
	Author     string
	AuthorKind string
	Body       string
}

// NewReply contains one response to an existing comment thread.
type NewReply struct {
	ReplyTo    string
	Author     string
	AuthorKind string
	Body       string
}

// CreateThread validates an anchor against its immutable review and persists its opening comment.
func (s *DiscussionService) CreateThread(reviewID string, input NewThread) (*artifact.Thread, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("discussion service requires a store")
	}
	if err := validateCommentInput(input.Author, input.AuthorKind, input.Body); err != nil {
		return nil, err
	}
	location, err := s.Store.FindBlock(reviewID, input.BlockID)
	if err != nil {
		return nil, err
	}
	target, err := buildThreadTarget(location, input)
	if err != nil {
		return nil, err
	}
	discussion, err := s.Store.ReadDiscussion(location.Stored)
	if err != nil {
		return nil, err
	}
	threadID, err := s.newID("thread")
	if err != nil {
		return nil, err
	}
	commentID, err := s.newID("comment")
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Format(time.RFC3339)
	thread := artifact.Thread{
		ID:       threadID,
		Target:   target,
		Resolved: false,
		Comments: []artifact.Comment{{ID: commentID, Author: strings.TrimSpace(input.Author), AuthorKind: input.AuthorKind, Body: strings.TrimSpace(input.Body), CreatedAt: now}},
	}
	discussion.UpdatedAt = now
	discussion.Threads = append(discussion.Threads, thread)
	if err := s.Store.WriteDiscussion(location.Stored, discussion); err != nil {
		return nil, err
	}
	return &thread, nil
}

// Reply appends a uniquely addressable response to an existing thread.
func (s *DiscussionService) Reply(reviewID, threadID string, input NewReply) (*artifact.Comment, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("discussion service requires a store")
	}
	if err := validateCommentInput(input.Author, input.AuthorKind, input.Body); err != nil {
		return nil, err
	}
	location, err := s.Store.FindThread(reviewID, threadID)
	if err != nil {
		return nil, err
	}
	thread := &location.Discussion.Threads[location.ThreadIndex]
	replyTo := input.ReplyTo
	if replyTo == "" {
		replyTo = thread.Comments[len(thread.Comments)-1].ID
	}
	found := false
	for _, comment := range thread.Comments {
		if comment.ID == replyTo {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("reply_to must reference a comment in thread %s", threadID)
	}
	commentID, err := s.newID("comment")
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Format(time.RFC3339)
	comment := artifact.Comment{ID: commentID, Author: strings.TrimSpace(input.Author), AuthorKind: input.AuthorKind, Body: strings.TrimSpace(input.Body), CreatedAt: now, ReplyTo: replyTo}
	thread.Comments = append(thread.Comments, comment)
	location.Discussion.UpdatedAt = now
	if err := s.Store.WriteDiscussion(location.Stored, location.Discussion); err != nil {
		return nil, err
	}
	return &comment, nil
}

// SetResolved changes one thread's review state without altering its messages.
func (s *DiscussionService) SetResolved(reviewID, threadID string, resolved bool) (*artifact.Thread, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("discussion service requires a store")
	}
	location, err := s.Store.FindThread(reviewID, threadID)
	if err != nil {
		return nil, err
	}
	thread := &location.Discussion.Threads[location.ThreadIndex]
	thread.Resolved = resolved
	location.Discussion.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.Store.WriteDiscussion(location.Stored, location.Discussion); err != nil {
		return nil, err
	}
	return thread, nil
}

// buildThreadTarget derives trusted commit identity and validates user-selected lines.
func buildThreadTarget(location *BlockLocation, input NewThread) (artifact.ThreadTarget, error) {
	target := artifact.ThreadTarget{Type: input.TargetType, BlockID: input.BlockID}
	if target.Type == "" || target.Type == "block" {
		target.Type = "block"
		return target, nil
	}
	if target.Type != "code" {
		return artifact.ThreadTarget{}, fmt.Errorf("target type must be block or code")
	}
	block := location.Block
	if block.Type != "code" && block.Type != "diff" {
		return artifact.ThreadTarget{}, fmt.Errorf("block %s does not contain commentable source lines", block.ID)
	}
	if input.Path != block.Path {
		return artifact.ThreadTarget{}, fmt.Errorf("comment path must match block %s", block.ID)
	}
	if input.Side != "base" && input.Side != "target" {
		return artifact.ThreadTarget{}, fmt.Errorf("comment side must be base or target")
	}
	if input.StartLine < 1 || input.EndLine < input.StartLine || input.EndLine-input.StartLine >= 500 {
		return artifact.ThreadTarget{}, fmt.Errorf("comment line range is invalid")
	}
	if block.Type == "code" && (input.Side != block.Source || input.StartLine < block.StartLine || input.EndLine > block.EndLine) {
		return artifact.ThreadTarget{}, fmt.Errorf("comment range must stay inside the code block")
	}
	commitSHA := location.Stored.Review.Source.TargetSHA
	if input.Side == "base" {
		commitSHA = location.Stored.Review.Source.BaseSHA
	}
	target.Path = input.Path
	target.Side = input.Side
	target.CommitSHA = commitSHA
	target.StartLine = input.StartLine
	target.EndLine = input.EndLine
	return target, nil
}

// validateCommentInput keeps persisted authorship and message content explicit.
func validateCommentInput(author, authorKind, body string) error {
	if strings.TrimSpace(author) == "" || strings.TrimSpace(body) == "" {
		return fmt.Errorf("author and body are required")
	}
	if authorKind != "human" && authorKind != "agent" {
		return fmt.Errorf("author kind must be human or agent")
	}
	return nil
}

// now returns an injectable UTC clock for deterministic artifacts and tests.
func (s *DiscussionService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// newID returns an injectable or cryptographically random artifact identifier.
func (s *DiscussionService) newID(prefix string) (string, error) {
	if s.NewID != nil {
		return s.NewID(prefix)
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate %s ID: %w", prefix, err)
	}
	return prefix + "-" + hex.EncodeToString(random[:]), nil
}
