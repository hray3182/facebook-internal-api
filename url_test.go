package fbia

import "testing"

func TestParsePostURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		postID  string
		groupID string
	}{
		{
			name:    "hoist multi_permalinks",
			rawURL:  "https://www.facebook.com/groups/709857932418486/?multi_permalinks=37480973331546816&hoisted_section_header_type=recently_seen",
			postID:  "37480973331546816",
			groupID: "709857932418486",
		},
		{
			name:    "multi_permalinks multiple IDs",
			rawURL:  "https://www.facebook.com/groups/709857932418486/?multi_permalinks=111,222",
			postID:  "111",
			groupID: "709857932418486",
		},
		{
			name:    "group /posts/",
			rawURL:  "https://www.facebook.com/groups/123456/posts/789/",
			postID:  "789",
			groupID: "123456",
		},
		{
			name:    "group /permalink/",
			rawURL:  "https://www.facebook.com/groups/example/permalink/987654321/",
			postID:  "987654321",
			groupID: "example",
		},
		{
			name:    "page /posts/ + comment_id",
			rawURL:  "https://www.facebook.com/example/posts/123456789?comment_id=1",
			postID:  "123456789",
			groupID: "",
		},
		{
			name:    "story.php",
			rawURL:  "https://www.facebook.com/story.php?story_fbid=111&id=222",
			postID:  "111",
			groupID: "",
		},
		{
			name:    "permalink.php",
			rawURL:  "https://www.facebook.com/permalink.php?story_fbid=111&id=222",
			postID:  "111",
			groupID: "",
		},
		{
			name:    "fbid",
			rawURL:  "https://www.facebook.com/photo.php?fbid=555666777",
			postID:  "555666777",
			groupID: "",
		},
		{
			name:    "video_id",
			rawURL:  "https://www.facebook.com/watch/?video_id=1122334455",
			postID:  "1122334455",
			groupID: "",
		},
		{
			name:    "group homepage only",
			rawURL:  "https://www.facebook.com/groups/709857932418486/",
			postID:  "",
			groupID: "709857932418486",
		},
		{
			name:    "blank",
			rawURL:  "  ",
			postID:  "",
			groupID: "",
		},
		{
			name:    "query wins over path posts segment",
			rawURL:  "https://www.facebook.com/groups/123/posts/999/?multi_permalinks=111",
			postID:  "111",
			groupID: "123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePostURL(tt.rawURL)
			if got.PostID != tt.postID || got.GroupID != tt.groupID {
				t.Fatalf("ParsePostURL(%q) = {PostID:%q GroupID:%q}, want {PostID:%q GroupID:%q}",
					tt.rawURL, got.PostID, got.GroupID, tt.postID, tt.groupID)
			}
		})
	}
}

func TestParsePostURL_hoist_post_id_is_not_group_id(t *testing.T) {
	const (
		groupID = "709857932418486"
		postID  = "37480973331546816"
		rawURL  = "https://www.facebook.com/groups/709857932418486/?multi_permalinks=37480973331546816&hoisted_section_header_type=recently_seen"
	)

	got := ParsePostURL(rawURL)
	if got.PostID == groupID {
		t.Fatalf("PostID = %q, must not equal group ID", got.PostID)
	}
	if got.PostID != postID {
		t.Fatalf("PostID = %q, want %q", got.PostID, postID)
	}
	if got.GroupID != groupID {
		t.Fatalf("GroupID = %q, want %q", got.GroupID, groupID)
	}
}
