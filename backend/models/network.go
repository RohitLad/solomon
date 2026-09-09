package models

// Network is one of the 7 supported social networks.
type Network string

const (
	NetworkTwitter   Network = "twitter"   // X API v2
	NetworkFacebook  Network = "facebook"  // Graph API v21
	NetworkInstagram Network = "instagram" // Instagram Graph API (via FB Page)
	NetworkYouTube   Network = "youtube"   // YouTube Data API v3
	NetworkTikTok    Network = "tiktok"    // TikTok Content Posting API v2
	NetworkLinkedIn  Network = "linkedin"  // LinkedIn Posts API (versioned)
	NetworkPinterest Network = "pinterest" // Pinterest API v5
)

func AllNetworks() []Network {
	return []Network{
		NetworkTwitter, NetworkFacebook, NetworkInstagram,
		NetworkYouTube, NetworkTikTok, NetworkLinkedIn, NetworkPinterest,
	}
}
