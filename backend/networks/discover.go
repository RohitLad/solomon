package networks

// Native ("Outside Solomon") post discovery: list recent posts made OUTSIDE
// the app on a connected account, so analytics covers the whole account —
// the Buffer/Hootsuite standard (30-day backfill, ~100/day caps, metrics
// frozen ~20 days after publishing).
//
// Every network is best-effort: missing IDs/scopes return a clear error and
// the caller skips that account with a note (same pattern as FetchStats).
// Demo (or empty) tokens return deterministic pseudo-posts, so the whole
// flow works with zero credentials.

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"solomon/backend/models"
)

// ExternalCandidate is one native post found on an account.
type ExternalCandidate struct {
	NetworkPostID string
	Text          string
	Permalink     string
	PublishedAt   time.Time
}

// ListRecentPosts returns up to `limit` native posts newer than `since`.
func ListRecentPosts(network models.Network, accessToken, extra, externalID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if accessToken == "" || (len(accessToken) >= 5 && accessToken[:5] == "demo-") {
		return demoExternalPosts(externalID+string(network), since, limit), nil
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	switch network {
	case models.NetworkFacebook:
		return listFacebookPosts(accessToken, acctID(extra, externalID, "page_id"), since, limit)
	case models.NetworkInstagram:
		return listInstagramMedia(accessToken, acctID(extra, externalID, "ig_user_id"), since, limit)
	case models.NetworkTwitter:
		return listTweets(accessToken, externalID, since, limit)
	case models.NetworkYouTube:
		return listYouTubeVideos(accessToken, extraField(extra, "channel_id"), since, limit)
	case models.NetworkTikTok:
		return nil, fmt.Errorf("tiktok video.list needs an audited app + video.list scope — native discovery skipped")
	case models.NetworkLinkedIn:
		return listLinkedInPosts(accessToken, extraField(extra, "author_urn"), since, limit)
	case models.NetworkPinterest:
		return listPins(accessToken, extraField(extra, "board_id"), since, limit)
	default:
		return nil, fmt.Errorf("unknown network %s", network)
	}
}

func acctID(extra, externalID, key string) string {
	if v := extraField(extra, key); v != "" {
		return v
	}
	return externalID
}

// demoExternalPosts returns stable pseudo-posts (fnv-seeded, like demoStats).
// Three fall inside the 30-day backfill window; the fourth (31d) exercises
// the cutoff and must be filtered by the caller via `since`.
func demoExternalPosts(seed string, since time.Time, limit int) []ExternalCandidate {
	ages := []time.Duration{0, 49 * time.Hour, 98 * time.Hour, 31 * 24 * time.Hour}
	texts := []string{
		"Posted natively while testing the waters",
		"A native photo post from the phone",
		"Weekend update, posted directly",
		"Ancient history (outside backfill)",
	}
	var out []ExternalCandidate
	for i, age := range ages {
		if len(out) >= limit {
			break
		}
		at := time.Now().Add(-age).Truncate(time.Second)
		if at.Before(since) {
			continue
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(fmt.Sprintf("%s|ext|%d", seed, i)))
		out = append(out, ExternalCandidate{
			NetworkPostID: fmt.Sprintf("ext_%x_%d", h.Sum64()&0xffff, i),
			Text:          texts[i%len(texts)],
			PublishedAt:   at,
		})
	}
	return out
}

// getList GETs an endpoint and returns the {data:[...]} array (Meta-style).
func getList(endpoint string, params url.Values, token string) ([]any, error) {
	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", firstLine(string(raw), 200))
	}
	var out struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		return nil, fmt.Errorf("no data array in response")
	}
	return out.Data, nil
}

// authedGET performs a Bearer GET and returns the raw body.
func authedGET(endpoint string, params url.Values, token string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, nil
}

// getObject GETs an endpoint returning a single JSON object (X user,
// LinkedIn /me, Pinterest user_account).
func getObject(endpoint string, params url.Values, token string) (map[string]any, error) {
	raw, code, err := authedGET(endpoint, params, token)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("%s", firstLine(string(raw), 200))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// getItems GETs an endpoint returning an {items:[...]} array
// (YouTube Data API, Pinterest v5).
func getItems(endpoint string, params url.Values, token string) ([]any, error) {
	raw, code, err := authedGET(endpoint, params, token)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("%s", firstLine(string(raw), 200))
	}
	var out struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		return nil, fmt.Errorf("no items array in response")
	}
	return out.Items, nil
}

