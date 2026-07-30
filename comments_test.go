package fbia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestParseCommentsResponse_returns_error_when_graphql_errors_hide_comments(t *testing.T) {
	// Given
	body := `{"errors":[{"message":"A server error missing_required_variable_value occured. Check server logs for details.","severity":"CRITICAL","code":1675012,"summary":"無法處理你的請求","description":"處理此要求時發生問題，請稍後再試。","is_silent":false}],"extensions":{}}`

	// When
	_, _, _, err := parseCommentsResponse(body)

	// Then
	if !errors.Is(err, ErrGraphQLResponse) {
		t.Fatalf("parseCommentsResponse() error = %v, want ErrGraphQLResponse", err)
	}
	var graphQLErr *GraphQLError
	if !errors.As(err, &graphQLErr) {
		t.Fatalf("parseCommentsResponse() error type = %T, want *GraphQLError", err)
	}
	if graphQLErr.Code != 1675012 {
		t.Fatalf("GraphQLError.Code = %d, want 1675012", graphQLErr.Code)
	}
	if graphQLErr.Severity != "CRITICAL" {
		t.Fatalf("GraphQLError.Severity = %q, want CRITICAL", graphQLErr.Severity)
	}
}

func TestParseCommentsResponse_allows_warning_errors_when_comments_exist(t *testing.T) {
	// Given
	body := `{"data":{"node":{"comment_rendering_instance_for_feed_location":{"comments":{"edges":[],"page_info":{"end_cursor":null}}}}},"errors":[{"message":"warning","severity":"WARNING"}],"extensions":{}}`

	// When
	comments, cursor, _, err := parseCommentsResponse(body)

	// Then
	if err != nil {
		t.Fatalf("parseCommentsResponse() error = %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("len(comments) = %d, want 0", len(comments))
	}
	if cursor != "" {
		t.Fatalf("cursor = %q, want empty", cursor)
	}
}

func TestParseCommentsResponse_returns_error_when_comments_block_missing(t *testing.T) {
	// Given
	body := `{"data":{"node":{"__typename":"Feedback"}}}`

	// When
	_, _, _, err := parseCommentsResponse(body)

	// Then
	if !errors.Is(err, ErrGraphQLResponse) {
		t.Fatalf("parseCommentsResponse() error = %v, want ErrGraphQLResponse", err)
	}
}

func TestParseCommentsResponse_detects_facebook_ar_error(t *testing.T) {
	body := `for (;;);{"__ar":1,"error":1357004,"errorSummary":"Sorry, something went wrong","errorDescription":"Please try closing and re-opening your browser window.","isNotCritical":1}`

	_, _, _, err := parseCommentsResponse(body)

	if !errors.Is(err, ErrGraphQLResponse) {
		t.Fatalf("error = %v, want ErrGraphQLResponse", err)
	}
	var ge *GraphQLError
	if !errors.As(err, &ge) {
		t.Fatalf("error type = %T, want *GraphQLError", err)
	}
	if ge.Code != 1357004 {
		t.Fatalf("Code = %d, want 1357004", ge.Code)
	}
}

func TestListComments_reports_incomplete_when_more_pages_without_CommentsPage(t *testing.T) {
	transport := &sequenceRoundTripper{
		t: t,
		responses: []string{
			dialogCommentsResponse(true),
		},
	}
	client := NewClient(
		map[string]string{"c_user": "123", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: transport}),
	)
	client.retryDelay = 0

	comments, err := Collect(client.ListComments(context.Background(), StoryID("123", "36674113645566126")))
	if !errors.Is(err, ErrIncompleteComments) {
		t.Fatalf("error = %v, want ErrIncompleteComments", err)
	}
	if len(comments) != 1 {
		t.Fatalf("len(comments) = %d, want 1 (first page still returned)", len(comments))
	}
}

func TestListComments_paginates_when_CommentsPage_configured(t *testing.T) {
	transport := &sequenceRoundTripper{
		t: t,
		responses: []string{
			dialogCommentsResponse(true),
			commentsGraphQLResponse(),
		},
	}
	client := NewClient(
		map[string]string{"c_user": "123", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithDocIDs(DocIDs{CommentsPage: "page-doc"}),
	)
	client.retryDelay = 0

	comments, err := Collect(client.ListComments(context.Background(), StoryID("123", "36674113645566126")))
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if transport.calls != 2 {
		t.Fatalf("RoundTrip calls = %d, want 2", transport.calls)
	}
	if len(comments) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(comments))
	}
}

func TestStoryID_roundtrip(t *testing.T) {
	id := StoryID("100007461752812", "1722750337980889")
	if !IsStoryID(id) {
		t.Fatalf("IsStoryID(%q) = false", id)
	}
	author, post, err := AuthorPostFromStory(id)
	if err != nil {
		t.Fatal(err)
	}
	if author != "100007461752812" || post != "1722750337980889" {
		t.Fatalf("got author=%q post=%q", author, post)
	}
	if IsStoryID(FeedbackID("1722750337980889")) {
		t.Fatal("FeedbackID should not be IsStoryID")
	}
}

