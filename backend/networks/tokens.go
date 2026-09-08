package networks

// Token auto-refresh per network (latest OAuth2 refresh flows, 2025-26).
// Demo tokens ("demo-*") just get their expiry extended — no network call.
// Real tokens use each provider's refresh grant with the client id/secret
// from env (see backend/.env.example).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type RefreshResult struct {
	AccessToken  string
	RefreshToken string // empty = unchanged
	ExpiresIn    time.Duration
}

func envOf(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func postForm(endpoint string, form url.Values) (map[string]any, error) {
	resp, err := httpClient.PostForm(endpoint, form)
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

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func numSecs(m map[string]any, keys ...string) time.Duration {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return time.Duration(v) * time.Second
		case string:
			var n float64
			if _, err := fmt.Sscanf(v, "%f", &n); err == nil {
				return time.Duration(n) * time.Second
			}
		}
	}
	return 0
}

// RefreshToken exchanges a refresh_token (or FB short-lived token) for new tokens.
func RefreshToken(network, accessToken, refreshToken, extra string) (RefreshResult, error) {
	if accessToken == "" || strings.HasPrefix(accessToken, "demo") {
		return RefreshResult{AccessToken: accessToken, ExpiresIn: 60 * 24 * time.Hour}, nil
	}
	switch network {
	case "twitter":
		out, err := postForm("https://api.twitter.com/2/oauth2/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
			"client_id": {envOf("TWITTER_CLIENT_ID")},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("twitter refresh: %v", err)
		}
		return RefreshResult{AccessToken: str(out, "access_token"), RefreshToken: str(out, "refresh_token"), ExpiresIn: numSecs(out, "expires_in")}, nil
	case "facebook", "instagram":
		// long-lived (~60d) page/user token exchange
		out, err := getJSON("https://graph.facebook.com/v21.0/oauth/access_token", url.Values{
			"grant_type": {"fb_exchange_token"}, "client_id": {envOf("FB_APP_ID")},
			"client_secret": {envOf("FB_APP_SECRET")}, "fb_exchange_token": {accessToken},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("facebook refresh: %v", err)
		}
		return RefreshResult{AccessToken: str(out, "access_token"), ExpiresIn: numSecs(out, "expires_in")}, nil
	case "youtube":
		out, err := postForm("https://oauth2.googleapis.com/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
			"client_id": {envOf("GOOGLE_CLIENT_ID")}, "client_secret": {envOf("GOOGLE_CLIENT_SECRET")},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("google refresh: %v", err)
		}
		return RefreshResult{AccessToken: str(out, "access_token"), ExpiresIn: numSecs(out, "expires_in")}, nil
	case "tiktok":
		out, err := postForm("https://open.tiktokapis.com/v2/oauth/token/", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
			"client_key": {envOf("TIKTOK_CLIENT_KEY")}, "client_secret": {envOf("TIKTOK_CLIENT_SECRET")},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("tiktok refresh: %v", err)
		}
		return RefreshResult{
			AccessToken: str(out, "access_token"), RefreshToken: str(out, "refresh_token"),
			ExpiresIn: numSecs(out, "expires_in"),
		}, nil
	case "linkedin":
		out, err := postForm("https://www.linkedin.com/oauth/v2/accessToken", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
			"client_id": {envOf("LINKEDIN_CLIENT_ID")}, "client_secret": {envOf("LINKEDIN_CLIENT_SECRET")},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("linkedin refresh: %v (note: programmatic refresh needs an approved partner app)", err)
		}
		return RefreshResult{AccessToken: str(out, "access_token"), ExpiresIn: numSecs(out, "expires_in")}, nil
	case "pinterest":
		out, err := postForm("https://api.pinterest.com/v5/oauth/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		})
		if err != nil {
			return RefreshResult{}, fmt.Errorf("pinterest refresh: %v", err)
		}
		return RefreshResult{AccessToken: str(out, "access_token"), ExpiresIn: numSecs(out, "expires_in")}, nil
	}
	return RefreshResult{}, fmt.Errorf("unknown network %s", network)
}
