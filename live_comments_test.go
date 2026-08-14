package fbia

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

// Live fixture: test-group post with many top-level comments.
// https://www.facebook.com/groups/1635204946735429/posts/4400009946921568
const (
	liveTestGroupID             = "1635204946735429"
	liveTestPostID              = "4400009946921568"
	liveTestMinCommentCount     = 40
	liveTestAuthPath            = "auth.json"
	liveTestCommentTogglePostID = "1722750337980889"
)

type liveAuthFile struct {
	Cookies []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"cookies"`
	FBDTSG  string            `json:"fb_dtsg"`
	LSD     string            `json:"lsd"`
	CUser   string            `json:"c_user"`
	Session map[string]string `json:"session"`
}

func loadLiveAuth(t *testing.T) (*Client, bool) {
	t.Helper()

	raw, err := os.ReadFile(liveTestAuthPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false
		}
		t.Fatalf("read %s: %v", liveTestAuthPath, err)
	}

	var a liveAuthFile
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("parse %s: %v", liveTestAuthPath, err)
	}
	if a.FBDTSG == "" || len(a.Cookies) == 0 {
		return nil, false
	}

	cookies := make(map[string]string, len(a.Cookies))
	for _, c := range a.Cookies {
		if c.Name != "" {
			cookies[c.Name] = c.Value
		}
	}
	if cookies["c_user"] == "" && a.CUser != "" {
		cookies["c_user"] = a.CUser
	}
	if cookies["c_user"] == "" || cookies["xs"] == "" {
		return nil, false
	}

	opts := []Option{}
	if a.LSD != "" {
		opts = append(opts, WithLSD(a.LSD))
	}
	if len(a.Session) > 0 {
		opts = append(opts, WithSessionForm(a.Session))
	}
	return NewClient(cookies, a.FBDTSG, opts...), true
}

func TestLive_ListComments_testGroupPostHasAtLeastForty(t *testing.T) {
	client, ok := loadLiveAuth(t)
	if !ok {
		t.Skip("skip live test: place a valid auth.json (extension export) in the module root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	comments, err := Collect(client.ListComments(ctx, FeedbackID(liveTestPostID)))
	if err != nil {
		t.Fatalf("ListComments(%s): %v", liveTestPostID, err)
	}
	if len(comments) < liveTestMinCommentCount {
		t.Fatalf("got %d comments, want >= %d (group=%s post=%s)",
			len(comments), liveTestMinCommentCount, liveTestGroupID, liveTestPostID)
	}
	t.Logf("got %d comments from post %s", len(comments), liveTestPostID)
}

func TestLive_GroupFeed_includesCommentsDisabled(t *testing.T) {
	client, ok := loadLiveAuth(t)
	if !ok {
		t.Skip("skip live test: place a valid auth.json (extension export) in the module root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	n := 0
	for post, err := range client.ListGroupPosts(ctx, liveTestGroupID) {
		if err != nil {
			t.Fatalf("ListGroupPosts(%s): %v", liveTestGroupID, err)
		}
		n++
		if post.CommentsDisabled == nil {
			t.Fatalf("post %s CommentsDisabled is nil; group UFI should include have_comments_been_disabled", post.PostID)
		}
		t.Logf("post %s comments_disabled=%t comment_count=%d", post.PostID, *post.CommentsDisabled, post.CommentCount)
		if n >= 3 {
			return
		}
	}
	if n == 0 {
		t.Fatal("ListGroupPosts returned 0 posts")
	}
}

func TestLive_FetchCommentingStatus(t *testing.T) {
	client, ok := loadLiveAuth(t)
	if !ok {
		t.Skip("skip live test: place a valid auth.json (extension export) in the module root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status, err := client.FetchCommentingStatus(ctx, liveTestCommentTogglePostID)
	if err != nil {
		t.Fatalf("FetchCommentingStatus(%s): %v", liveTestCommentTogglePostID, err)
	}
	t.Logf("feedback_id=%s comments_disabled=%t", status.FeedbackID, status.CommentsDisabled)
}

func TestLive_ToggleCommenting_roundTrip(t *testing.T) {
	client, ok := loadLiveAuth(t)
	if !ok {
		t.Skip("skip live test: place a valid auth.json (extension export) in the module root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	original, fetchErr := client.FetchCommentingStatus(ctx, liveTestCommentTogglePostID)
	if fetchErr != nil {
		t.Logf("FetchCommentingStatus before toggle: %v", fetchErr)
	} else {
		t.Logf("original comments_disabled=%t", original.CommentsDisabled)
	}

	disabled, err := client.DisableCommenting(ctx, liveTestCommentTogglePostID)
	if err != nil {
		t.Fatalf("DisableCommenting(%s): %v", liveTestCommentTogglePostID, err)
	}
	if !disabled.CommentsDisabled {
		t.Fatalf("after DisableCommenting, CommentsDisabled=%t, want true", disabled.CommentsDisabled)
	}
	t.Logf("disabled ok feedback_id=%s", disabled.FeedbackID)

	afterDisable, err := client.FetchCommentingStatus(ctx, liveTestCommentTogglePostID)
	if err != nil {
		t.Fatalf("FetchCommentingStatus after disable: %v", err)
	}
	if !afterDisable.CommentsDisabled {
		t.Fatalf("FetchCommentingStatus after disable = false, want true")
	}

	enabled, err := client.EnableCommenting(ctx, liveTestCommentTogglePostID)
	if err != nil {
		t.Fatalf("EnableCommenting(%s): %v", liveTestCommentTogglePostID, err)
	}
	if enabled.CommentsDisabled {
		t.Fatalf("after EnableCommenting, CommentsDisabled=%t, want false", enabled.CommentsDisabled)
	}
	t.Logf("enabled ok feedback_id=%s", enabled.FeedbackID)

	afterEnable, err := client.FetchCommentingStatus(ctx, liveTestCommentTogglePostID)
	if err != nil {
		t.Fatalf("FetchCommentingStatus after enable: %v", err)
	}
	if afterEnable.CommentsDisabled {
		t.Fatalf("FetchCommentingStatus after enable = true, want false")
	}

	if fetchErr == nil && original.CommentsDisabled {
		restored, err := client.DisableCommenting(ctx, liveTestCommentTogglePostID)
		if err != nil {
			t.Fatalf("restore DisableCommenting(%s): %v", liveTestCommentTogglePostID, err)
		}
		t.Logf("restored comments_disabled=%t", restored.CommentsDisabled)
	}
}
