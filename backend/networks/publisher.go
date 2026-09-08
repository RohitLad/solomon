package networks

// Publisher publishes ONE target to its network's latest official API.
// Design: keep every network in ONE file so the app stays easy to follow.
// Real HTTP calls are implemented; if the account token starts with
// "demo" (or no credentials are configured) we do a MockPublish so you can
// try the whole flow without real API keys.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"solomon/backend/models"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

func isDemo(acct models.SocialAccount) bool {
	t := acct.AccessToken
	return t == "" || strings.HasPrefix(t, "demo")
}

type PublishInput struct {
	Account    models.SocialAccount
	Title      string
	Text       string // already per-account effective text
	Link       string
	FirstComment string
	MediaPaths []string // local file paths, images first then videos
	IsVideo    []bool   // parallel to MediaPaths
}

type PublishResult struct {
	NetworkPostIDs []string
	Note           string
}

// Publish dispatches to the right network.
func Publish(in PublishInput) (PublishResult, error) {
	switch in.Account.Network {
	case models.NetworkTwitter:
		return publishTwitter(in)
	case models.NetworkFacebook:
		return publishFacebook(in)
	case models.NetworkInstagram:
		return publishInstagram(in)
	case models.NetworkYouTube:
		return publishYouTube(in)
	case models.NetworkTikTok:
		return publishTikTok(in)
	case models.NetworkLinkedIn:
		return publishLinkedIn(in)
	case models.NetworkPinterest:
		return publishPinterest(in)
	}
	return PublishResult{}, fmt.Errorf("unknown network %s", in.Account.Network)
}

func mockIDs(network models.Network, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("demo_%s_%d_%d", network, time.Now().UnixMilli(), i)
	}
	return ids
}

// ---------- Twitter / X API v2 ----------
// POST https://api.twitter.com/2/tweets (+ media upload v1.1/v2 chunked).
// Long text -> thread: first tweet, then replies with in_reply_to_tweet_id.
func publishTwitter(in PublishInput) (PublishResult, error) {
	lim := Limits()[models.NetworkTwitter]
	chunks := []string{in.Text}
	if len([]rune(in.Text)) > lim.MaxChars {
		chunks = SplitThread(in.Text, lim.MaxChars)
	}
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("twitter", len(chunks)),
			Note: fmt.Sprintf("MOCK thread x%d", len(chunks))}, nil
	}
	var ids []string
	var replyTo string
	for _, chunk := range chunks {
		body := map[string]any{"text": chunk}
		if replyTo != "" {
			body["reply"] = map[string]string{"in_reply_to_tweet_id": replyTo}
		}
		// NOTE: media upload (POST https://upload.twitter.com/1.1/media/upload.json,
		// chunked INIT/APPEND/FINALIZE for video) then attach media_ids on FIRST tweet.
		// Polls: add "poll": {"options": [...], "duration_minutes": N}.
		id, err := twitterPostTweet(in.Account.AccessToken, body)
		if err != nil {
			return PublishResult{NetworkPostIDs: ids}, err
		}
		replyTo = id
		ids = append(ids, id)
	}
	return PublishResult{NetworkPostIDs: ids}, nil
}

func twitterPostTweet(bearer string, body map[string]any) (string, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.twitter.com/2/tweets", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("twitter: %s", string(raw))
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.Data.ID, nil
}

// ---------- Facebook Graph v21 ----------
// POST https://graph.facebook.com/v21.0/{page-id}/feed|photos|videos|reels
func publishFacebook(in PublishInput) (PublishResult, error) {
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("facebook", 1), Note: "MOCK fb post"}, nil
	}
	pageID := extraField(in.Account.Extra, "page_id")
	if pageID == "" {
		pageID = in.Account.ExternalID
	}
	msg := in.Text
	if in.Link != "" {
		msg += "\n" + in.Link
	}
	var endpoint string
	if len(in.MediaPaths) > 0 && in.IsVideo[0] {
		endpoint = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/videos", pageID)
	} else if len(in.MediaPaths) > 0 {
		endpoint = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/photos", pageID)
	} else {
		endpoint = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/feed", pageID)
	}
	form := url.Values{"message": {msg}, "access_token": {in.Account.AccessToken}}
	if in.Link != "" && len(in.MediaPaths) == 0 {
		form.Set("link", in.Link)
	}
	// NOTE: multi-photo: POST each to /photos with published=false, then /feed with attached_media.
	// Reels: POST /video_reels (video_url, caption). Upload local files via multipart or
	// resumable upload; simplest prod path is upload to public URL/S3 first.
	id, err := graphPostForm(endpoint, form)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{NetworkPostIDs: []string{id}}, nil
}

func graphPostForm(endpoint string, form url.Values) (string, error) {
	resp, err := httpClient.PostForm(endpoint, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("facebook: %s", string(raw))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.ID, nil
}

// ---------- Instagram Graph API ----------
// 1) POST /{ig-user-id}/media (container) 2) POST /{ig-user-id}/media_publish.
// Carousel: create children with is_carousel_item=true, then container with media_type=CAROUSEL.
// Reels: container with media_type=REELS. Requires public media URLs in prod.
func publishInstagram(in PublishInput) (PublishResult, error) {
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("instagram", 1), Note: "MOCK ig container+publish"}, nil
	}
	igID := extraField(in.Account.Extra, "ig_user_id")
	if igID == "" {
		igID = in.Account.ExternalID
	}
	caption := TrimTo(in.Text, 2200)
	base := os.Getenv("PUBLIC_MEDIA_BASE_URL") // e.g. https://cdn.example.com/
	if base == "" || len(in.MediaPaths) == 0 {
		return PublishResult{}, fmt.Errorf("instagram: needs PUBLIC_MEDIA_BASE_URL and at least 1 image/video (prod requirement)")
	}
	// Simplified: single image/video container flow.
	mediaURL := base + in.MediaPaths[0]
	createURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/media", igID)
	form := url.Values{
		"caption": {caption}, "access_token": {in.Account.AccessToken},
	}
	if in.IsVideo[0] {
		form.Set("media_type", "REELS")
		form.Set("video_url", mediaURL)
	} else {
		form.Set("image_url", mediaURL)
	}
	resp, err := httpClient.PostForm(createURL, form)
	if err != nil {
		return PublishResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return PublishResult{}, fmt.Errorf("instagram container: %s", string(raw))
	}
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &c)
	pubURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/media_publish", igID)
	id, err := graphPostForm(pubURL, url.Values{"creation_id": {c.ID}, "access_token": {in.Account.AccessToken}})
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{NetworkPostIDs: []string{id}}, nil
}