func TestListComments_retries_when_graphql_error_hides_comments(t *testing.T) {
	// Given
	transport := &sequenceRoundTripper{
		t: t,
		responses: []string{
			`{"errors":[{"message":"A server error missing_required_variable_value occured. Check server logs for details.","severity":"CRITICAL","code":1675012,"summary":"無法處理你的請求","description":"處理此要求時發生問題，請稍後再試。","is_silent":false}],"extensions":{}}`,
			commentsGraphQLResponse(),
		},
	}
	client := NewClient(
		map[string]string{"c_user": "123", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: transport}),
	)
	client.retryDelay = 0

	// When
	comments, err := Collect(client.ListComments(context.Background(), FeedbackID("36674113645566126")))

	// Then
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if transport.calls != 2 {
		t.Fatalf("RoundTrip calls = %d, want 2", transport.calls)
	}
	if len(comments) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(comments))
	}
	if comments[0].Text != "hello" {
		t.Fatalf("comments[0].Text = %q, want hello", comments[0].Text)
	}
}

func TestCreateComment_sends_create_comment_mutation_when_input_valid(t *testing.T) {
	// Given
	transport := &captureRoundTripper{
		t:    t,
		body: createCommentGraphQLResponse(),
	}
	client := NewClient(
		map[string]string{"c_user": "100087873054623", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithDocIDs(DocIDs{CreateComment: "create-doc"}),
		WithLSD("lsd-token"),
	)
	input := CreateCommentInput{
		FeedbackID: FeedbackID("25024155660614181"),
		Text:       "hello",
		GroupID:    "10081235341999458",
	}

	// When
	comment, err := client.CreateComment(context.Background(), input)

	// Then
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if comment.CommentID != "Y29tbWVudDoyNTAyNDE1NTY2MDYxNDE4MV8yNzA3MDcwMTI0MjYyNjI2OQ==" {
		t.Fatalf("CommentID = %q, want created comment id", comment.CommentID)
	}
	if comment.Text != "hello" {
		t.Fatalf("Text = %q, want hello", comment.Text)
	}
	if transport.req.Header.Get("X-FB-Friendly-Name") != "useCometUFICreateCommentMutation" {
		t.Fatalf("X-FB-Friendly-Name = %q, want create mutation", transport.req.Header.Get("X-FB-Friendly-Name"))
	}
	if transport.req.Header.Get("X-FB-LSD") != "lsd-token" {
		t.Fatalf("X-FB-LSD = %q, want lsd-token", transport.req.Header.Get("X-FB-LSD"))
	}

	form := transport.form()
	if form.Get("doc_id") != "create-doc" {
		t.Fatalf("doc_id = %q, want create-doc", form.Get("doc_id"))
	}
	if form.Get("lsd") != "lsd-token" {
		t.Fatalf("lsd = %q, want lsd-token", form.Get("lsd"))
	}

	variables := decodeVariables(t, form)
	if jsonStr(variables, "feedLocation") != "GROUP" {
		t.Fatalf("feedLocation = %q, want GROUP", jsonStr(variables, "feedLocation"))
	}
	if jsonStr(variables, "groupID") != "10081235341999458" {
		t.Fatalf("groupID = %q, want 10081235341999458", jsonStr(variables, "groupID"))
	}

	mutationInput := jsonMap(variables, "input")
	message := jsonMap(mutationInput, "message")
	if jsonStr(mutationInput, "actor_id") != "100087873054623" {
		t.Fatalf("actor_id = %q, want cookie user id", jsonStr(mutationInput, "actor_id"))
	}
	if jsonStr(mutationInput, "feedback_id") != FeedbackID("25024155660614181") {
		t.Fatalf("feedback_id = %q, want encoded feedback id", jsonStr(mutationInput, "feedback_id"))
	}
	if jsonStr(message, "text") != "hello" {
		t.Fatalf("message.text = %q, want hello", jsonStr(message, "text"))
	}
	if jsonStr(mutationInput, "idempotence_token") == "" {
		t.Fatal("idempotence_token is empty")
	}
	if jsonStr(mutationInput, "session_id") == "" {
		t.Fatal("session_id is empty")
	}
}

func TestDeleteComment_sends_delete_comment_mutation_when_input_valid(t *testing.T) {
	// Given
	transport := &captureRoundTripper{
		t:    t,
		body: deleteCommentGraphQLResponse(),
	}
	client := NewClient(
		map[string]string{"c_user": "100087873054623", "xs": "session"},
		"token",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithDocIDs(DocIDs{DeleteComment: "delete-doc"}),
	)
	input := DeleteCommentInput{
		CommentID: "Y29tbWVudDoyNTAyNDE1NTY2MDYxNDE4MV8yNzA3MDcwMTI0MjYyNjI2OQ==",
	}

	// When
	result, err := client.DeleteComment(context.Background(), input)

	// Then
	if err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	if result.DeletedCommentID != input.CommentID {
		t.Fatalf("DeletedCommentID = %q, want %q", result.DeletedCommentID, input.CommentID)
	}
	if transport.req.Header.Get("X-FB-Friendly-Name") != "useCometUFIDeleteCommentMutation" {
		t.Fatalf("X-FB-Friendly-Name = %q, want delete mutation", transport.req.Header.Get("X-FB-Friendly-Name"))
	}

	form := transport.form()
	if form.Get("doc_id") != "delete-doc" {
		t.Fatalf("doc_id = %q, want delete-doc", form.Get("doc_id"))
	}

	variables := decodeVariables(t, form)
	mutationInput := jsonMap(variables, "input")
	if jsonStr(mutationInput, "actor_id") != "100087873054623" {
		t.Fatalf("actor_id = %q, want cookie user id", jsonStr(mutationInput, "actor_id"))
	}
	if jsonStr(mutationInput, "comment_id") != input.CommentID {
		t.Fatalf("comment_id = %q, want %q", jsonStr(mutationInput, "comment_id"), input.CommentID)
	}
	if jsonStr(mutationInput, "remove_location") != "MENU" {
		t.Fatalf("remove_location = %q, want MENU", jsonStr(mutationInput, "remove_location"))
	}
}

type sequenceRoundTripper struct {
	t         *testing.T
	responses []string
	calls     int
}

func (t *sequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	if t.calls > len(t.responses) {
		t.t.Fatalf("unexpected RoundTrip call %d", t.calls)
	}

	body := t.responses[t.calls-1]
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

func commentsGraphQLResponse() string {
	return `{"data":{"node":{"__typename":"Feedback","comment_rendering_instance_for_feed_location":{"comments":{"edges":[{"node":{"id":"c1","author":{"id":"u1","name":"Alice"},"body":{"text":"hello"},"feedback":{"id":"comment-feedback","expansion_info":{"expansion_token":"token"},"reactors":{"count_reduced":"0"}}}}],"page_info":{"end_cursor":null,"has_next_page":false}}}}}}`
}

func dialogCommentsResponse(hasNext bool) string {
	pageInfo := `{"end_cursor":null,"has_next_page":false}`
	if hasNext {
		pageInfo = `{"end_cursor":"cursor-1","has_next_page":true}`
	}
	return `{"data":{"node_v2":{"id":"story-1","comet_sections":{"feedback":{"story":{"story_ufi_container":{"story":{"feedback_context":{"feedback_target_with_context":{"comment_list_renderer":{"feedback":{"comment_rendering_instance_for_feed_location":{"comments":{"edges":[{"node":{"id":"c-dialog","author":{"id":"u1","name":"Alice"},"body":{"text":"dialog-hello"},"feedback":{"id":"comment-feedback","expansion_info":{"expansion_token":"token"},"reactors":{"count_reduced":"0"}}}}],"page_info":` + pageInfo + `}}}}}}}}}}}}}}`
}

type captureRoundTripper struct {
	t    *testing.T
	body string
	req  *http.Request
}

func (t *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t.req = req
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(t.body)),
		ContentLength: int64(len(t.body)),
		Request:       req,
	}, nil
}

