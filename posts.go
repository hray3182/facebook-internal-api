package fbia

import (
	"context"
	"fmt"
	"iter"
)

// ListPosts returns a sequence of posts from a user's timeline or page.
func (c *Client) ListPosts(ctx context.Context, userID string) iter.Seq2[Post, error] {
	return func(yield func(Post, error) bool) {
		var cursor string

		for {
			variables := map[string]any{
				"count":           3,
				"cursor":          nilIfEmpty(cursor),
				"id":              userID,
				"feedLocation":    "TIMELINE",
				"renderLocation":  "timeline",
				"scale":           2,
				"useDefaultActor": false,
			}

			body, err := c.doRequest(ctx, c.docIDs.Posts, variables, "ProfileCometTimelineFeedRefetchQuery")
			if err != nil {
				yield(Post{}, fmt.Errorf("list posts: %w", err))
				return
			}

			posts, nextCursor, err := parsePostsResponse(body)
			if err != nil {
				yield(Post{}, err)
				return
			}

			for _, p := range posts {
				if !yield(p, nil) {
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

func parsePostsResponse(body string) ([]Post, string, error) {
	blocks := extractDataBlocks(body)
	if len(blocks) == 0 {
		return nil, "", nil
	}

	stories := collectStoryNodes(blocks)

	var nextCursor string
	for _, block := range blocks {
		node := jsonMap(block, "node")
		tlfu := jsonMap(node, "timeline_list_feed_units")
		pi := jsonMap(tlfu, "page_info")
		if c := jsonStr(pi, "end_cursor"); c != "" {
			nextCursor = c
			break
		}
	}

	var posts []Post
	for _, story := range stories {
		p := extractPost(story)
		if p.PostID != "" {
			posts = append(posts, p)
		}
	}

	return posts, nextCursor, nil
}
