package fbia

import (
	"context"
	"fmt"
	"iter"
)

// ListComments returns a sequence of comments for a post.
// feedbackID is the base64-encoded feedback ID (use FeedbackID to convert from a post ID).
func (c *Client) ListComments(ctx context.Context, feedbackID string) iter.Seq2[Comment, error] {
	return func(yield func(Comment, error) bool) {
		var cursor string

		for {
			variables := map[string]any{
				"commentsAfterCount":  -1,
				"commentsAfterCursor": nilIfEmpty(cursor),
				"commentsIntentToken": "REVERSE_CHRONOLOGICAL_UNFILTERED_INTENT_V1",
				"feedLocation":        "DEDICATED_COMMENTING_SURFACE",
				"focusCommentID":      nil,
				"scale":               2,
				"useDefaultActor":     false,
				"id":                  feedbackID,
			}

			body, err := c.doRequest(ctx, c.docIDs.Comments, variables, "CommentsListComponentsPaginationQuery")
			if err != nil {
				yield(Comment{}, fmt.Errorf("list comments: %w", err))
				return
			}

			comments, nextCursor, _, err := parseCommentsResponse(body)
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

// FetchPostInfo extracts post metadata (story ID, first media ID) from the comment API.
// Useful for obtaining the startNodeID needed by ListImages.
func (c *Client) FetchPostInfo(ctx context.Context, feedbackID string) (*PostInfo, error) {
	variables := map[string]any{
		"commentsAfterCount":  -1,
		"commentsAfterCursor": nil,
		"commentsIntentToken": "REVERSE_CHRONOLOGICAL_UNFILTERED_INTENT_V1",
		"feedLocation":        "DEDICATED_COMMENTING_SURFACE",
		"focusCommentID":      nil,
		"scale":               2,
		"useDefaultActor":     false,
		"id":                  feedbackID,
	}

	body, err := c.doRequest(ctx, c.docIDs.Comments, variables, "CommentsListComponentsPaginationQuery")
	if err != nil {
		return nil, fmt.Errorf("fetch post info: %w", err)
	}

	_, _, postInfo, err := parseCommentsResponse(body)
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

func parseCommentsResponse(body string) ([]Comment, string, *PostInfo, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse comments json: %w", err)
	}

	commentsBlock := jsonNav(parsed,
		"data", "node", "comment_rendering_instance_for_feed_location", "comments")
	if commentsBlock == nil {
		return nil, "", nil, nil
	}

	var postInfo *PostInfo
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

		comments = append(comments, Comment{
			CommentID:      jsonStr(node, "id"),
			AuthorID:       jsonStr(author, "id"),
			AuthorName:     jsonStr(author, "name"),
			Text:           jsonStr(bodyMap, "text"),
			ReactionCount:  jsonStr(reactors, "count_reduced"),
			FeedbackID:     jsonStr(fb, "id"),
			ExpansionToken: jsonStr(expInfo, "expansion_token"),
		})
	}

	nextCursor := jsonStr(jsonMap(commentsBlock, "page_info"), "end_cursor")
	return comments, nextCursor, postInfo, nil
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
