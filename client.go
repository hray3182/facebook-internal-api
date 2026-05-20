package fbia

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const graphqlURL = "https://www.facebook.com/api/graphql/"

// DocIDs holds the Facebook GraphQL document IDs used by each API method.
// Facebook rotates these periodically; pass updated values via WithDocIDs.
type DocIDs struct {
	Posts    string // ProfileCometTimelineFeedRefetchQuery
	Groups   string // GroupsCometFeedRegularStoriesPaginationQuery
	Comments string // CommentsListComponentsPaginationQuery
	Replies  string // Depth1CommentsListPaginationQuery
	Photos   string // CometPhotoRootContentQuery
}

// DefaultDocIDs contains the doc_id values known to work as of 2025-05.
var DefaultDocIDs = DocIDs{
	Posts:    "25430544756617998",
	Groups:   "25716860671307636",
	Comments: "25550760954572974",
	Replies:  "26570577339199586",
	Photos:   "26168653472729001",
}

// Client talks to Facebook's internal GraphQL API.
type Client struct {
	http      *http.Client
	cookies   map[string]string
	fbDTSG    string
	userAgent string
	docIDs    DocIDs
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.http = c }
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(cl *Client) { cl.userAgent = ua }
}

// WithDocIDs overrides the default GraphQL document IDs.
// Zero-value fields fall back to DefaultDocIDs.
func WithDocIDs(ids DocIDs) Option {
	return func(cl *Client) {
		if ids.Posts != "" {
			cl.docIDs.Posts = ids.Posts
		}
		if ids.Groups != "" {
			cl.docIDs.Groups = ids.Groups
		}
		if ids.Comments != "" {
			cl.docIDs.Comments = ids.Comments
		}
		if ids.Replies != "" {
			cl.docIDs.Replies = ids.Replies
		}
		if ids.Photos != "" {
			cl.docIDs.Photos = ids.Photos
		}
	}
}

// NewClient creates a client authenticated with the given Facebook session cookies and DTSG token.
func NewClient(cookies map[string]string, fbDTSG string, opts ...Option) *Client {
	c := &Client{
		http:      http.DefaultClient,
		cookies:   cookies,
		fbDTSG:    fbDTSG,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		docIDs:    DefaultDocIDs,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) userID() string {
	if id := c.cookies["c_user"]; id != "" {
		return id
	}
	return "0"
}

// FeedbackID converts a post ID to the base64-encoded feedback ID that the comment API expects.
func FeedbackID(postID string) string {
	return base64.StdEncoding.EncodeToString([]byte("feedback:" + postID))
}

func (c *Client) doRequest(ctx context.Context, docID string, variables map[string]any, friendlyName string) (string, error) {
	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return "", fmt.Errorf("marshal variables: %w", err)
	}

	form := url.Values{
		"av":        {c.userID()},
		"__user":    {c.userID()},
		"__a":       {"1"},
		"fb_dtsg":   {c.fbDTSG},
		"doc_id":    {docID},
		"variables": {string(varsJSON)},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphqlURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Origin", "https://www.facebook.com")
	if friendlyName != "" {
		req.Header.Set("X-FB-Friendly-Name", friendlyName)
	}

	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}

	return string(body), nil
}