func (t *captureRoundTripper) form() url.Values {
	body, err := io.ReadAll(t.req.Body)
	if err != nil {
		t.t.Fatalf("read request body: %v", err)
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		t.t.Fatalf("parse request form: %v", err)
	}
	return form
}

func decodeVariables(t *testing.T, form url.Values) map[string]any {
	t.Helper()

	var variables map[string]any
	if err := json.Unmarshal([]byte(form.Get("variables")), &variables); err != nil {
		t.Fatalf("decode variables: %v", err)
	}
	return variables
}

func createCommentGraphQLResponse() string {
	return `{"data":{"comment_create":{"feedback_comment_edge":{"node":{"id":"Y29tbWVudDoyNTAyNDE1NTY2MDYxNDE4MV8yNzA3MDcwMTI0MjYyNjI2OQ==","author":{"id":"100087873054623","name":"Ray"},"body":{"text":"hello"},"feedback":{"id":"comment-feedback","expansion_info":{"expansion_token":"reply-token"},"reactors":{"count_reduced":"0"}}}}}}}`
}

func deleteCommentGraphQLResponse() string {
	return `{"data":{"comment_delete":{"deleted_comment_id":"Y29tbWVudDoyNTAyNDE1NTY2MDYxNDE4MV8yNzA3MDcwMTI0MjYyNjI2OQ==","feedback":{"id":"ZmVlZGJhY2s6MjUwMjQxNTU2NjA2MTQxODE=","associated_group":{"id":"10081235341999458"}}}}}`
}
