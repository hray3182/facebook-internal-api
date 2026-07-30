// Package main probes whether DefaultDocIDs still work against live Facebook GraphQL.
//
//	go run ./cmd/probe -auth auth.json -user USER_ID -group GROUP_ID -post POST_ID
//
// auth.json is produced by extension/ (cookies + fb_dtsg + optional lsd).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	fbia "github.com/hray3182/facebook-internal-api"
)

type authFile struct {
	Cookies    []cookieEntry `json:"cookies"`
	CookieMap  map[string]string
	FBDTSG     string `json:"fb_dtsg"`
	LSD        string `json:"lsd"`
	CUser      string `json:"c_user"`
	ExtractedAt string `json:"extracted_at"`
}

type cookieEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type probeResult struct {
	Name   string
	DocID  string
	OK     bool
	Detail string
}

func main() {
	authPath := flag.String("auth", "auth.json", "path to auth.json from the cookie extension")
	userID := flag.String("user", envOr("FB_PROBE_USER", ""), "profile/page user id for Posts probe")
	groupID := flag.String("group", envOr("FB_PROBE_GROUP", ""), "group id for Groups probe")
	postID := flag.String("post", envOr("FB_PROBE_POST", ""), "post id for Comments/Replies/Photos probe")
	timeout := flag.Duration("timeout", 30*time.Second, "per-probe timeout")
	flag.Parse()

	auth, err := loadAuth(*authPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load auth: %v\n", err)
		os.Exit(2)
	}

	opts := []fbia.Option{}
	if auth.LSD != "" {
		opts = append(opts, fbia.WithLSD(auth.LSD))
	}
	client := fbia.NewClient(auth.CookieMap, auth.FBDTSG, opts...)

	fmt.Printf("probing DefaultDocIDs (as of library defaults)\n")
	fmt.Printf("auth: %s  c_user=%s\n\n", *authPath, auth.CookieMap["c_user"])

	var results []probeResult
	ctx := context.Background()

	if *userID != "" {
		results = append(results, probePosts(ctx, client, *userID, *timeout))
	} else {
		results = append(results, skipped("Posts", fbia.DefaultDocIDs.Posts, "pass -user"))
	}

	if *groupID != "" {
		results = append(results, probeGroups(ctx, client, *groupID, *timeout))
	} else {
		results = append(results, skipped("Groups", fbia.DefaultDocIDs.Groups, "pass -group"))
	}

	if *postID != "" {
		results = append(results, probeComments(ctx, client, *postID, *timeout)...)
	} else if *groupID != "" {
		// Prefer a story ID from the group feed when no post was given.
		results = append(results, skipped("Comments", fbia.DefaultDocIDs.Comments, "pass -post (or rely on group feed story ids)"))
		results = append(results,
			skipped("Replies", fbia.DefaultDocIDs.Replies, "pass -post"),
			skipped("Photos", fbia.DefaultDocIDs.Photos, "pass -post"),
		)
	} else {
		results = append(results,
			skipped("Comments", fbia.DefaultDocIDs.Comments, "pass -post"),
			skipped("Replies", fbia.DefaultDocIDs.Replies, "pass -post"),
			skipped("Photos", fbia.DefaultDocIDs.Photos, "pass -post"),
		)
	}

	results = append(results,
		skipped("CreateComment", fbia.DefaultDocIDs.CreateComment, "mutation; capture via tools/capture"),
		skipped("DeleteComment", fbia.DefaultDocIDs.DeleteComment, "mutation; capture via tools/capture"),
	)

	failed := 0
	for _, r := range results {
		status := "OK"
		switch {
		case strings.HasPrefix(r.Detail, "skipped:"):
			status = "SKIP"
		case !r.OK:
			status = "FAIL"
			failed++
		}
		fmt.Printf("%-6s %-14s doc_id=%s  %s\n", status, r.Name, r.DocID, r.Detail)
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d probe(s) failed — doc_id may have rotated; run tools/capture\n", failed)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadAuth(path string) (*authFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a authFile
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	a.CookieMap = make(map[string]string, len(a.Cookies))
	for _, c := range a.Cookies {
		if c.Name != "" {
			a.CookieMap[c.Name] = c.Value
		}
	}
	if a.FBDTSG == "" {
		return nil, errors.New("auth.json missing fb_dtsg")
	}
	if len(a.CookieMap) == 0 {
		return nil, errors.New("auth.json has no cookies")
	}
	if a.CookieMap["c_user"] == "" && a.CUser != "" {
		a.CookieMap["c_user"] = a.CUser
	}
	return &a, nil
}

func skipped(name, docID, reason string) probeResult {
	return probeResult{Name: name, DocID: docID, OK: true, Detail: "skipped: " + reason}
}

func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

func probePosts(parent context.Context, c *fbia.Client, userID string, d time.Duration) probeResult {
	ctx, cancel := withTimeout(parent, d)
	defer cancel()

	for post, err := range c.ListPosts(ctx, userID) {
		if err != nil {
			return fail("Posts", fbia.DefaultDocIDs.Posts, err)
		}
		return ok("Posts", fbia.DefaultDocIDs.Posts, fmt.Sprintf("got post %s", post.PostID))
	}
	return ok("Posts", fbia.DefaultDocIDs.Posts, "request ok, 0 posts")
}

func probeGroups(parent context.Context, c *fbia.Client, groupID string, d time.Duration) probeResult {
	ctx, cancel := withTimeout(parent, d)
	defer cancel()

	for post, err := range c.ListGroupPosts(ctx, groupID) {
		if err != nil {
			return fail("Groups", fbia.DefaultDocIDs.Groups, err)
		}
		return ok("Groups", fbia.DefaultDocIDs.Groups, fmt.Sprintf("got post %s", post.PostID))
	}
	return ok("Groups", fbia.DefaultDocIDs.Groups, "request ok, 0 posts")
}

func probeComments(parent context.Context, c *fbia.Client, postID string, d time.Duration) []probeResult {
	// Prefer a real StoryID from the caller's post when possible; FeedbackID
	// synthesis only works for posts authored by c_user.
	id := fbia.FeedbackID(postID)
	var out []probeResult

	{
		ctx, cancel := withTimeout(parent, d)
		var first *fbia.Comment
		var n int
		for comment, err := range c.ListComments(ctx, id) {
			if err != nil {
				out = append(out, fail("Comments", fbia.DefaultDocIDs.Comments, err))
				cancel()
				out = append(out,
					fail("Replies", fbia.DefaultDocIDs.Replies, errors.New("skipped: comments failed")),
					fail("Photos", fbia.DefaultDocIDs.Photos, errors.New("skipped: comments failed")),
				)
				return out
			}
			n++
			cp := comment
			if first == nil {
				first = &cp
			}
		}
		cancel()
		if first == nil {
			out = append(out, ok("Comments", fbia.DefaultDocIDs.Comments, "request ok, 0 comments"))
			out = append(out, skipped("Replies", fbia.DefaultDocIDs.Replies, "no comments to probe"))
		} else {
			out = append(out, ok("Comments", fbia.DefaultDocIDs.Comments, fmt.Sprintf("got %d comment(s), first=%s", n, first.CommentID)))
			ctx, cancel := withTimeout(parent, d)
			replies, err := c.FetchReplies(ctx, *first)
			cancel()
			if err != nil {
				out = append(out, fail("Replies", fbia.DefaultDocIDs.Replies, err))
			} else {
				out = append(out, ok("Replies", fbia.DefaultDocIDs.Replies, fmt.Sprintf("%d replies", len(replies))))
			}
		}
	}

	{
		ctx, cancel := withTimeout(parent, d)
		info, err := c.FetchPostInfo(ctx, id)
		cancel()
		if err != nil {
			out = append(out, fail("Photos", fbia.DefaultDocIDs.Photos, err))
			return out
		}
		if info == nil || info.MediaID == "" {
			out = append(out, skipped("Photos", fbia.DefaultDocIDs.Photos, "no media_id on post"))
			return out
		}
		ctx, cancel = withTimeout(parent, d)
		for img, err := range c.ListImages(ctx, info.MediaID, postID) {
			if err != nil {
				out = append(out, fail("Photos", fbia.DefaultDocIDs.Photos, err))
				cancel()
				return out
			}
			out = append(out, ok("Photos", fbia.DefaultDocIDs.Photos, fmt.Sprintf("got image node=%s", img.NodeID)))
			cancel()
			return out
		}
		cancel()
		out = append(out, ok("Photos", fbia.DefaultDocIDs.Photos, "request ok, 0 images"))
	}
	return out
}

func ok(name, docID, detail string) probeResult {
	return probeResult{Name: name, DocID: docID, OK: true, Detail: detail}
}

func fail(name, docID string, err error) probeResult {
	detail := err.Error()
	var ge *fbia.GraphQLError
	if errors.As(err, &ge) && ge.Code == 1675012 {
		detail = fmt.Sprintf("doc_id/variables mismatch (code 1675012): %s", ge.Summary)
	}
	return probeResult{Name: name, DocID: docID, OK: false, Detail: detail}
}
