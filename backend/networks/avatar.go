package networks

// Profile-picture fetch for account avatars. Best-effort by design: demo or
// empty tokens return "" (the UI falls back to an initials circle), and any
// failure returns a clear error so the caller can skip with a note — account
// creation/refresh must NEVER fail because of an avatar.
//
// Frontend renders Avatar.svelte: photo with on:error fallback + network logo
// badge, so a stale/broken URL degrades gracefully on its own.

import (
	"fmt"
	"net/url"

	"solomon/backend/models"
)

// FetchAvatar returns a public profile-image URL, or "" when unavailable.
func FetchAvatar(network models.Network, accessToken, extra, externalID string) (string, error) {
	if accessToken == "" || len(accessToken) >= 5 && accessToken[:5] == "demo-" {
		return "", nil
	}
	switch network {
	case models.NetworkFacebook:
		return fetchFacebookAvatar(accessToken, acctID(extra, externalID, "page_id"))
	case models.NetworkInstagram:
		return fetchInstagramAvatar(accessToken, acctID(extra, externalID, "ig_user_id"))
	case models.NetworkTwitter:
		return fetchTwitterAvatar(accessToken, externalID)
	case models.NetworkYouTube:
		return fetchYouTubeAvatar(accessToken, extraField(extra, "channel_id"))
	case models.NetworkTikTok:
		return "", fmt.Errorf("tiktok user/info needs an audited app — avatar skipped")
	case models.NetworkLinkedIn:
		return fetchLinkedInAvatar(accessToken)
	case models.NetworkPinterest:
		return fetchPinterestAvatar(accessToken)
	default:
		return "", fmt.Errorf("unknown network %s", network)
	}
}

func fetchFacebookAvatar(token, pageID string) (string, error) {
	id := pageID
	if id == "" {
		id = "me"
	}
	out, err := getJSON("https://graph.facebook.com/v21.0/"+id+"/picture",
		url.Values{"redirect": {"false"}, "type": {"large"}, "access_token": {token}})
	if err != nil {
		return "", fmt.Errorf("facebook picture: %v", err)
	}
	if data, ok := out["data"].(map[string]any); ok {
		if u, ok := data["url"].(string); ok && u != "" {
			return u, nil
		}
	}
	return "", fmt.Errorf("facebook picture: no url in response")
}

func fetchInstagramAvatar(token, igID string) (string, error) {
	if igID == "" {
		return "", fmt.Errorf("instagram: ig_user_id (Extra) required for avatar")
	}
	out, err := getJSON("https://graph.facebook.com/v21.0/"+igID,
		url.Values{"fields": {"profile_picture_url"}, "access_token": {token}})
	if err != nil {
		return "", fmt.Errorf("instagram avatar: %v", err)
	}
	if u, ok := out["profile_picture_url"].(string); ok && u != "" {
		return u, nil
	}
	return "", fmt.Errorf("instagram avatar: no profile_picture_url in response")
}

func fetchTwitterAvatar(token, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("twitter/X: external user ID required (numeric user ID in External ID)")
	}
	out, err := getObject("https://api.twitter.com/2/users/"+userID,
		url.Values{"user.fields": {"profile_image_url"}}, token)
	if err != nil {
		return "", fmt.Errorf("twitter/X avatar: %v", err)
	}
	if data, ok := out["data"].(map[string]any); ok {
		if u, ok := data["profile_image_url"].(string); ok && u != "" {
			return u, nil
		}
	}
	return "", fmt.Errorf("twitter/X avatar: no profile_image_url in response")
}

func fetchYouTubeAvatar(token, channelID string) (string, error) {
	params := url.Values{"part": {"snippet"}}
	if channelID != "" {
		params.Set("id", channelID)
	} else {
		params.Set("mine", "true")
	}
	items, err := getItems("https://www.googleapis.com/youtube/v3/channels", params, token)
	if err != nil || len(items) == 0 {
		return "", fmt.Errorf("youtube channel: %v", err)
	}
	m, _ := items[0].(map[string]any)
	if sn, ok := m["snippet"].(map[string]any); ok {
		if th, ok := sn["thumbnails"].(map[string]any); ok {
			for _, size := range []string{"medium", "default", "high"} {
				if t, ok := th[size].(map[string]any); ok {
					if u, ok := t["url"].(string); ok && u != "" {
						return u, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("youtube avatar: no thumbnails in response")
}

func fetchLinkedInAvatar(token string) (string, error) {
	params := url.Values{}
	params.Set("projection", "(profilePicture(displayImage~:playableStreams))")
	out, err := getObject("https://api.linkedin.com/v2/me", params, token)
	if err != nil {
		return "", fmt.Errorf("linkedin avatar: %v (needs r_liteprofile)", err)
	}
	if pp, ok := out["profilePicture"].(map[string]any); ok {
		if di, ok := pp["displayImage~"].(map[string]any); ok {
			if els, ok := di["elements"].([]any); ok && len(els) > 0 {
				if last, ok := els[len(els)-1].(map[string]any); ok {
					for _, k := range []string{"identifier", "url"} {
						if u, ok := last[k].(string); ok && u != "" {
							return u, nil
						}
					}
					if ids, ok := last["identifiers"].([]any); ok && len(ids) > 0 {
						if m, ok := ids[0].(map[string]any); ok {
							if u, ok := m["identifier"].(string); ok && u != "" {
								return u, nil
							}
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("linkedin avatar: no playableStreams image in response")
}

func fetchPinterestAvatar(token string) (string, error) {
	out, err := getObject("https://api.pinterest.com/v5/user_account/", url.Values{}, token)
	if err != nil {
		return "", fmt.Errorf("pinterest avatar: %v", err)
	}
	if u, ok := out["profile_image"].(string); ok && u != "" {
		return u, nil
	}
	return "", fmt.Errorf("pinterest avatar: no profile_image in response")
}
