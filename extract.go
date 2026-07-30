package fbia

import "strings"

func extractPost(node map[string]any) Post {
	postID := jsonStr(node, "post_id")
	if postID == "" {
		return Post{}
	}

	fb := jsonMap(node, "feedback")
	storyID := jsonStr(node, "id")
	if !strings.HasPrefix(storyID, "Uzpf") {
		storyID = ""
	}

	return Post{
		PostID:       postID,
		StoryID:      storyID,
		FeedbackID:   jsonStr(fb, "id"),
		Text:         extractMessage(node),
		Permalink:    extractPermalink(node),
		CommentCount: extractCommentCount(node),
		PageName:     extractPageName(node),
		GroupName:    extractGroupName(node),
		Media:        extractMedia(node),
		VideoOrReel:  isVideoOrReel(node),
	}
}

func extractMessage(node map[string]any) string {
	msg := jsonNav(node, "comet_sections", "content", "story", "message")
	return jsonStr(msg, "text")
}

func extractPermalink(node map[string]any) string {
	attachments := jsonSlice(node, "attachments")
	if len(attachments) == 0 {
		return ""
	}
	att, _ := attachments[0].(map[string]any)
	return jsonStr(jsonNav(att, "styles", "attachment"), "url")
}

// extractCommentCount tries 6 known JSON paths that Facebook uses in different
// response variants. The paths change depending on feed type, post age, and
// whether the response came from a timeline or group query.
func extractCommentCount(node map[string]any) int {
	paths := [][]string{
		{"feedback", "comment_rendering_instance", "comments"},
		{"comet_sections", "feedback", "story", "story_ufi_container", "story",
			"feedback_context", "feedback_target_with_context",
			"comment_rendering_instance", "comments"},
		{"comet_sections", "feedback", "story", "story_ufi_container", "story",
			"feedback_context", "feedback_target_with_context",
			"comet_ufi_summary_and_actions_renderer", "feedback",
			"comment_rendering_instance", "comments"},
		{"comet_sections", "feedback", "story",
			"feedback_context", "feedback_target_with_context",
			"comment_rendering_instance", "comments"},
		{"feedback", "comments_count_summary_renderer", "feedback",
			"comment_rendering_instance", "comments"},
		{"comet_sections", "feedback", "story", "story_ufi_container", "story",
			"feedback_context", "feedback_target_with_context",
			"comet_ufi_summary_and_actions_renderer", "feedback",
			"comments_count_summary_renderer", "feedback",
			"comment_rendering_instance", "comments"},
	}

	for _, p := range paths {
		if m := jsonNav(node, p...); m != nil {
			if v := jsonFloat(m, "total_count"); v > 0 {
				return int(v)
			}
		}
	}
	return 0
}

func extractPageName(node map[string]any) string {
	actors := jsonSlice(
		jsonNav(node, "comet_sections", "content", "story"),
		"actors",
	)
	if len(actors) > 0 {
		if a, ok := actors[0].(map[string]any); ok {
			if name := jsonStr(a, "name"); name != "" {
				return name
			}
		}
	}

	op := jsonNav(node, "feedback", "owning_profile")
	if name := jsonStr(op, "name"); name != "" {
		return name
	}
	return jsonStr(op, "short_name")
}

func extractGroupName(node map[string]any) string {
	to := jsonNav(node, "comet_sections", "context_layout", "story",
		"comet_sections", "title", "story", "to")
	if jsonStr(to, "__typename") == "Group" {
		if name := jsonStr(to, "name"); name != "" {
			return name
		}
	}

	tg := jsonNav(node, "comet_sections", "content", "story", "target_group")
	if name := jsonStr(tg, "name"); name != "" {
		return name
	}

	ag := jsonNav(node, "feedback", "associated_group")
	return jsonStr(ag, "name")
}

func extractMedia(node map[string]any) []Media {
	var media []Media

	for _, raw := range jsonSlice(node, "attachments") {
		att, _ := raw.(map[string]any)
		if att == nil {
			continue
		}

		styledMedia := jsonNav(att, "styles", "attachment", "media")
		directMedia := jsonMap(att, "media")

		for _, m := range []map[string]any{styledMedia, directMedia} {
			media = appendMediaFromNode(media, m)
		}

		subNodes := jsonSlice(jsonNav(att, "styles", "attachment", "all_subattachments"), "nodes")
		if subNodes == nil {
			subNodes = jsonSlice(jsonNav(att, "all_subattachments"), "nodes")
		}
		for _, s := range subNodes {
			sub, _ := s.(map[string]any)
			media = appendMediaFromNode(media, jsonMap(sub, "media"))
		}
	}
	return media
}

func appendMediaFromNode(media []Media, m map[string]any) []Media {
	if m == nil {
		return media
	}

	typename := jsonStr(m, "__typename")
	id := jsonStr(m, "id")

	if typename == "Video" {
		if u := jsonStr(m, "playable_url"); u != "" {
			media = append(media, Media{Type: "video", URL: u, ID: id})
		}
		return media
	}

	for _, imgKey := range []string{"photo_image", "image"} {
		img := jsonMap(m, imgKey)
		if u := jsonStr(img, "uri"); u != "" {
			media = append(media, Media{Type: "photo", URL: u, ID: id})
			return media
		}
	}
	return media
}

func isVideoOrReel(node map[string]any) bool {
	if strings.Contains(strings.ToLower(jsonStr(node, "__typename")), "reel") {
		return true
	}

	content := jsonNav(node, "comet_sections", "content")
	if strings.Contains(strings.ToLower(jsonStr(content, "__typename")), "reel") {
		return true
	}

	for _, raw := range jsonSlice(node, "attachments") {
		att, _ := raw.(map[string]any)
		if att == nil {
			continue
		}

		for _, m := range []map[string]any{
			jsonMap(att, "media"),
			jsonNav(att, "styles", "attachment", "media"),
		} {
			if m == nil {
				continue
			}
			if jsonStr(m, "__typename") == "Video" {
				return true
			}
			if strings.Contains(strings.ToLower(jsonStr(m, "__typename")), "reel") {
				return true
			}
		}

		subNodes := jsonSlice(jsonNav(att, "all_subattachments"), "nodes")
		for _, s := range subNodes {
			sub, _ := s.(map[string]any)
			sm := jsonMap(sub, "media")
			if jsonStr(sm, "__typename") == "Video" {
				return true
			}
		}
	}
	return false
}

func collectStoryNodes(blocks []map[string]any) []map[string]any {
	var stories []map[string]any

	for _, block := range blocks {
		node := jsonMap(block, "node")
		if node == nil {
			continue
		}

		typename := jsonStr(node, "__typename")

		if tlfu := jsonMap(node, "timeline_list_feed_units"); tlfu != nil {
			for _, e := range jsonSlice(tlfu, "edges") {
				edge, _ := e.(map[string]any)
				en := jsonMap(edge, "node")
				if jsonStr(en, "__typename") == "Story" {
					stories = append(stories, en)
				}
			}
		}

		if typename == "Story" {
			stories = append(stories, node)
		}

		if typename == "Group" {
			gf := jsonMap(node, "group_feed")
			for _, e := range jsonSlice(gf, "edges") {
				edge, _ := e.(map[string]any)
				en := jsonMap(edge, "node")
				if jsonStr(en, "__typename") == "Story" {
					stories = append(stories, en)
				}
			}
		}
	}
	return stories
}
