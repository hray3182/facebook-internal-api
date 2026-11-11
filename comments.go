package fbia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"time"
)

const (
	commentsMaxAttempts        = 5
	commentsListFriendlyName   = "CommentsListComponentsPaginationQuery"
	commentsDialogFriendlyName = "CometSinglePostDialogContentQuery"
)

// ErrGraphQLResponse marks a Facebook GraphQL response that cannot be trusted as a complete result.
var ErrGraphQLResponse = errors.New("facebook graphql response error")

// ErrUnauthenticated marks a Facebook session that is missing, expired, or otherwise not logged in.
// Callers can errors.Is against this to prompt re-login. GraphQL code 1357001 ("Log in to continue")
// and HTTP 401/403 responses match this sentinel.
var ErrUnauthenticated = errors.New("facebook session unauthenticated or expired")

// facebookUnauthenticatedCode is returned by Facebook when the session requires login.
const facebookUnauthenticatedCode = 1357001

// GraphQLError contains structured metadata from Facebook's GraphQL / AJAX error payload.
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
	if target == ErrGraphQLResponse {
		return true
	}
	return target == ErrUnauthenticated && e.Code == facebookUnauthenticatedCode
}

// ListComments returns a paginated sequence of comments for a post.
//
// id may be either:
//   - FeedbackID("postID") (preferred for CommentsList; used by joaimy / fbGraber), or
//   - a plain Comet StoryID("S:_I{author}:VK:{post}") — converted to FeedbackID
//
// Uses CommentsListComponentsPaginationQuery (same approach as fbGraber).
func (c *Client) ListComments(ctx context.Context, id string) iter.Seq2[Comment, error] {
	return func(yield func(Comment, error) bool) {
		feedbackID, err := c.resolveFeedbackID(ctx, id)
		if err != nil {
			yield(Comment{}, err)
			return
		}

		var cursor string
		for {
			comments, nextCursor, err := c.fetchCommentsPageWithRetry(ctx, feedbackID, cursor)
			if err != nil {
				yield(Comment{}, err)
				return
			}
			for _, comment := range comments {
				if !yield(comment, nil) {
					return
				}
			}
			if nextCursor == "" {
				return
			}
			cursor = nextCursor
		}
	}
}

// ListCommentsForPost lists comments for a post id (author is not required for CommentsList).
func (c *Client) ListCommentsForPost(ctx context.Context, authorID, postID string) iter.Seq2[Comment, error] {
	_ = authorID
	return c.ListComments(ctx, FeedbackID(postID))
}

// FindGroupPost scans a group's feed for a post id and returns that Post
// (including StoryID). Useful when callers only have groupURL + postID.
func (c *Client) FindGroupPost(ctx context.Context, groupID, postID string) (*Post, error) {
	for post, err := range c.ListGroupPosts(ctx, groupID) {
		if err != nil {
			return nil, err
		}
		if post.PostID == postID {
			p := post
			return &p, nil
		}
	}
	return nil, fmt.Errorf("post %s not found in group %s feed", postID, groupID)
}

