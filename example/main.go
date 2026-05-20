package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	fbia "github.com/hray3182/facebook-internal-api"
)

func main() {
	client := newClient()
	ctx := context.Background()

	if len(os.Args) < 2 {
		fmt.Println("usage: go run main.go <command> [args]")
		fmt.Println()
		fmt.Println("commands:")
		fmt.Println("  posts   <user_id>             list posts from a page/profile")
		fmt.Println("  group   <group_id>            list posts from a group")
		fmt.Println("  comments <post_id>            list comments on a post")
		fmt.Println("  replies  <post_id>            list comments + replies on a post")
		fmt.Println("  images   <media_id> <post_id> walk images in a post")
		fmt.Println("  postinfo <post_id>            fetch post metadata (media_id)")
		return
	}

	switch os.Args[1] {
	case "posts":
		demoPosts(ctx, client, os.Args[2])
	case "group":
		demoGroup(ctx, client, os.Args[2])
	case "comments":
		demoComments(ctx, client, os.Args[2])
	case "replies":
		demoReplies(ctx, client, os.Args[2])
	case "images":
		demoImages(ctx, client, os.Args[2], os.Args[3])
	case "postinfo":
		demoPostInfo(ctx, client, os.Args[2])
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func newClient() *fbia.Client {
	// --- Option A: pass cookies as a map directly ---
	//
	// client := fbia.NewClient(
	// 	map[string]string{
	// 		"c_user": "YOUR_C_USER",
	// 		"xs":     "YOUR_XS_TOKEN",
	// 		"datr":   "YOUR_DATR",
	// 		"fr":     "YOUR_FR",
	// 	},
	// 	"YOUR_FB_DTSG_TOKEN",
	// )
	//
	// --- Option B: with custom doc IDs (when Facebook rotates them) ---
	//
	// client := fbia.NewClient(cookies, dtsg,
	// 	fbia.WithDocIDs(fbia.DocIDs{
	// 		Comments: "NEW_DOC_ID_HERE",
	// 	}),
	// )
	//
	// --- Option C: from environment variables (used below) ---

	cookieStr := os.Getenv("FB_COOKIES")
	dtsg := os.Getenv("FB_DTSG")

	if cookieStr == "" || dtsg == "" {
		fmt.Fprintln(os.Stderr, "set FB_COOKIES and FB_DTSG environment variables")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "how to get them:")
		fmt.Fprintln(os.Stderr, "  1. open Facebook in browser -> DevTools -> Network tab")
		fmt.Fprintln(os.Stderr, "  2. find any graphql request")
		fmt.Fprintln(os.Stderr, "  3. copy the Cookie header value  -> export FB_COOKIES='...'")
		fmt.Fprintln(os.Stderr, "  4. copy fb_dtsg from request body -> export FB_DTSG='...'")
		os.Exit(1)
	}

	cookies := parseCookies(cookieStr)
	return fbia.NewClient(cookies, dtsg)
}

func parseCookies(raw string) map[string]string {
	cookies := make(map[string]string)
	for _, pair := range strings.Split(raw, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok {
			cookies[k] = v
		}
	}
	return cookies
}

func demoPosts(ctx context.Context, c *fbia.Client, userID string) {
	count := 0
	for post, err := range c.ListPosts(ctx, userID) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		printPost(post)
		if count >= 5 {
			break
		}
	}
	fmt.Printf("\nshowed %d posts\n", count)
}

func demoGroup(ctx context.Context, c *fbia.Client, groupID string) {
	count := 0
	for post, err := range c.ListGroupPosts(ctx, groupID) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		printPost(post)
		if count >= 5 {
			break
		}
	}
	fmt.Printf("\nshowed %d posts\n", count)
}

func demoComments(ctx context.Context, c *fbia.Client, postID string) {
	feedbackID := fbia.FeedbackID(postID)
	count := 0
	for comment, err := range c.ListComments(ctx, feedbackID) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		text := comment.Text
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		fmt.Printf("  [%s reactions] %s\n", comment.ReactionCount, text)
		if count >= 20 {
			break
		}
	}
	fmt.Printf("\nshowed %d comments\n", count)
}

func demoReplies(ctx context.Context, c *fbia.Client, postID string) {
	feedbackID := fbia.FeedbackID(postID)
	count := 0
	for comment, err := range c.ListComments(ctx, feedbackID) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		text := comment.Text
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		fmt.Printf("  [%s reactions] %s\n", comment.ReactionCount, text)

		replies, err := c.FetchReplies(ctx, comment)
		if err != nil {
			log.Printf("    error fetching replies: %v", err)
			continue
		}
		for _, r := range replies {
			rText := r.Text
			if len(rText) > 60 {
				rText = rText[:60] + "..."
			}
			fmt.Printf("    ↳ [%s] %s\n", r.ReactionCount, rText)
		}

		if count >= 10 {
			break
		}
	}
}

func demoImages(ctx context.Context, c *fbia.Client, mediaID, postID string) {
	count := 0
	for img, err := range c.ListImages(ctx, mediaID, postID) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		fmt.Printf("  image %d: node=%s url=%s\n", count, img.NodeID, img.URL[:80]+"...")
	}
	fmt.Printf("\nfound %d images\n", count)
}

func demoPostInfo(ctx context.Context, c *fbia.Client, postID string) {
	feedbackID := fbia.FeedbackID(postID)
	info, err := c.FetchPostInfo(ctx, feedbackID)
	if err != nil {
		log.Fatal(err)
	}
	if info == nil {
		fmt.Println("no post info found (post may have no comments)")
		return
	}
	fmt.Printf("story_id: %s\n", info.StoryID)
	fmt.Printf("media_id: %s\n", info.MediaID)

	if info.MediaID != "" {
		fmt.Println("\nyou can now run:")
		fmt.Printf("  go run main.go images %s %s\n", info.MediaID, postID)
	}
}

func printPost(p fbia.Post) {
	fmt.Printf("\n--- %s ---\n", p.PostID)
	if p.PageName != "" {
		fmt.Printf("  page: %s\n", p.PageName)
	}
	if p.GroupName != "" {
		fmt.Printf("  group: %s\n", p.GroupName)
	}
	text := p.Text
	if len(text) > 100 {
		text = text[:100] + "..."
	}
	if text != "" {
		fmt.Printf("  text: %s\n", text)
	}
	fmt.Printf("  comments: %d  video_or_reel: %v\n", p.CommentCount, p.VideoOrReel)
	for _, m := range p.Media {
		url := m.URL
		if len(url) > 60 {
			url = url[:60] + "..."
		}
		fmt.Printf("  media: [%s] id=%s url=%s\n", m.Type, m.ID, url)
	}
}
