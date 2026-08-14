package fbia

import (
	"context"
	"fmt"
	"time"
)

const (
	disableCommentingFriendlyName = "useCometFeedStoryDisableCommentingMutation"
	enableCommentingFriendlyName  = "useCometFeedStoryEnableCommentingMutation"
)

// DisableCommenting turns off commenting on a post.
//
// id may be a FeedbackID, a bare post id, or a plain Comet StoryID.
func (c *Client) DisableCommenting(ctx context.Context, id string) (*CommentingStatus, error) {
	return c.setCommentingDisabled(ctx, id, true)
}

// EnableCommenting turns on commenting on a post.
//
// id may be a FeedbackID, a bare post id, or a plain Comet StoryID.
func (c *Client) EnableCommenting(ctx context.Context, id string) (*CommentingStatus, error) {
	return c.setCommentingDisabled(ctx, id, false)
}

// FetchCommentingStatus reads whether commenting is disabled on a post.
//
// Uses CometSinglePostDialogContentQuery (CommentsList omits the post-level
// flag). Group feed payloads also expose this as Post.CommentsDisabled.
func (c *Client) FetchCommentingStatus(ctx context.Context, id string) (*CommentingStatus, error) {
	feedbackID, err := c.resolveFeedbackID(id)
	if err != nil {
		return nil, err
	}

	storyID, err := c.resolveStoryIDForDialog(id, feedbackID)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, c.docIDs.CommentsDialog, commentsDialogVariables(storyID), commentsDialogFriendlyName)
	if err != nil {
		return nil, fmt.Errorf("commenting status: %w", err)
	}

	status, err := parseCommentingStatusResponse(body, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("parse commenting status: %w", err)
	}
	return status, nil
}

func (c *Client) setCommentingDisabled(ctx context.Context, id string, disabled bool) (*CommentingStatus, error) {
	feedbackID, err := c.resolveFeedbackID(id)
	if err != nil {
		return nil, err
	}

	docID := c.docIDs.EnableCommenting
	friendly := enableCommentingFriendlyName
	dataKey := "feedback_enable_commenting"
	if disabled {
		docID = c.docIDs.DisableCommenting
		friendly = disableCommentingFriendlyName
		dataKey = "feedback_disable_commenting"
	}

	body, err := c.doRequest(ctx, docID, c.commentingToggleVariables(feedbackID), friendly)
	if err != nil {
		return nil, fmt.Errorf("set commenting disabled=%t: %w", disabled, err)
	}

	status, err := parseToggleCommentingResponse(body, dataKey)
	if err != nil {
		return nil, fmt.Errorf("parse commenting toggle: %w", err)
	}
	return status, nil
}

func (c *Client) commentingToggleVariables(feedbackID string) map[string]any {
	return map[string]any{
		"input": map[string]any{
			"feedback_id":        feedbackID,
			"actor_id":           c.userID(),
			"client_mutation_id": fmt.Sprintf("%d", time.Now().UnixNano()),
		},
		"scale": 1,
	}
}

func parseToggleCommentingResponse(body, dataKey string) (*CommentingStatus, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, fmt.Errorf("parse commenting toggle json: %w", err)
	}

	feedback := jsonNav(parsed, "data", dataKey, "feedback")
	if feedback == nil {
		return nil, mutationResponseError(parsed, dataKey)
	}

	disabled, ok := jsonBool(feedback, "have_comments_been_disabled")
	if !ok {
		return nil, fmt.Errorf("have_comments_been_disabled missing: %w", ErrGraphQLResponse)
	}

	return &CommentingStatus{
		FeedbackID:       jsonStr(feedback, "id"),
		CommentsDisabled: disabled,
	}, nil
}

func parseCommentingStatusResponse(body, fallbackFeedbackID string) (*CommentingStatus, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, fmt.Errorf("parse commenting status json: %w", err)
	}

	node := jsonNav(parsed, "data", "node")
	candidates := []map[string]any{
		jsonNav(parsed,
			"data", "node_v2", "comet_sections", "feedback", "story", "story_ufi_container", "story",
			"feedback_context", "feedback_target_with_context", "comment_list_renderer", "feedback"),
		jsonNav(parsed,
			"data", "node_v2", "comet_sections", "feedback", "story", "story_ufi_container", "story",
			"feedback_context", "feedback_target_with_context"),
		node,
	}
	for _, m := range candidates {
		if disabled, ok := jsonBool(m, "have_comments_been_disabled"); ok {
			id := jsonStr(m, "id")
			if id == "" {
				id = fallbackFeedbackID
			}
			return &CommentingStatus{
				FeedbackID:       id,
				CommentsDisabled: disabled,
			}, nil
		}
	}

	if graphQLErr := firstResponseError(parsed); graphQLErr != nil {
		return nil, graphQLErr
	}
	return nil, fmt.Errorf("have_comments_been_disabled missing: %w", ErrGraphQLResponse)
}