// FetchPostInfo extracts post metadata (story ID, first media ID).
// Prefers CommentsList parent_post_story; falls back to CommentsDialog when configured.
func (c *Client) FetchPostInfo(ctx context.Context, id string) (*PostInfo, error) {
	feedbackID, err := c.resolveFeedbackID(ctx, id)
	if err != nil {
		return nil, err
	}

	body, reqErr := c.doRequest(ctx, c.docIDs.Comments, commentsListVariables(feedbackID, ""), commentsListFriendlyName)
	if reqErr == nil {
		_, _, postInfo, parseErr := parseCommentsResponse(body)
		if parseErr == nil && postInfo != nil {
			return postInfo, nil
		}
		if parseErr != nil && !errors.Is(parseErr, ErrGraphQLResponse) {
			return nil, parseErr
		}
	} else if c.docIDs.CommentsDialog == "" {
		return nil, reqErr
	}

	if c.docIDs.CommentsDialog == "" {
		return nil, fmt.Errorf("post info not found in comments response")
	}

	storyID, err := c.resolveStoryIDForDialog(id)
	if err != nil {
		return nil, err
	}
	_, _, postInfo, err := c.fetchCommentsDialogWithRetry(ctx, storyID)
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

// postRef is what an id passed by a caller tells us about a post.
type postRef struct {
	feedbackID string // pfbid-based feedback id, usable as-is
	storyID    string // dialog story id, used to look up feedbackID
}

// parsePostRef accepts a pfbid FeedbackID, a numeric FeedbackID, a plain
// Comet StoryID, or a bare numeric post id.
func (c *Client) parsePostRef(id string) (postRef, error) {
	if id == "" {
		return postRef{}, fmt.Errorf("resolve post: empty id")
	}
	if inner, err := PostIDFromFeedback(id); err == nil {
		if !isAllDigits(inner) {
			return postRef{feedbackID: id}, nil
		}
		return c.postRefFromPostID(inner)
	}
	if IsStoryID(id) {
		return postRef{storyID: id}, nil
	}
	if isAllDigits(id) {
		return c.postRefFromPostID(id)
	}
	return postRef{}, fmt.Errorf("resolve post: unsupported id %q", id)
}

func (c *Client) postRefFromPostID(postID string) (postRef, error) {
	// Facebook resolves the story by post id; the author segment does not
	// have to match the real author, but it must be a non-empty user id.
	authorID := c.userID()
	if authorID == "" || authorID == "0" {
		return postRef{}, fmt.Errorf("resolve post: missing c_user cookie")
	}
	return postRef{storyID: StoryID(authorID, postID)}, nil
}

// resolveFeedbackID returns a feedback id Facebook accepts.
//
// Since 2026-11 Facebook only resolves pfbid-based feedback ids
// (base64 "feedback:pfbid…"); base64 "feedback:<numeric post id>" returns
// node:null for queries and "Incorrect Query" (1675002) for mutations.
// Numeric ids are looked up once through the post dialog and cached.
func (c *Client) resolveFeedbackID(ctx context.Context, id string) (string, error) {
	ref, err := c.parsePostRef(id)
	if err != nil {
		return "", err
	}
	if ref.feedbackID != "" {
		return ref.feedbackID, nil
	}
	if cached, ok := c.feedbackIDs.Load(ref.storyID); ok {
		return cached.(string), nil
	}
	status, err := c.fetchDialogCommentingStatus(ctx, ref.storyID, "")
	if err != nil {
		return "", fmt.Errorf("resolve feedback id: %w", err)
	}
	if _, err := PostIDFromFeedback(status.FeedbackID); err != nil {
		return "", fmt.Errorf("resolve feedback id: dialog returned %q: %w", status.FeedbackID, ErrGraphQLResponse)
	}
	c.feedbackIDs.Store(ref.storyID, status.FeedbackID)
	return status.FeedbackID, nil
}

// resolveStoryIDForDialog returns the story id for CometSinglePostDialogContentQuery.
// A pfbid FeedbackID cannot be turned back into a story id; pass a post id or StoryID.
func (c *Client) resolveStoryIDForDialog(id string) (string, error) {
	ref, err := c.parsePostRef(id)
	if err != nil {
		return "", err
	}
	if ref.storyID == "" {
		return "", fmt.Errorf("resolve story id: %q is a pfbid feedback id; pass the post id or StoryID", id)
	}
	return ref.storyID, nil
}

func commentsListVariables(feedbackID, cursor string) map[string]any {
	// Variables aligned with fbGraber FetchComments.
	return map[string]any{
		"commentsAfterCount":   -1,
		"commentsAfterCursor":  nilIfEmpty(cursor),
		"commentsBeforeCount":  nil,
		"commentsBeforeCursor": nil,
		"commentsIntentToken":  nil,
		"feedLocation":         "POST_PERMALINK_DIALOG",
		"focusCommentID":       nil,
		"scale":                1,
		"targetDialect":        nil,
		"useDefaultActor":      false,
		"id":                   feedbackID,
		"__relay_internal__pv__CometUFICommentAutoTranslationTyperelayprovider":        "AUTO_TRANSLATE",
		"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider": false,
		"__relay_internal__pv__CometUFICommentActionLinksRewriteEnabledrelayprovider":  true,
		"__relay_internal__pv__IsWorkUserrelayprovider":                                false,
	}
}

func (c *Client) fetchCommentsPageWithRetry(ctx context.Context, feedbackID, cursor string) ([]Comment, string, error) {
	var lastErr error
	for attempt := 1; attempt <= commentsMaxAttempts; attempt++ {
		comments, next, err := c.fetchCommentsPage(ctx, feedbackID, cursor)
		if err == nil {
			return comments, next, nil
		}
		if errors.Is(err, ErrUnauthenticated) || !errors.Is(err, ErrGraphQLResponse) {
			return nil, "", err
		}
		lastErr = err
		if attempt == commentsMaxAttempts {
			break
		}
		if err := c.waitBeforeCommentsRetry(ctx, attempt, lastErr); err != nil {
			return nil, "", fmt.Errorf("wait before retry comments page: %w", err)
		}
	}
	return nil, "", fmt.Errorf("fetch comments page after %d attempts: %w", commentsMaxAttempts, lastErr)
}

func (c *Client) fetchCommentsPage(ctx context.Context, feedbackID, cursor string) ([]Comment, string, error) {
	body, err := c.doRequest(ctx, c.docIDs.Comments, commentsListVariables(feedbackID, cursor), commentsListFriendlyName)
	if err != nil {
		return nil, "", fmt.Errorf("comments page: %w", err)
	}
	comments, next, _, err := parseCommentsResponse(body)
	if err != nil {
		return nil, "", fmt.Errorf("parse comments page: %w", err)
	}
	return comments, next, nil
}

func (c *Client) fetchCommentsDialogWithRetry(ctx context.Context, storyID string) ([]Comment, string, *PostInfo, error) {
	var lastErr error
	for attempt := 1; attempt <= commentsMaxAttempts; attempt++ {
		comments, cursor, postInfo, err := c.fetchCommentsDialog(ctx, storyID)
		if err == nil {
			return comments, cursor, postInfo, nil
		}
		if errors.Is(err, ErrUnauthenticated) || !errors.Is(err, ErrGraphQLResponse) {
			return nil, "", nil, err
		}
		lastErr = err
		if attempt == commentsMaxAttempts {
			break
		}
		if err := c.waitBeforeCommentsRetry(ctx, attempt, lastErr); err != nil {
			return nil, "", nil, fmt.Errorf("wait before retry comments dialog: %w", err)
		}
	}
	return nil, "", nil, fmt.Errorf("fetch comments dialog after %d attempts: %w", commentsMaxAttempts, lastErr)
}

func (c *Client) fetchCommentsDialog(ctx context.Context, storyID string) ([]Comment, string, *PostInfo, error) {
	body, err := c.doRequest(ctx, c.docIDs.CommentsDialog, commentsDialogVariables(storyID), commentsDialogFriendlyName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("comments dialog: %w", err)
	}
	comments, cursor, postInfo, err := parseCommentsResponse(body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse comments dialog: %w", err)
	}
	return comments, cursor, postInfo, nil
}

func commentsDialogVariables(storyID string) map[string]any {
	return withRelayProviders(map[string]any{
		"feedbackSource":                2,
		"feedLocation":                  "POST_PERMALINK_DIALOG",
		"focusCommentID":                nil,
		"privacySelectorRenderLocation": "COMET_STREAM",
		"renderLocation":                "permalink",
		"scale":                         1,
		"shouldChangeNodeFieldName":     true,
		"storyID":                       storyID,
		"useDefaultActor":               false,
	}, postDialogRelayProviders)
}

func (c *Client) waitBeforeCommentsRetry(ctx context.Context, attempt int, lastErr error) error {
	if c.retryDelay <= 0 {
		return nil
	}

	delay := time.Duration(attempt) * c.retryDelay
	var ge *GraphQLError
	if errors.As(lastErr, &ge) && ge.Code == 1357004 {
		delay = time.Duration(attempt) * c.retryDelay * 4
	}

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

		comments = append(comments, commentFromNode(node))
	}

	pageInfo := jsonMap(commentsBlock, "page_info")
	nextCursor := jsonStr(pageInfo, "end_cursor")
	// Match fbGraber: stop when end_cursor is empty. Only clear when has_next_page is explicitly false.
	if hasNext, ok := pageInfo["has_next_page"].(bool); ok && !hasNext {
		nextCursor = ""
	}
	return comments, nextCursor, postInfo, nil
}

