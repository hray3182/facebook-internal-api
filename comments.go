package fbia

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"
)

const (
	commentsMaxAttempts        = 3
	commentsDialogFriendlyName = "CometSinglePostDialogContentQuery"
)

// ErrGraphQLResponse marks a Facebook GraphQL response that cannot be trusted as a complete result.
var ErrGraphQLResponse = errors.New("facebook graphql response error")

// GraphQLError contains structured metadata from Facebook's GraphQL error payload.
type GraphQLError struct {
	Message     string
	Severity    string
	Summary     string
	Description string
	Code        int
}

func (e *GraphQLError) Error() string {
	if e.Summary != "" {
		return fmt.Sprintf("facebook graphql %s error %d: %s", e.Severity, e.Code, e.Summary)
	}
	return fmt.Sprintf("facebook graphql %s error %d: %s", e.Severity, e.Code, e.Message)
}

func (e *GraphQLError) Is(target error) bool {
	return target == ErrGraphQLResponse
}

// ListComments returns comments for a post.
//
// id may be either:
//   - FeedbackID("postID") (legacy / joaimy callers), or
//   - a Comet story ID (base64 of "S:_I{author}:VK:{postID}"), preferred
//
// Prefer Post.StoryID from ListGroupPosts / ListPosts when available. When a
// FeedbackID is given, the story ID is synthesized with the logged-in c_user as
// author, which works for posts authored by that user.
func (c *Client) ListComments(ctx context.Context, id string) iter.Seq2[Comment, error] {
	return func(yield func(Comment, error) bool) {
		storyID, err := c.resolveStoryID(id)
		if err != nil {
			yield(Comment{}, err)
			return
		}

		comments, _, err := c.fetchCommentsDialogWithRetry(ctx, storyID)
		if err != nil {
			yield(Comment{}, err)
			return
		}
		for _, comment := range comments {
			if !yield(comment, nil) {
				return
			}
		}
	}
}

// FetchPostInfo extracts post metadata (story ID, first media ID) from the permalink dialog query.
// Useful for obtaining the startNodeID needed by ListImages.
//
// id may be a FeedbackID or a Comet story ID (see ListComments).
func (c *Client) FetchPostInfo(ctx context.Context, id string) (*PostInfo, error) {
	storyID, err := c.resolveStoryID(id)
	if err != nil {
		return nil, err
	}
	_, postInfo, err := c.fetchCommentsDialogWithRetry(ctx, storyID)
	return postInfo, err
}

// FetchReplies fetches all replies for the given comment.
func (c *Client) FetchReplies(ctx context.Context, comment Comment) ([]Reply, error) {
	variables := map[string]any{
		"clientKey":       nil,
		"expansionToken":  comment.ExpansionToken,
		"feedLocation":    "POST_PERMALINK_DIALOG",
		"focusCommentID":  nil,
		"scale":           2,
		"useDefaultActor": false,
		"id":              comment.FeedbackID,
	}

	body, err := c.doRequest(ctx, c.docIDs.Replies, variables, "Depth1CommentsListPaginationQuery")
	if err != nil {
		return nil, fmt.Errorf("fetch replies: %w", err)
	}

	return parseRepliesResponse(body)
}

func (c *Client) resolveStoryID(id string) (string, error) {
	if strings.HasPrefix(id, "Uzpf") {
		return id, nil
	}
	postID, err := PostIDFromFeedback(id)
	if err != nil {
		if id != "" {
			return id, nil
		}
		return "", fmt.Errorf("resolve story id: %w", err)
	}
	authorID := c.userID()
	if authorID == "" || authorID == "0" {
		return "", fmt.Errorf("resolve story id: missing c_user cookie for author")
	}
	return StoryID(authorID, postID), nil
}

func (c *Client) fetchCommentsDialogWithRetry(ctx context.Context, storyID string) ([]Comment, *PostInfo, error) {
	var lastErr error
	for attempt := 1; attempt <= commentsMaxAttempts; attempt++ {
		comments, postInfo, err := c.fetchCommentsDialog(ctx, storyID)
		if err == nil {
			return comments, postInfo, nil
		}
		if !errors.Is(err, ErrGraphQLResponse) {
			return nil, nil, err
		}
		lastErr = err
		if attempt == commentsMaxAttempts {
			break
		}
		if err := c.waitBeforeCommentsRetry(ctx, attempt); err != nil {
			return nil, nil, fmt.Errorf("wait before retry comments: %w", err)
		}
	}
	return nil, nil, fmt.Errorf("fetch comments dialog after %d attempts: %w", commentsMaxAttempts, lastErr)
}

func (c *Client) fetchCommentsDialog(ctx context.Context, storyID string) ([]Comment, *PostInfo, error) {
	variables := map[string]any{
		"feedbackSource":                2,
		"feedLocation":                  "POST_PERMALINK_DIALOG",
		"focusCommentID":                nil,
		"privacySelectorRenderLocation": "COMET_STREAM",
		"renderLocation":                "permalink",
		"scale":                         1,
		"shouldChangeNodeFieldName":     true,
		"storyID":                       storyID,
		"useDefaultActor":               false,
	}

	body, err := c.doRequest(ctx, c.docIDs.Comments, variables, commentsDialogFriendlyName)
	if err != nil {
		return nil, nil, fmt.Errorf("comments dialog: %w", err)
	}
	comments, _, postInfo, err := parseCommentsResponse(body)
	if err != nil {
		return nil, nil, fmt.Errorf("parse comments dialog: %w", err)
	}
	return comments, postInfo, nil
}

