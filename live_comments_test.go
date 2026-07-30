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
	liveTestGroupID          = "1635204946735429"
	liveTestPostID           = "4400009946921568"
	liveTestMinCommentCount  = 40
	liveTestAuthPath         = "auth.json"
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
