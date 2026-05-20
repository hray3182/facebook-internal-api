package fbia

import (
	"context"
	"fmt"
	"iter"
)

// ListGroupPosts returns a sequence of posts from a Facebook group.
func (c *Client) ListGroupPosts(ctx context.Context, groupID string) iter.Seq2[Post, error] {
	return func(yield func(Post, error) bool) {
		var cursor string

		for {
			variables := map[string]any{
				"count":                        3,
				"cursor":                       nilIfEmpty(cursor),
				"feedLocation":                 "GROUP",
				"feedType":                     "DISCUSSION",
				"feedbackSource":               0,
				"filterTopicId":                nil,
				"focusCommentID":               nil,
				"privacySelectorRenderLocation": "COMET_STREAM",
				"renderLocation":               "group",
				"scale":                        2,
				"stream_initial_count":         1,
				"useDefaultActor":              false,
				"id":                           groupID,
			}

			body, err := c.doRequest(ctx, c.docIDs.Groups, variables, "GroupsCometFeedRegularStoriesPaginationQuery")
			if err != nil {
				yield(Post{}, fmt.Errorf("list group posts: %w", err))
				return
			}

			posts, nextCursor, err := parseGroupPostsResponse(body)
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

func parseGroupPostsResponse(body string) ([]Post, string, error) {
	blocks := extractDataBlocks(body)
	if len(blocks) == 0 {
		return nil, "", nil
	}

	stories := collectStoryNodes(blocks)

	var nextCursor string
	for _, block := range blocks {
		pi := jsonMap(block, "page_info")
		if pi != nil {
			if hasNext, _ := pi["has_next_page"].(bool); hasNext {
				if c := jsonStr(pi, "end_cursor"); c != "" {
					nextCursor = c
				}
			}
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
