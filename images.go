package fbia

import (
	"context"
	"fmt"
	"iter"
)

// ListImages walks a post's media set one image at a time, following the
// nextMediaAfterNodeId linked-list until there are no more images.
func (c *Client) ListImages(ctx context.Context, startNodeID string, postID string) iter.Seq2[Image, error] {
	return func(yield func(Image, error) bool) {
		visited := make(map[string]bool)
		nodeID := startNodeID

		for nodeID != "" && !visited[nodeID] {
			visited[nodeID] = true

			variables := map[string]any{
				"isMediaset":                     true,
				"renderLocation":                 "comet_media_viewer",
				"nodeID":                         nodeID,
				"mediasetToken":                  fmt.Sprintf("pcb.%s", postID),
				"scale":                          2,
				"feedLocation":                   "COMET_MEDIA_VIEWER",
				"feedbackSource":                 65,
				"focusCommentID":                 nil,
				"privacySelectorRenderLocation":  "COMET_MEDIA_VIEWER",
				"useDefaultActor":                false,
				"shouldShowComments":             true,
			}

			body, err := c.doRequest(ctx, c.docIDs.Photos, variables, "CometPhotoRootContentQuery")
			if err != nil {
				yield(Image{}, fmt.Errorf("list images: %w", err))
				return
			}

			blocks := extractDataBlocks(body)

			var imageURL string
			for _, block := range blocks {
				img := jsonMap(jsonMap(block, "currMedia"), "image")
				if u := jsonStr(img, "uri"); u != "" {
					imageURL = u
					break
				}
			}

			if imageURL == "" {
				return
			}

			if !yield(Image{NodeID: nodeID, URL: imageURL}, nil) {
				return
			}

			nodeID = ""
			for _, block := range blocks {
				nma := jsonMap(block, "nextMediaAfterNodeId")
				if id := jsonStr(nma, "id"); id != "" {
					nodeID = id
					break
				}
			}
		}
	}
}
