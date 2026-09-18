package fbia

import (
	"errors"
	"testing"
)

func TestToggleCommentingPreservesFacebookErrors(t *testing.T) {
	for _, key := range []string{"feedback_disable_commenting", "feedback_enable_commenting"} {
		for _, tc := range []struct {
			name, body string
			code       int
		}{
			{"top-level", `for (;;);{"error":1357004,"errorSummary":"Session failed","errorDescription":"Please reopen the browser"}`, 1357004},
			{"graphql", `{"errors":[{"code":1675012,"message":"Invalid variables"}]}`, 1675012},
			{"missing-block", `{"data":{}}`, 0},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				status, err := parseToggleCommentingResponse(tc.body, key)
				if status != nil || err == nil {
					t.Fatalf("status=%v err=%v; want failure", status, err)
				}
				if tc.code == 0 {
					if !errors.Is(err, ErrGraphQLResponse) {
						t.Fatalf("err=%v; want ErrGraphQLResponse", err)
					}
					return
				}
				var ge *GraphQLError
				if !errors.As(err, &ge) || ge.Code != tc.code {
					t.Fatalf("err=%v; want GraphQLError code %d", err, tc.code)
				}
				if tc.name == "top-level" && (ge.Summary != "Session failed" || ge.Description != "Please reopen the browser") {
					t.Fatalf("lost error details: %+v", ge)
				}
			})
		}
	}
}
