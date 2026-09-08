package networks

// OAuth / auth-URL helpers for connecting accounts.
// Each network uses OAuth2 (or OAuth 2 + PKCE). We only GENERATE the
// consent URL here; the callback exchange happens provider-by-provider
// (see README for the exact scopes & redirect URIs).
// Latest docs (2025-26): X OAuth 2.0 PKCE | FB Login | IG via FB Login with
// instagram_basic+instagram_content_publish | Google OAuth (youtube.upload) |
// TikTok Login Kit + video.upload | LinkedIn r_liteprofile/w_member_social (+w_organization_social) |
// Pinterest user_accounts:read,pins:read,pins:write,boards:read,boards:write.

import (
	"fmt"
	"net/url"
	"os"

	"solomon/backend/models"
)

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

// AuthURL returns the "Connect" URL the frontend opens in a popup.
func AuthURL(network models.Network) (string, error) {
	redirect := env("OAUTH_REDIRECT_BASE", "http://localhost:8080/api/accounts/callback") + "/" + string(network)
	switch network {
	case models.NetworkTwitter:
		q := url.Values{
			"response_type": {"code"}, "client_id": {env("TWITTER_CLIENT_ID", "YOUR_X_CLIENT_ID")},
			"redirect_uri": {redirect}, "scope": {"tweet.read tweet.write users.read offline.access"},
			"state": {"xyz"}, "code_challenge": {"challenge"}, "code_challenge_method": {"plain"},
		}
		return "https://twitter.com/i/oauth2/authorize?" + q.Encode(), nil
	case models.NetworkFacebook:
		q := url.Values{
			"client_id": {env("FB_APP_ID", "YOUR_FB_APP_ID")}, "redirect_uri": {redirect},
			"scope": {"pages_show_list,pages_read_engagement,pages_manage_posts"}, "response_type": {"code"},
		}
		return "https://www.facebook.com/v21.0/dialog/oauth?" + q.Encode(), nil
	case models.NetworkInstagram:
		q := url.Values{
			"client_id": {env("FB_APP_ID", "YOUR_FB_APP_ID")}, "redirect_uri": {redirect},
			"scope": {"instagram_basic,instagram_content_publish,pages_show_list"}, "response_type": {"code"},
		}
		return "https://www.facebook.com/v21.0/dialog/oauth?" + q.Encode(), nil
	case models.NetworkYouTube:
		q := url.Values{
			"client_id": {env("GOOGLE_CLIENT_ID", "YOUR_GOOGLE_CLIENT_ID")}, "redirect_uri": {redirect},
			"scope": {"https://www.googleapis.com/auth/youtube.upload"}, "response_type": {"code"},
			"access_type": {"offline"}, "prompt": {"consent"},
		}
		return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), nil
	case models.NetworkTikTok:
		q := url.Values{
			"client_key": {env("TIKTOK_CLIENT_KEY", "YOUR_TIKTOK_KEY")}, "redirect_uri": {redirect},
			"scope": {"user.info.basic,video.upload,video.publish"}, "response_type": {"code"},
		}
		return "https://www.tiktok.com/v2/auth/authorize/?" + q.Encode(), nil
	case models.NetworkLinkedIn:
		q := url.Values{
			"client_id": {env("LINKEDIN_CLIENT_ID", "YOUR_LI_ID")}, "redirect_uri": {redirect},
			"scope": {"openid profile w_member_social"}, "response_type": {"code"},
		}
		return "https://www.linkedin.com/oauth/v2/authorization?" + q.Encode(), nil
	case models.NetworkPinterest:
		q := url.Values{
			"client_id": {env("PINTEREST_APP_ID", "YOUR_PIN_ID")}, "redirect_uri": {redirect},
			"scope": {"boards:read,boards:write,pins:read,pins:write"}, "response_type": {"code"},
		}
		return "https://www.pinterest.com/oauth/?" + q.Encode(), nil
	}
	return "", fmt.Errorf("unknown network %s", network)
}