func commentFromNode(node map[string]any) Comment {
	fb := jsonMap(node, "feedback")
	bodyMap := jsonMap(node, "body")
	reactors := jsonMap(fb, "reactors")
	expInfo := jsonMap(fb, "expansion_info")
	author := jsonMap(node, "author")
	commentID := jsonStr(node, "id")
	if commentID == "" {
		commentID = jsonStr(node, "legacy_fbid")
	}
	raw, _ := json.Marshal(node)
	return Comment{
		CommentID:      commentID,
		LegacyFbid:     jsonStr(node, "legacy_fbid"),
		AuthorID:       jsonStr(author, "id"),
		AuthorName:     jsonStr(author, "name"),
		AuthorURL:      jsonStr(author, "url"),
		Text:           jsonStr(bodyMap, "text"),
		CreatedTime:    jsonInt64(node, "created_time"),
		ReactionCount:  jsonStr(reactors, "count_reduced"),
		FeedbackID:     jsonStr(fb, "id"),
		ExpansionToken: jsonStr(expInfo, "expansion_token"),
		Node:           raw,
	}
}

func findCommentsBlock(parsed map[string]any) map[string]any {
	// CommentsListComponentsPaginationQuery shape.
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
	if graphQLErr := firstResponseError(parsed); graphQLErr != nil {
		return graphQLErr
	}
	return fmt.Errorf("comments block missing: %w", ErrGraphQLResponse)
}

