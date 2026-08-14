package fbia

import (
	"net/url"
	"strings"
)

// PostRef is a Facebook post identified from a permalink.
type PostRef struct {
	PostID  string
	GroupID string
}

// ParsePostURL extracts post/group IDs from a Facebook permalink.
// Query IDs win over path segments.
func ParsePostURL(rawURL string) PostRef {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return PostRef{}
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		u = &url.URL{Path: rawURL}
	}

	segs := splitPath(u.Path)
	ref := PostRef{GroupID: groupIDFromPath(segs)}
	if id := postIDFromQuery(u.Query()); id != "" {
		ref.PostID = id
		return ref
	}
	ref.PostID = postIDFromPath(segs)
	return ref
}

func postIDFromQuery(q url.Values) string {
	for _, key := range []string{"story_fbid", "fbid", "video_id", "multi_permalinks"} {
		id := cleanID(q.Get(key))
		if key == "multi_permalinks" {
			id, _, _ = strings.Cut(id, ",")
			id = strings.TrimSpace(id)
		}
		if id != "" {
			return id
		}
	}
	return ""
}

func postIDFromPath(segs []string) string {
	for i, seg := range segs {
		if (seg == "posts" || seg == "permalink") && i+1 < len(segs) {
			if id := cleanID(segs[i+1]); id != "" {
				return id
			}
		}
	}
	return ""
}

func groupIDFromPath(segs []string) string {
	for i, seg := range segs {
		if seg == "groups" && i+1 < len(segs) {
			return cleanID(segs[i+1])
		}
	}
	return ""
}

func splitPath(path string) []string {
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func cleanID(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, "?#"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