func firstLine(s string, n int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func strVal(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func parseTime(v any) time.Time {
	s, _ := v.(string)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05+0000"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func listFacebookPosts(token, pageID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if pageID == "" {
		return nil, fmt.Errorf("facebook: page_id (Extra) or external ID required to list posts")
	}
	items, err := getList("https://graph.facebook.com/v21.0/"+pageID+"/posts",
		url.Values{
			"fields":       {"id,message,created_time,permalink_url"},
			"since":        {fmt.Sprint(since.Unix())},
			"limit":        {fmt.Sprint(limit)},
			"access_token": {token},
		}, "")
	if err != nil {
		return nil, fmt.Errorf("facebook posts: %v (needs pages_read_engagement)", err)
	}
	var out []ExternalCandidate
	for _, d := range items {
		m, _ := d.(map[string]any)
		at := parseTime(m["created_time"])
		if at.IsZero() || at.Before(since) {
			continue
		}
		out = append(out, ExternalCandidate{
			NetworkPostID: strVal(m, "id"), Text: strVal(m, "message"),
			Permalink: strVal(m, "permalink_url"), PublishedAt: at,
		})
	}
	return out, nil
}

func listInstagramMedia(token, igID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if igID == "" {
		return nil, fmt.Errorf("instagram: ig_user_id (Extra) required to list media")
	}
	items, err := getList("https://graph.facebook.com/v21.0/"+igID+"/media",
		url.Values{
			"fields":       {"id,caption,timestamp,permalink"},
			"since":        {fmt.Sprint(since.Unix())},
			"limit":        {fmt.Sprint(limit)},
			"access_token": {token},
		}, "")
	if err != nil {
		return nil, fmt.Errorf("instagram media: %v", err)
	}
	var out []ExternalCandidate
	for _, d := range items {
		m, _ := d.(map[string]any)
		at := parseTime(m["timestamp"])
		if at.IsZero() || at.Before(since) {
			continue
		}
		out = append(out, ExternalCandidate{
			NetworkPostID: strVal(m, "id"), Text: strVal(m, "caption"),
			Permalink: strVal(m, "permalink"), PublishedAt: at,
		})
	}
	return out, nil
}

func listTweets(token, userID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if userID == "" {
		return nil, fmt.Errorf("twitter/X: external user ID required (put the numeric user ID in External ID)")
	}
	n := limit
	if n > 100 {
		n = 100
	}
	items, err := getList("https://api.twitter.com/2/users/"+userID+"/tweets",
		url.Values{
			"max_results":  {fmt.Sprint(n)},
			"start_time":   {since.UTC().Format(time.RFC3339)},
			"tweet.fields": {"created_at"},
			"exclude":      {"replies,retweets"},
		}, token)
	if err != nil {
		return nil, fmt.Errorf("twitter/X timeline: %v", err)
	}
	var out []ExternalCandidate
	for _, d := range items {
		m, _ := d.(map[string]any)
		id := strVal(m, "id")
		at := parseTime(m["created_at"])
		if id == "" || at.IsZero() {
			continue
		}
		out = append(out, ExternalCandidate{
			NetworkPostID: id, Text: strVal(m, "text"),
			Permalink: "https://x.com/i/status/" + id, PublishedAt: at,
		})
	}
	return out, nil
}

func listYouTubeVideos(token, channelID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	// Resolve the uploads playlist (own channel when no ID configured).
	playlist := ""
	if channelID == "" {
		ch, err := getItems("https://www.googleapis.com/youtube/v3/channels",
			url.Values{"part": {"contentDetails"}, "mine": {"true"}}, token)
		if err != nil || len(ch) == 0 {
			return nil, fmt.Errorf("youtube channels: %v", err)
		}
		if cd, ok := ch[0].(map[string]any)["contentDetails"].(map[string]any); ok {
			if rp, ok := cd["relatedPlaylists"].(map[string]any); ok {
				playlist, _ = rp["uploads"].(string)
			}
		}
	}
	if playlist == "" && channelID != "" {
		playlist = "UU" + strings.TrimPrefix(channelID, "UC")
	}
	if playlist == "" {
		return nil, fmt.Errorf("youtube: uploads playlist not found (set channel_id in Extra or grant youtube.readonly)")
	}
	n := limit
	if n > 50 {
		n = 50
	}
	items, err := getItems("https://www.googleapis.com/youtube/v3/playlistItems",
		url.Values{"part": {"contentDetails"}, "playlistId": {playlist}, "maxResults": {fmt.Sprint(n)}}, token)
	if err != nil {
		return nil, fmt.Errorf("youtube playlist: %v", err)
	}
	var ids []string
	atByID := map[string]time.Time{}
	for _, d := range items {
		m, _ := d.(map[string]any)
		cd, _ := m["contentDetails"].(map[string]any)
		vid := str(cd, "videoId")
		at := parseTime(cd["videoPublishedAt"])
		if vid == "" || at.IsZero() || at.Before(since) {
			continue
		}
		ids = append(ids, vid)
		atByID[vid] = at
	}
	if len(ids) == 0 {
		return nil, nil
	}
	// One batched call for titles (quota-cheap).
	vids, err := getItems("https://www.googleapis.com/youtube/v3/videos",
		url.Values{"part": {"snippet"}, "id": {strings.Join(ids, ",")}}, token)
	if err != nil {
		return nil, fmt.Errorf("youtube videos: %v", err)
	}
	var out []ExternalCandidate
	for _, d := range vids {
		m, _ := d.(map[string]any)
		id := strVal(m, "id")
		at, ok := atByID[id]
		if !ok {
			continue
		}
		title := ""
		if sn, ok := m["snippet"].(map[string]any); ok {
			title = str(sn, "title")
		}
		out = append(out, ExternalCandidate{
			NetworkPostID: id, Text: title,
			Permalink: "https://youtu.be/" + id, PublishedAt: at,
		})
	}
	return out, nil
}

func listLinkedInPosts(token, authorURN string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if authorURN == "" {
		return nil, fmt.Errorf("linkedin: author_urn (Extra) required to list posts")
	}
	n := limit
	if n > 50 {
		n = 50
	}
	req, err := http.NewRequest("GET",
		"https://api.linkedin.com/v2/ugcPosts?q=authors&authors=List("+url.QueryEscape(authorURN)+")&count="+fmt.Sprint(n)+"&sortBy=CREATED", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("LinkedIn-Version", "202401")
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("linkedin ugcPosts: %s (needs r_organization_social / w_member_social)", firstLine(string(raw), 200))
	}
	var out struct {
		Elements []any `json:"elements"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	var res []ExternalCandidate
	for _, d := range out.Elements {
		m, _ := d.(map[string]any)
		id := strVal(m, "id")
		var at time.Time
		if created, ok := m["created"].(map[string]any); ok {
			if ms, ok := created["time"].(float64); ok {
				at = time.UnixMilli(int64(ms))
			}
		}
		if id == "" || at.IsZero() || at.Before(since) {
			continue
		}
		text := ""
		if sp, ok := m["specificContent"].(map[string]any); ok {
			if sc, ok := sp["com.linkedin.ugc.ShareContent"].(map[string]any); ok {
				if sh, ok := sc["shareCommentary"].(map[string]any); ok {
					text = str(sh, "text")
				}
			}
		}
		res = append(res, ExternalCandidate{NetworkPostID: id, Text: text, PublishedAt: at})
	}
	return res, nil
}

func listPins(token, boardID string, since time.Time, limit int) ([]ExternalCandidate, error) {
	if boardID == "" {
		return nil, fmt.Errorf("pinterest: board_id (Extra) required to list pins")
	}
	n := limit
	if n > 50 {
		n = 50
	}
	items, err := getItems("https://api.pinterest.com/v5/boards/"+boardID+"/pins",
		url.Values{"page_size": {fmt.Sprint(n)}}, token)
	if err != nil {
		return nil, fmt.Errorf("pinterest pins: %v", err)
	}
	var out []ExternalCandidate
	for _, d := range items {
		m, _ := d.(map[string]any)
		id := strVal(m, "id")
		at := parseTime(m["created_at"])
		if id == "" {
			continue
		}
		if !at.IsZero() && at.Before(since) {
			continue
		}
		if at.IsZero() {
			at = time.Now()
		}
		out = append(out, ExternalCandidate{
			NetworkPostID: id, Text: strVal(m, "description"),
			Permalink: strVal(m, "link"), PublishedAt: at,
		})
	}
	return out, nil
}