func firstResponseError(parsed map[string]any) *GraphQLError {
	if graphQLErr := firstGraphQLError(parsed); graphQLErr != nil {
		return graphQLErr
	}
	if code := int(jsonFloat(parsed, "error")); code > 0 {
		summary := jsonStr(parsed, "errorSummary")
		desc := jsonStr(parsed, "errorDescription")
		return &GraphQLError{
			Message:     summary,
			Severity:    "CRITICAL",
			Summary:     summary,
			Description: desc,
			Code:        code,
		}
	}
	return nil
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
		if node == nil {
			continue
		}
		replies = append(replies, replyFromNode(node))
	}
	return replies, nil
}

func replyFromNode(node map[string]any) Reply {
	fb := jsonMap(node, "feedback")
	bodyMap := jsonMap(node, "body")
	reactors := jsonMap(fb, "reactors")
	author := jsonMap(node, "author")
	commentID := jsonStr(node, "id")
	if commentID == "" {
		commentID = jsonStr(node, "legacy_fbid")
	}
	raw, _ := json.Marshal(node)
	return Reply{
		CommentID:     commentID,
		LegacyFbid:    jsonStr(node, "legacy_fbid"),
		AuthorID:      jsonStr(author, "id"),
		AuthorName:    jsonStr(author, "name"),
		AuthorURL:     jsonStr(author, "url"),
		Text:          jsonStr(bodyMap, "text"),
		CreatedTime:   jsonInt64(node, "created_time"),
		ReactionCount: jsonStr(reactors, "count_reduced"),
		Node:          raw,
	}
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

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
