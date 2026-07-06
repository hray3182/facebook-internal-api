package fbia

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	createCommentFriendlyName = "useCometUFICreateCommentMutation"
	deleteCommentFriendlyName = "useCometUFIDeleteCommentMutation"
	defaultCommentFeedSource  = "PROFILE"
	defaultCommentLocation    = "MENU"
	defaultFeedLocation       = "GROUP"
)

var ErrInvalidCommentMutationInput = errors.New("invalid comment mutation input")

type CreateCommentInput struct {
	FeedbackID       string
	Text             string
	GroupID          string
	FeedLocation     string
	FeedbackSource   string
	FeedbackReferrer string
	AttributionID    string
	Tracking         []string
}

type DeleteCommentInput struct {
	CommentID      string
	GroupID        string
	RemoveLocation string
	AttributionID  string
	Tracking       []string
}

type DeleteCommentResult struct {
	DeletedCommentID string
	FeedbackID       string
	GroupID          string
}

func (c *Client) CreateComment(ctx context.Context, input CreateCommentInput) (*Comment, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}

	variables, err := c.createCommentVariables(input)
	if err != nil {
		return nil, fmt.Errorf("build create comment variables: %w", err)
	}

	body, err := c.doRequest(ctx, c.docIDs.CreateComment, variables, createCommentFriendlyName)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	comment, err := parseCreateCommentResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse create comment response: %w", err)
	}
	return comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, input DeleteCommentInput) (*DeleteCommentResult, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, c.docIDs.DeleteComment, c.deleteCommentVariables(input), deleteCommentFriendlyName)
	if err != nil {
		return nil, fmt.Errorf("delete comment: %w", err)
	}

	result, err := parseDeleteCommentResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse delete comment response: %w", err)
	}
	return result, nil
}

func (input CreateCommentInput) validate() error {
	if strings.TrimSpace(input.FeedbackID) == "" {
		return fmt.Errorf("feedback id is required: %w", ErrInvalidCommentMutationInput)
	}
	if strings.TrimSpace(input.Text) == "" {
		return fmt.Errorf("comment text is required: %w", ErrInvalidCommentMutationInput)
	}
	return nil
}

func (input DeleteCommentInput) validate() error {
	if strings.TrimSpace(input.CommentID) == "" {
		return fmt.Errorf("comment id is required: %w", ErrInvalidCommentMutationInput)
	}
	return nil
}

func (c *Client) createCommentVariables(input CreateCommentInput) (map[string]any, error) {
	sessionID, err := randomUUID()
	if err != nil {
		return nil, err
	}
	idempotenceID, err := randomUUID()
	if err != nil {
		return nil, err
	}

	feedLocation := input.FeedLocation
	if feedLocation == "" {
		feedLocation = defaultFeedLocation
	}
	feedbackSource := input.FeedbackSource
	if feedbackSource == "" {
		feedbackSource = defaultCommentFeedSource
	}
	feedbackReferrer := input.FeedbackReferrer
	if feedbackReferrer == "" {
		feedbackReferrer = "/"
	}

	return map[string]any{
		"feedLocation":   feedLocation,
		"feedbackSource": 0,
		"groupID":        nilIfEmpty(input.GroupID),
		"input": map[string]any{
			"actor_id":              c.userID(),
			"client_mutation_id":    fmt.Sprintf("%d", time.Now().UnixNano()),
			"attachments":           nil,
			"feedback_id":           input.FeedbackID,
			"formatting_style":      nil,
			"message":               map[string]any{"ranges": []map[string]any{}, "text": input.Text},
			"reply_target_clicked":  false,
			"attribution_id_v2":     nilIfEmpty(input.AttributionID),
			"vod_video_timestamp":   nil,
			"feedback_referrer":     feedbackReferrer,
			"is_tracking_encrypted": len(input.Tracking) > 0,
			"tracking":              trackingOrEmpty(input.Tracking),
			"feedback_source":       feedbackSource,
			"idempotence_token":     "client:" + idempotenceID,
			"session_id":            sessionID,
		},
		"inviteShortLinkKey":    nil,
		"renderLocation":        nil,
		"scale":                 1,
		"useDefaultActor":       false,
		"focusCommentID":        nil,
		"translationType":       "AUTO_TRANSLATE",
		"canUseNicknameOnComet": false,
		"__relay_internal__pv__groups_comet_use_glvrelayprovider":                      false,
		"__relay_internal__pv__CometUFICommentActionLinksRewriteEnabledrelayprovider":  false,
		"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider": false,
		"__relay_internal__pv__IsWorkUserrelayprovider":                                false,
		"__relay_internal__pv__CometUFICommentAutoTranslationTyperelayprovider":        "AUTO_TRANSLATE",
	}, nil
}

