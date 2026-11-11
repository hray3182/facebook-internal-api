package fbia

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// recordingRoundTripper answers requests in order and keeps each request's
// friendly name and decoded variables.
type recordingRoundTripper struct {
	t         *testing.T
	responses []string
	friendly  []string
	variables []string
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	n := len(r.friendly)
	if n >= len(r.responses) {
		r.t.Fatalf("unexpected RoundTrip call %d", n+1)
	}
	raw, _ := io.ReadAll(req.Body)
	form, _ := url.ParseQuery(string(raw))
	r.friendly = append(r.friendly, req.Header.Get("X-FB-Friendly-Name"))
	r.variables = append(r.variables, form.Get("variables"))
	body := r.responses[n]
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func newRecordingClient(t *testing.T, responses ...string) (*Client, *recordingRoundTripper) {
	rt := &recordingRoundTripper{t: t, responses: responses}
	c := NewClient(
		map[string]string{"c_user": "123", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: rt}),
	)
	return c, rt
}

func TestDisableCommenting_numericPostID_looksUpPfbidFeedbackFirst(t *testing.T) {
	c, rt := newRecordingClient(t, commentingStatusGraphQLResponse(false), commentingToggleGraphQLResponse(true))

	status, err := c.DisableCommenting(context.Background(), "1722750337980889")
	if err != nil {
		t.Fatalf("DisableCommenting() error = %v", err)
	}
	if !status.CommentsDisabled {
		t.Fatal("CommentsDisabled = false, want true")
	}
	if len(rt.friendly) != 2 || rt.friendly[0] != commentsDialogFriendlyName || rt.friendly[1] != disableCommentingFriendlyName {
		t.Fatalf("requests = %v, want dialog lookup then disable mutation", rt.friendly)
	}
	if !strings.Contains(rt.variables[1], FeedbackID("pfbid0TestPost")) {
		t.Fatalf("mutation variables = %s, want pfbid feedback id", rt.variables[1])
	}
	if strings.Contains(rt.variables[1], FeedbackID("1722750337980889")) {
		t.Fatalf("mutation still sends numeric feedback id: %s", rt.variables[1])
	}
}

func TestResolveFeedbackID_numericFeedbackID_isLookedUpAndCached(t *testing.T) {
	c, rt := newRecordingClient(t, commentingStatusGraphQLResponse(false))

	for i := 0; i < 2; i++ {
		got, err := c.resolveFeedbackID(context.Background(), FeedbackID("1722750337980889"))
		if err != nil {
			t.Fatalf("resolveFeedbackID() error = %v", err)
		}
		if got != FeedbackID("pfbid0TestPost") {
			t.Fatalf("resolveFeedbackID() = %q, want pfbid feedback id", got)
		}
	}
	if len(rt.friendly) != 1 {
		t.Fatalf("requests = %d, want 1 (second call served from cache)", len(rt.friendly))
	}
}

func TestResolveFeedbackID_pfbidFeedbackID_needsNoRequest(t *testing.T) {
	c, rt := newRecordingClient(t)

	got, err := c.resolveFeedbackID(context.Background(), FeedbackID("pfbid0TestPost"))
	if err != nil {
		t.Fatalf("resolveFeedbackID() error = %v", err)
	}
	if got != FeedbackID("pfbid0TestPost") || len(rt.friendly) != 0 {
		t.Fatalf("got %q with %d requests", got, len(rt.friendly))
	}
}

func TestResolveStoryIDForDialog_rejectsPfbidFeedbackID(t *testing.T) {
	c, _ := newRecordingClient(t)
	if _, err := c.resolveStoryIDForDialog(FeedbackID("pfbid0TestPost")); err == nil {
		t.Fatal("want error for pfbid feedback id")
	}
}

func TestQueryVariables_includeRelayProviders(t *testing.T) {
	dialog := commentsDialogVariables("story")
	for k := range postDialogRelayProviders {
		if _, ok := dialog[k]; !ok {
			t.Fatalf("dialog variables missing %s", k)
		}
	}
	list := commentsListVariables(FeedbackID("pfbid0TestPost"), "")
	if _, ok := list["targetDialect"]; !ok {
		t.Fatal("comments list variables missing targetDialect")
	}
}
