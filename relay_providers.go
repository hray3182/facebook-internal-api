package fbia

// Relay provider flags captured from facebook.com on 2026-11-11.
//
// Since the 2026-11 doc_id rotation Facebook rejects these queries with
// missing_required_variable_value unless every __relay_internal__pv__ flag
// the web client sends is present. Re-capture with tools/capture when the
// doc_ids rotate again and regenerate this file from graphql-log.json.

// groupFeedRelayProviders are the Relay provider flags GroupsCometFeedRegularStoriesPaginationQuery sends (group feed).
var groupFeedRelayProviders = map[string]any{
	"__relay_internal__pv__CometFeedShareMedia_shouldPrefetchShareImagerelayprovider":           true,
	"__relay_internal__pv__CometFeedStory_enable_post_permalink_white_space_clickrelayprovider": false,
	"__relay_internal__pv__CometFeedStory_enable_reactor_facepilerelayprovider":                 false,
	"__relay_internal__pv__CometFeedStory_enable_social_bubblesrelayprovider":                   false,
	"__relay_internal__pv__CometImmersivePhotoCanUserDisable3DMotionrelayprovider":              false,
	"__relay_internal__pv__CometUFICommentActionLinksRewriteEnabledrelayprovider":               true,
	"__relay_internal__pv__CometUFICommentAutoTranslationTyperelayprovider":                     "AUTO_TRANSLATE",
	"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider":              false,
	"__relay_internal__pv__CometUFIReactionsEnableShortNamerelayprovider":                       false,
	"__relay_internal__pv__CometUFIShareActionMigrationrelayprovider":                           true,
	"__relay_internal__pv__CometUFISingleLineUFIrelayprovider":                                  true,
	"__relay_internal__pv__CometUFI_dedicated_comment_routable_dialog_gkrelayprovider":          true,
	"__relay_internal__pv__FBReelsIFUTileContent_reelsIFUPlayOnHoverrelayprovider":              true,
	"__relay_internal__pv__FBReelsMediaFooter_comet_enable_reels_ads_gkrelayprovider":           true,
	"__relay_internal__pv__FBReels_deprecate_short_form_video_context_gkrelayprovider":          true,
	"__relay_internal__pv__FBReels_enable_view_dubbed_audio_type_gkrelayprovider":               true,
	"__relay_internal__pv__GHLShouldChangeAdIdFieldNamerelayprovider":                           true,
	"__relay_internal__pv__GHLShouldChangeSponsoredDataFieldNamerelayprovider":                  true,
	"__relay_internal__pv__GeminiUFISaveToKnowledgeHubGKrelayprovider":                          false,
	"__relay_internal__pv__GroupsCometGYSJFeedItemHeightrelayprovider":                          206,
	"__relay_internal__pv__IsMergQAPollsrelayprovider":                                          false,
	"__relay_internal__pv__IsWorkUserrelayprovider":                                             false,
	"__relay_internal__pv__ShouldEnableBakedInTextStoriesrelayprovider":                         false,
	"__relay_internal__pv__StoriesShouldEnablePhotosensitiveContentWarningrelayprovider":        false,
	"__relay_internal__pv__StoriesShouldIncludeFbNotesrelayprovider":                            true,
	"__relay_internal__pv__TestPilotShouldIncludeDemoAdUseCaserelayprovider":                    false,
	"__relay_internal__pv__WorkCometIsEmployeeGKProviderrelayprovider":                          false,
	"__relay_internal__pv__relay_provider_comet_ufi_ssr_seo_deferrelayprovider":                 true,
}

// postDialogRelayProviders are the Relay provider flags CometSinglePostDialogContentQuery sends (post dialog).
var postDialogRelayProviders = map[string]any{
	"__relay_internal__pv__CometFeedShareMedia_shouldPrefetchShareImagerelayprovider":           true,
	"__relay_internal__pv__CometFeedStory_enable_post_permalink_white_space_clickrelayprovider": false,
	"__relay_internal__pv__CometFeedStory_enable_reactor_facepilerelayprovider":                 false,
	"__relay_internal__pv__CometFeedStory_enable_social_bubblesrelayprovider":                   false,
	"__relay_internal__pv__CometImmersivePhotoCanUserDisable3DMotionrelayprovider":              false,
	"__relay_internal__pv__CometSinglePostDialogContent_isFloatingCTAEnabledrelayprovider":      false,
	"__relay_internal__pv__CometUFICommentActionLinksRewriteEnabledrelayprovider":               true,
	"__relay_internal__pv__CometUFICommentAutoTranslationTyperelayprovider":                     "AUTO_TRANSLATE",
	"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider":              false,
	"__relay_internal__pv__CometUFIReactionsEnableShortNamerelayprovider":                       false,
	"__relay_internal__pv__CometUFIShareActionMigrationrelayprovider":                           true,
	"__relay_internal__pv__CometUFISingleLineUFIrelayprovider":                                  true,
	"__relay_internal__pv__CometUFI_dedicated_comment_routable_dialog_gkrelayprovider":          true,
	"__relay_internal__pv__FBReelsIFUTileContent_reelsIFUPlayOnHoverrelayprovider":              true,
	"__relay_internal__pv__FBReelsMediaFooter_comet_enable_reels_ads_gkrelayprovider":           true,
	"__relay_internal__pv__FBReels_deprecate_short_form_video_context_gkrelayprovider":          true,
	"__relay_internal__pv__FBReels_enable_view_dubbed_audio_type_gkrelayprovider":               true,
	"__relay_internal__pv__GHLShouldChangeAdIdFieldNamerelayprovider":                           true,
	"__relay_internal__pv__GHLShouldChangeSponsoredDataFieldNamerelayprovider":                  true,
	"__relay_internal__pv__GeminiUFISaveToKnowledgeHubGKrelayprovider":                          false,
	"__relay_internal__pv__GroupsCometGYSJFeedItemHeightrelayprovider":                          206,
	"__relay_internal__pv__IsMergQAPollsrelayprovider":                                          false,
	"__relay_internal__pv__IsWorkUserrelayprovider":                                             false,
	"__relay_internal__pv__ProfileCometSeoIsViewerLoggedOutrelayprovider":                       false,
	"__relay_internal__pv__ShouldEnableBakedInTextStoriesrelayprovider":                         false,
	"__relay_internal__pv__StoriesShouldEnablePhotosensitiveContentWarningrelayprovider":        false,
	"__relay_internal__pv__StoriesShouldIncludeFbNotesrelayprovider":                            true,
	"__relay_internal__pv__TestPilotShouldIncludeDemoAdUseCaserelayprovider":                    false,
	"__relay_internal__pv__WorkCometIsEmployeeGKProviderrelayprovider":                          false,
	"__relay_internal__pv__relay_provider_comet_ufi_ssr_seo_deferrelayprovider":                 true,
}

// commentsListRelayProviders are the Relay provider flags CommentsListComponentsPaginationQuery sends (comment list).
var commentsListRelayProviders = map[string]any{
	"__relay_internal__pv__CometUFICommentActionLinksRewriteEnabledrelayprovider":  true,
	"__relay_internal__pv__CometUFICommentAutoTranslationTyperelayprovider":        "AUTO_TRANSLATE",
	"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider": false,
	"__relay_internal__pv__IsWorkUserrelayprovider":                                false,
}

// withRelayProviders returns vars with the provider flags merged in.
// Keys already present in vars win.
func withRelayProviders(vars, providers map[string]any) map[string]any {
	for k, v := range providers {
		if _, ok := vars[k]; !ok {
			vars[k] = v
		}
	}
	return vars
}
