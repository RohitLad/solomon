// Package models holds every GORM/SQLite entity, one file per model:
//
//	network.go        Network enum (7 supported social networks)
//	social_account.go SocialAccount (one connected profile/page/channel)
//	media.go          MediaType + MediaAsset (uploaded files attached to posts)
//	post.go           PostStatus + Post (master post) + TargetStatus + PostTarget
//	analytics.go      AnalyticsSnapshot (one metrics pull per target)
//	external.go       ExternalPost (native posts discovered outside the app)
//	evergreen.go      EvergreenRule (recycle-a-pool schedule)
//
// All IDs are UUID strings (see the BeforeCreate hooks). Tokens live only on
// SocialAccount and are stripped in every JSON serializer in handlers/.
package models