func (c *Client) deleteCommentVariables(input DeleteCommentInput) map[string]any {
	removeLocation := input.RemoveLocation
	if removeLocation == "" {
		removeLocation = defaultCommentLocation
	}

	return map[string]any{
		"groupID": nilIfEmpty(input.GroupID),
		"input": map[string]any{
			"actor_id":           c.userID(),
			"client_mutation_id": fmt.Sprintf("%d", time.Now().UnixNano()),
			"attribution_id_v2":  nilIfEmpty(input.AttributionID),
			"comment_id":         input.CommentID,
			"remove_location":    removeLocation,
			"tracking":           trackingOrEmpty(input.Tracking),
		},
		"inviteShortLinkKey":    nil,
		"renderLocation":        nil,
		"scale":                 1,
		"canUseNicknameOnComet": false,
		"__relay_internal__pv__groups_comet_use_glvrelayprovider": false,
	}
}

func parseCreateCommentResponse(body string) (*Comment, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, fmt.Errorf("parse create comment json: %w", err)
	}

	node := jsonNav(parsed, "data", "comment_create", "feedback_comment_edge", "node")
	if node == nil {
		return nil, mutationResponseError(parsed, "comment_create")
	}

	comment := commentFromNode(node)
	return &comment, nil
}

func parseDeleteCommentResponse(body string) (*DeleteCommentResult, error) {
	parsed, err := parseFBJSON(body)
	if err != nil {
		return nil, fmt.Errorf("parse delete comment json: %w", err)
	}

	deleted := jsonNav(parsed, "data", "comment_delete")
	if deleted == nil {
		return nil, mutationResponseError(parsed, "comment_delete")
	}

	feedback := jsonMap(deleted, "feedback")
	group := jsonMap(feedback, "associated_group")
	return &DeleteCommentResult{
		DeletedCommentID: jsonStr(deleted, "deleted_comment_id"),
		FeedbackID:       jsonStr(feedback, "id"),
		GroupID:          jsonStr(group, "id"),
	}, nil
}

func commentFromNode(node map[string]any) Comment {
	fb := jsonMap(node, "feedback")
	bodyMap := jsonMap(node, "body")
	reactors := jsonMap(fb, "reactors")
	expInfo := jsonMap(fb, "expansion_info")
	author := jsonMap(node, "author")

	return Comment{
		CommentID:      jsonStr(node, "id"),
		AuthorID:       jsonStr(author, "id"),
		AuthorName:     jsonStr(author, "name"),
		Text:           jsonStr(bodyMap, "text"),
		ReactionCount:  jsonStr(reactors, "count_reduced"),
		FeedbackID:     jsonStr(fb, "id"),
		ExpansionToken: jsonStr(expInfo, "expansion_token"),
	}
}

func mutationResponseError(parsed map[string]any, key string) error {
	if graphQLErr := firstGraphQLError(parsed); graphQLErr != nil {
		return graphQLErr
	}
	return fmt.Errorf("%s block missing: %w", key, ErrGraphQLResponse)
}

func trackingOrEmpty(tracking []string) []string {
	if tracking == nil {
		return []string{}
	}
	return tracking
}

func randomUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
