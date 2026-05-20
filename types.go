package fbia

// Post represents a Facebook post from a page timeline or group feed.
type Post struct {
	PostID       string  `json:"post_id"`
	FeedbackID   string  `json:"feedback_id,omitempty"`
	Text         string  `json:"text,omitempty"`
	Permalink    string  `json:"permalink,omitempty"`
	CommentCount int     `json:"comment_count"`
	PageName     string  `json:"page_name,omitempty"`
	GroupName    string  `json:"group_name,omitempty"`
	Media        []Media `json:"media,omitempty"`
	VideoOrReel  bool    `json:"video_or_reel,omitempty"`
}

// Comment represents a top-level comment on a post.
type Comment struct {
	Text          string `json:"text"`
	ReactionCount string `json:"reaction_count"`

	// FeedbackID and ExpansionToken are needed to fetch replies via Client.FetchReplies.
	FeedbackID     string `json:"feedback_id"`
	ExpansionToken string `json:"expansion_token"`
}

// Reply represents a reply to a comment.
type Reply struct {
	Text          string `json:"text"`
	ReactionCount string `json:"reaction_count"`
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