func (c *Client) waitBeforeCommentsRetry(ctx context.Context, attempt int) error {
	if c.retryDelay <= 0 {
		return nil
	}

	delay := time.Duration(attempt) * c.retryDelay
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseCommentsResponse(body string) ([]Comment, string, *PostInfo, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse comments json: %w", err)
	}

	commentsBlock := findCommentsBlock(parsed)
	if commentsBlock == nil {
		return nil, "", nil, commentsResponseError(parsed)
	}

	var postInfo *PostInfo
	if node := jsonNav(parsed, "data", "node_v2"); node != nil {
		postInfo = extractDialogPostInfo(node)
	}

	var comments []Comment
	for _, e := range jsonSlice(commentsBlock, "edges") {
		edge, _ := e.(map[string]any)
		node := jsonMap(edge, "node")
		fb := jsonMap(node, "feedback")
		if node == nil || fb == nil {
			continue
		}

		if postInfo == nil {
			postInfo = extractPostInfo(node)
		}

		bodyMap := jsonMap(node, "body")
		expInfo := jsonMap(fb, "expansion_info")
		reactors := jsonMap(fb, "reactors")
		author := jsonMap(node, "author")

		commentID := jsonStr(node, "id")
		if commentID == "" {
			commentID = jsonStr(node, "legacy_fbid")
		}

		comments = append(comments, Comment{
			CommentID:      commentID,
			AuthorID:       jsonStr(author, "id"),
			AuthorName:     jsonStr(author, "name"),
			Text:           jsonStr(bodyMap, "text"),
			ReactionCount:  jsonStr(reactors, "count_reduced"),
			FeedbackID:     jsonStr(fb, "id"),
			ExpansionToken: jsonStr(expInfo, "expansion_token"),
		})
	}

	pageInfo := jsonMap(commentsBlock, "page_info")
	nextCursor := jsonStr(pageInfo, "end_cursor")
	if pageInfo["has_next_page"] != true {
		nextCursor = ""
	}
	return comments, nextCursor, postInfo, nil
}

func findCommentsBlock(parsed map[string]any) map[string]any {
	// Legacy CommentsListComponentsPaginationQuery shape.
	if block := jsonNav(parsed,
		"data", "node", "comment_rendering_instance_for_feed_location", "comments"); block != nil {
		return block
	}
	// CometSinglePostDialogContentQuery shape.
	return jsonNav(parsed,
		"data", "node_v2", "comet_sections", "feedback", "story", "story_ufi_container", "story",
		"feedback_context", "feedback_target_with_context", "comment_list_renderer", "feedback",
		"comment_rendering_instance_for_feed_location", "comments")
}

func extractDialogPostInfo(node map[string]any) *PostInfo {
	info := &PostInfo{
		StoryID: jsonStr(node, "id"),
	}
	for _, raw := range jsonSlice(node, "attachments") {
		att, _ := raw.(map[string]any)
		media := jsonMap(jsonMap(att, "styles"), "attachment")
		if media == nil {
			media = jsonMap(att, "media")
		} else {
			media = jsonMap(media, "media")
		}
		if id := jsonStr(media, "id"); id != "" {
			info.MediaID = id
			break
		}
	}
	if info.StoryID == "" && info.MediaID == "" {
		return nil
	}
	return info
}

func commentsResponseError(parsed map[string]any) error {
	if graphQLErr := firstGraphQLError(parsed); graphQLErr != nil {
		return graphQLErr
	}
	return fmt.Errorf("comments block missing: %w", ErrGraphQLResponse)
}

func firstGraphQLError(parsed map[string]any) *GraphQLError {
	for _, raw := range jsonSlice(parsed, "errors") {
		errMap, _ := raw.(map[string]any)
		if errMap == nil {
			continue
		}
		return &GraphQLError{
			Message:     jsonStr(errMap, "message"),
			Severity:    jsonStr(errMap, "severity"),
			Summary:     jsonStr(errMap, "summary"),
			Description: jsonStr(errMap, "description"),
			Code:        int(jsonFloat(errMap, "code")),
		}
	}
	return nil
}

func parseRepliesResponse(body string) ([]Reply, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, fmt.Errorf("parse replies json: %w", err)
	}

	repliesConn := jsonNav(parsed, "data", "node", "replies_connection")
	if repliesConn == nil {
		return nil, nil
	}

	var replies []Reply
	for _, e := range jsonSlice(repliesConn, "edges") {
		edge, _ := e.(map[string]any)
		node := jsonMap(edge, "node")
		fb := jsonMap(node, "feedback")

		bodyMap := jsonMap(node, "body")
		reactors := jsonMap(fb, "reactors")
		author := jsonMap(node, "author")

		replies = append(replies, Reply{
			AuthorID:      jsonStr(author, "id"),
			AuthorName:    jsonStr(author, "name"),
			Text:          jsonStr(bodyMap, "text"),
			ReactionCount: jsonStr(reactors, "count_reduced"),
		})
	}
	return replies, nil
}

func extractPostInfo(commentNode map[string]any) *PostInfo {
	pps := jsonMap(commentNode, "parent_post_story")
	if pps == nil {
		return nil
	}

	info := &PostInfo{
		StoryID: jsonStr(pps, "id"),
	}

	for _, raw := range jsonSlice(pps, "attachments") {
		att, _ := raw.(map[string]any)
		m := jsonMap(att, "media")
		if id := jsonStr(m, "id"); id != "" {
			info.MediaID = id
			break
		}
	}
	return info
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
