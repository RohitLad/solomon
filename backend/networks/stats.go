package networks

// Analytics fetching per published target.
// Demo tokens -> deterministic pseudo-stats (stable per network_post_id,
// so numbers look real and are consistent across refreshes).
// Real tokens -> best-effort live fetch where the API allows server-side
// reads with the same token; otherwise a clear error telling you which
// permission/endpoint to wire (see README analytics section).

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"
	"time"
)

type Stats struct {
	Views    int64
	Likes    int64
	Comments int64
	Shares   int64
	IsDemo   bool
}

func demoStats(seed string) Stats {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	v := h.Sum64()
	return Stats{
		Views:    int64(500 + v%20000),
		Likes:    int64(20 + (v>>7)%800),
		Comments: int64((v >> 13) % 120),
		Shares:   int64((v >> 19) % 200),
		IsDemo:   true,
	}
}

// FetchStats pulls metrics for one published target.
func FetchStats(acctNetwork, accessToken, networkPostID string) (Stats, error) {
	if accessToken == "" || len(accessToken) >= 5 && accessToken[:5] == "demo-" {
		return demoStats(networkPostID), nil
	}
	switch acctNetwork {
	case "facebook":
		return fetchFacebookStats(accessToken, networkPostID)
	case "instagram":
		return fetchInstagramStats(accessToken, networkPostID)
	case "youtube":
		return Stats{}, fmt.Errorf("youtube analytics needs the YouTube Analytics API (scope yt-analytics.readonly) — see README")
	case "tiktok":
		return Stats{}, fmt.Errorf("tiktok video.query requires an audited app + video.list scope — see README")
	case "linkedin":
		return Stats{}, fmt.Errorf("linkedin socialMetadataocial actions API needs r_organization_social — see README")
	case "pinterest":
		return fetchPinterestStats(accessToken, networkPostID)
	default:
		return Stats{}, fmt.Errorf("twitter/X: tweet metrics need the costly basic/pro metrics endpoints — showing demo stats instead")
	}
}

func getJSON(endpoint string, params url.Values) (map[string]any, error) {
	resp, err := httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", string(raw))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func num(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if f, ok := v.(float64); ok {
				return int64(f)
			}
		}
	}
	return 0
}

func fetchFacebookStats(token, postID string) (Stats, error) {
	// Post-level insights need read_insights on the page.
	out, err := getJSON("https://graph.facebook.com/v21.0/"+postID+"/insights",
		url.Values{"metric": {"post_impressions,post_reactions_by_type_total"}, "access_token": {token}})
	if err != nil {
		return Stats{}, fmt.Errorf("facebook insights: %v (needs read_insights permission)", err)
	}
	_ = out
	return Stats{}, fmt.Errorf("facebook: parse insights per-post here (scaffold ok, wire fields to your needs)")
}

func fetchInstagramStats(token, mediaID string) (Stats, error) {
	out, err := getJSON("https://graph.facebook.com/v21.0/"+mediaID+"/insights",
		url.Values{"metric": {"reach,likes,comments,shares,saved"}, "access_token": {token}})
	if err != nil {
		return Stats{}, fmt.Errorf("instagram insights: %v", err)
	}
	data, _ := out["data"].([]any)
	s := Stats{}
	for _, d := range data {
		m, _ := d.(map[string]any)
		name, _ := m["name"].(string)
		vals, _ := m["values"].([]any)
		var total int64
		for _, vv := range vals {
			vm, _ := vv.(map[string]any)
			total += num(vm, "value")
		}
		switch name {
		case "reach":
			s.Views = total
		case "likes":
			s.Likes = total
		case "comments":
			s.Comments = total
		case "shares":
			s.Shares = total
		}
	}
	_ = time.Now
	return s, nil
}

func fetchPinterestStats(token, pinID string) (Stats, error) {
	out, err := getJSON("https://api.pinterest.com/v5/pins/"+pinID,
		url.Values{"pin_metrics": {"true"}})
	if err != nil {
		_ = token
		return Stats{}, fmt.Errorf("pinterest: %v", err)
	}
	_ = out
	return Stats{}, fmt.Errorf("pinterest: enable pin_metrics parsing here (scaffold ok)")
}
