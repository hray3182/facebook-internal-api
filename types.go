package fbia

import "encoding/json"

// Post represents a Facebook post from a page timeline or group feed.
type Post struct {
	PostID       string `json:"post_id"`
	StoryID      string `json:"story_id,omitempty"`
	FeedbackID   string `json:"feedback_id,omitempty"`
	Text         string `json:"text,omitempty"`
	Permalink    string `json:"permalink,omitempty"`
	CommentCount int    `json:"comment_count"`
	// CommentsDisabled is set when the feed payload includes
	// have_comments_been_disabled (group UFI). Nil means the field was absent.
	CommentsDisabled *bool   `json:"comments_disabled,omitempty"`
	PageName         string  `json:"page_name,omitempty"`
	GroupName        string  `json:"group_name,omitempty"`
	Media            []Media `json:"media,omitempty"`
	VideoOrReel      bool    `json:"video_or_reel,omitempty"`
}

// CommentingStatus is the post-level commenting switch (have_comments_been_disabled).
type CommentingStatus struct {
	FeedbackID       string `json:"feedback_id"`
	CommentsDisabled bool   `json:"comments_disabled"`
}

// Comment is a top-level comment. Typed fields are convenience accessors;
// Node is the unmodified GraphQL comment object so callers can read any field.
type Comment struct {
	CommentID  string `json:"comment_id,omitempty"`
	LegacyFbid string `json:"legacy_fbid,omitempty"`
	AuthorID   string `json:"author_id,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	AuthorURL  string `json:"author_url,omitempty"`

	Text          string `json:"text"`
	CreatedTime   int64  `json:"created_time,omitempty"` // unix seconds
	ReactionCount string `json:"reaction_count"`

	// FeedbackID and ExpansionToken are needed to fetch replies via Client.FetchReplies.
	FeedbackID     string `json:"feedback_id"`
	ExpansionToken string `json:"expansion_token"`

	Node json.RawMessage `json:"node,omitempty"`
}

// Reply is a reply to a comment. Node is the unmodified GraphQL reply object.
type Reply struct {
	CommentID  string `json:"comment_id,omitempty"`
	LegacyFbid string `json:"legacy_fbid,omitempty"`
	AuthorID   string `json:"author_id,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	AuthorURL  string `json:"author_url,omitempty"`

	Text          string `json:"text"`
	CreatedTime   int64  `json:"created_time,omitempty"`
	ReactionCount string `json:"reaction_count"`

	Node json.RawMessage `json:"node,omitempty"`
}

// Media represents a photo or video attachment on a post.
type Media struct {
	Type string `json:"type"` // "photo" or "video"
	URL  string `json:"url"`
	ID   string `json:"id,omitempty"`
}

// Image represents a single image fetched from a post's media set.
type Image struct {
	NodeID string `json:"node_id"`
	URL    string `json:"url"`
}

// PostInfo contains metadata about a post, extracted from comment API responses.
// Use this to obtain the MediaID needed for Client.ListImages.
type PostInfo struct {
	StoryID string `json:"story_id,omitempty"`
	MediaID string `json:"media_id,omitempty"`
}
