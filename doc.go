// Package fbia is a Go client for Facebook's internal (unofficial) GraphQL API.
//
// It wraps the reverse-engineered endpoints that the Facebook web app uses,
// providing typed access to posts, comments, replies, and images.
//
// Authentication requires valid Facebook session cookies and an fb_dtsg token
// extracted from a logged-in browser session.
//
//	client := fbia.NewClient(
//	    map[string]string{"c_user": "...", "xs": "...", "datr": "..."},
//	    "fb_dtsg_token",
//	)
//
//	// Range over comments (FeedbackID — same as fbGraber / joaimy)
//	for comment, err := range client.ListComments(ctx, fbia.FeedbackID("POST_ID")) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Println(comment.Text)
//	}
//
//	// Collect into a slice
//	posts, err := fbia.Collect(client.ListPosts(ctx, "USER_ID"))
package fbia