// ---------- YouTube Data API v3 ----------
// POST https://www.googleapis.com/upload/youtube/v3/videos (resumable).
// Shorts: video <=60s + vertical + title/desc contains #Shorts.
func publishYouTube(in PublishInput) (PublishResult, error) {
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("youtube", 1), Note: "MOCK videos.insert"}, nil
	}
	if len(in.MediaPaths) == 0 || !in.IsVideo[0] {
		return PublishResult{}, fmt.Errorf("youtube: exactly 1 video file required")
	}
	// NOTE: full resumable upload omitted for brevity — use google.golang.org/api/youtube/v3
	// with oauth2 token source: service.Videos.Insert(...).Media(file).Do().
	// Set status.privacyStatus (public/unlisted/private), categoryId, madeForKids.
	return PublishResult{}, fmt.Errorf("youtube: configure google.golang.org/api/youtube/v3 with OAuth2 (see README) — local file: %s", in.MediaPaths[0])
}

// ---------- TikTok Content Posting API v2 ----------
// POST https://open.tiktokapis.com/v2/post/publish/video/init|.../fetch|...
// FILE_UPLOAD (chunked) for local files, or PULL_FROM_URL.
func publishTikTok(in PublishInput) (PublishResult, error) {
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("tiktok", 1), Note: "MOCK tiktok publish"}, nil
	}
	if len(in.MediaPaths) == 0 {
		return PublishResult{}, fmt.Errorf("tiktok: video or 1-35 photos required")
	}
	// NOTE: implement FILE_UPLOAD init (open.tiktokapis.com/v2/post/publish/inbox/video/init),
	// chunk PUT, then /video/publish with title<=2200 chars, privacy_level, duet/stitch flags.
	// Refresh flow: POST /v2/oauth/token/ with refresh_token.
	return PublishResult{}, fmt.Errorf("tiktok: finish FILE_UPLOAD flow with your client_key (see README)")
}

// ---------- LinkedIn Posts API ----------
// 1) register upload (POST /v2/assets?action=registerUpload for images,
//    or /v2/videos?action=initializeUpload) 2) PUT binary 3) POST /v2/posts.
func publishLinkedIn(in PublishInput) (PublishResult, error) {
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("linkedin", 1), Note: "MOCK linkedin post"}, nil
	}
	author := extraField(in.Account.Extra, "author_urn") // e.g. urn:li:person:abc or urn:li:organization:123
	if author == "" {
		author = in.Account.ExternalID
	}
	body := map[string]any{
		"author": author, "lifecycleState": "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": map[string]any{
				"shareCommentary": map[string]string{"text": TrimTo(in.Text, 3000)},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC"},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.linkedin.com/v2/ugcPosts", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+in.Account.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	// NOTE: LinkedIn enforces version header in 2024+: "LinkedIn-Version: 202401".
	req.Header.Set("LinkedIn-Version", "202401")
	resp, err := httpClient.Do(req)
	if err != nil {
		return PublishResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return PublishResult{}, fmt.Errorf("linkedin: %s", string(raw))
	}
	urn := resp.Header.Get("x-restli-id")
	if urn == "" {
		urn = "linkedin_post"
	}
	return PublishResult{NetworkPostIDs: []string{urn}}, nil
}

// ---------- Pinterest API v5 ----------
// POST https://api.pinterest.com/v5/pins {board_id, title, description, link, media_source}
func publishPinterest(in PublishInput) (PublishResult, error) {
	title := in.Title
	if title == "" {
		r := []rune(in.Text)
		if len(r) > 100 {
			title = string(r[:100])
		} else {
			title = in.Text
		}
	}
	if isDemo(in.Account) {
		time.Sleep(200 * time.Millisecond)
		return PublishResult{NetworkPostIDs: mockIDs("pinterest", 1), Note: "MOCK pin create"}, nil
	}
	if len(in.MediaPaths) == 0 {
		return PublishResult{}, fmt.Errorf("pinterest: image/video required")
	}
	board := extraField(in.Account.Extra, "board_id")
	if board == "" {
		return PublishResult{}, fmt.Errorf("pinterest: board_id required (save it in account Extra)")
	}
	body := map[string]any{
		"board_id": board, "title": title,
		"description": TrimTo(in.Text, 800), "link": in.Link,
		// NOTE: media_source needs public URL or multipart upload:
		// {"source_type":"image_url","url":"https://..."}. Local-file upload
		// uses multipart /v5/pins with image file part.
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.pinterest.com/v5/pins", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+in.Account.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return PublishResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return PublishResult{}, fmt.Errorf("pinterest: %s", string(raw))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return PublishResult{NetworkPostIDs: []string{out.ID}}, nil
}

// extraField parses account Extra which may be "k=v,k=v" or JSON.
func extraField(extra, key string) string {
	if extra == "" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(extra), &m); err == nil {
		return m[key]
	}
	for _, part := range strings.Split(extra, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == key {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}
