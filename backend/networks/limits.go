package networks

// This file is the heart of "network specific limitations + remedies".
// Latest official APIs (2025-2026):
//   Twitter/X: X API v2 free/basic/pro. Tweets endpoint POST /2/tweets.
//   Facebook: Graph API v21 (pages: /{page-id}/feed, /photos, /videos, /reels).
//   Instagram: Instagram Graph API (container + publish; Reels; Carousel).
//   YouTube: Data API v3 videos.insert (Shorts = <=60s vertical + #Shorts).
//   TikTok: Content Posting API v2 (FILE_UPLOAD + post; PHOTO or VIDEO).
//   LinkedIn: versioned Posts API (w/ 2024+ /v2/posts + /v2/ugcPosts legacy).
//   Pinterest: API v5 (POST /v5/pins).
//
// If any limit changes upstream, update ONLY this file.

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"solomon/backend/models"
)

// Limit describes what a network accepts.
type Limit struct {
	Network         models.Network `json:"network"`
	MaxChars        int            `json:"max_chars"`
	MaxImages       int            `json:"max_images"`
	MaxVideos       int            `json:"max_videos"`
	MaxVideoSeconds int            `json:"max_video_seconds"`
	MaxVideoMB      int            `json:"max_video_mb"`
	TextOnly        bool           `json:"text_only_allowed"`
	NeedsTitle      bool           `json:"needs_title"`
	Notes           string         `json:"notes"`
}

func Limits() map[models.Network]Limit {
	return map[models.Network]Limit{
		models.NetworkTwitter: {
			Network:  models.NetworkTwitter,
			MaxChars: 280, MaxImages: 4, MaxVideos: 1, MaxVideoSeconds: 140, MaxVideoMB: 512,
			Notes: "X API v2. Over 280 chars -> auto-split into a thread (reply chain). 4 photos OR 1 video/GIF per tweet. Polls supported separately.",
		},
		models.NetworkFacebook: {
			Network:  models.NetworkFacebook,
			MaxChars: 63206, MaxImages: 10, MaxVideos: 1, MaxVideoSeconds: 14400, MaxVideoMB: 10240,
			Notes: "Graph v21. Very generous. Multi-photo = album/feed carousel. Reels <=90s 9:16 recommended.",
		},
		models.NetworkInstagram: {
			Network:  models.NetworkInstagram,
			MaxChars: 2200, MaxImages: 10, MaxVideos: 1, MaxVideoSeconds: 5400, MaxVideoMB: 4000,
			Notes: "IG Graph API. Feed photo needs min 320px. Carousel = 2-10 items, same ratio. Reels <=3min 9:16. NO text-only posts -> requires image/video.",
		},
		models.NetworkYouTube: {
			Network: models.NetworkYouTube, NeedsTitle: true,
			MaxChars: 5000, MaxImages: 0, MaxVideos: 1, MaxVideoSeconds: 43200, MaxVideoMB: 262144,
			Notes: "Data API v3. VIDEO REQUIRED (Shorts <=60s vertical, else long-form). Title<=100 chars. Thumbnail image optional.",
		},
		models.NetworkTikTok: {
			Network:  models.NetworkTikTok,
			MaxChars: 2200, MaxImages: 35, MaxVideos: 1, MaxVideoSeconds: 600, MaxVideoMB: 2000,
			Notes: "Content Posting API v2. VIDEO or PHOTO post required (no text-only). Photo mode = 1-35 images. Video 3s-10min.",
		},
		models.NetworkLinkedIn: {
			Network:  models.NetworkLinkedIn,
			MaxChars: 3000, MaxImages: 9, MaxVideos: 1, MaxVideoSeconds: 900, MaxVideoMB: 5120,
			Notes: "Posts API. Text-only OK. Multi-image (carousel) up to 9. Video 3s-15min. Company + personal posts.",
		},
		models.NetworkPinterest: {
			Network: models.NetworkPinterest, NeedsTitle: true,
			MaxChars: 800, MaxImages: 5, MaxVideos: 1, MaxVideoSeconds: 900, MaxVideoMB: 2000,
			Notes: "API v5. IMAGE or VIDEO REQUIRED. Title<=100, desc<=800. Link (destination) strongly recommended. Board required -> stored in account Extra.",
		},
	}
}

// Adaptation is one automatic fix the scheduler/publisher will apply.
type Adaptation struct {
	Network models.Network `json:"network"`
	Action  string         `json:"action"` // e.g. "split_thread", "trim", "needs_media", "needs_title"
	Detail  string         `json:"detail"`
	Chunks  []string       `json:"chunks,omitempty"` // thread parts for twitter
}

// PlanResult is returned by the /api/preview endpoint and used before publish.
type PlanResult struct {
	OK          bool         `json:"ok"`
	Adaptations []Adaptation `json:"adaptations"`
	Errors      []string     `json:"errors"`
}

// SplitThread splits long text into <=280 char chunks on word boundaries,
// numbered "1/n ..." so the remedy is visible to the user.
func SplitThread(text string, max int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var chunks []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if utf8.RuneCountInString(try) <= max-10 { // reserve room for " (1/n)"
			cur = try
			continue
		}
		if cur != "" {
			chunks = append(chunks, cur)
		}
		// single word longer than limit: hard-cut it
		for utf8.RuneCountInString(w) > max-10 {
			r := []rune(w)
			chunks = append(chunks, string(r[:max-10]))
			w = string(r[max-10:])
		}
		cur = w
	}
	if cur != "" {
		chunks = append(chunks, cur)
	}
	n := len(chunks)
	for i := range chunks {
		chunks[i] = fmt.Sprintf("%s (%d/%d)", chunks[i], i+1, n)
	}
	return chunks
}

// ValidateAndAdapt checks ONE target and proposes remedies.
// nImages/nVideos/title/hasLink describe the master post.
func ValidateAndAdapt(network models.Network, text, title string, nImages, nVideos int, hasLink bool) PlanResult {
	lim, ok := Limits()[network]
	if !ok {
		return PlanResult{OK: false, Errors: []string{"unknown network"}}
	}
	res := PlanResult{OK: true}
	runes := utf8.RuneCountInString(text)

	addErr := func(s string) {
		res.OK = false
		res.Errors = append(res.Errors, string(network)+": "+s)
	}

	// --- text length ---
	if runes > lim.MaxChars {
		if network == models.NetworkTwitter {
			chunks := SplitThread(text, lim.MaxChars)
			res.Adaptations = append(res.Adaptations, Adaptation{
				Network: network, Action: "split_thread",
				Detail: fmt.Sprintf("%d chars > %d: will post as %d-tweet thread", runes, lim.MaxChars, len(chunks)),
				Chunks: chunks,
			})
		} else if network == models.NetworkLinkedIn || network == models.NetworkPinterest {
			// LinkedIn/Pinterest hard-fail on overlength: offer trim remedy
			res.Adaptations = append(res.Adaptations, Adaptation{
				Network: network, Action: "trim",
				Detail: fmt.Sprintf("%d chars > %d: will trim + append link/first-comment. Put full text in First Comment.", runes, lim.MaxChars),
			})
		} else {
			// FB/IG/YT/TikTok: still publishable but trim note
			res.Adaptations = append(res.Adaptations, Adaptation{
				Network: network, Action: "trim",
				Detail: fmt.Sprintf("%d chars > %d: will auto-trim to fit", runes, lim.MaxChars),
			})
		}
	}

	// --- media requirements ---
	nMedia := nImages + nVideos
	switch network {
	case models.NetworkInstagram, models.NetworkTikTok, models.NetworkPinterest, models.NetworkYouTube:
		if nMedia == 0 {
			addErr("requires image/video but post has none -> add media or skip this account (remedy: attach media, else target will be SKIPPED)")
		}
	case models.NetworkTwitter:
		if nImages > lim.MaxImages {
			res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "trim_media",
				Detail: fmt.Sprintf("%d images > %d: only first %d will be attached", nImages, lim.MaxImages, lim.MaxImages)})
		}
		if nImages > 0 && nVideos > 0 {
			res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "trim_media",
				Detail: "images+video together not allowed: video wins, images go to follow-up reply"})
		}
	case models.NetworkLinkedIn:
		if nImages > lim.MaxImages {
			res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "trim_media",
				Detail: fmt.Sprintf("%d images > %d: only first %d kept", nImages, lim.MaxImages, lim.MaxImages)})
		}
	case models.NetworkFacebook:
		if nImages > lim.MaxImages {
			res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "trim_media",
				Detail: fmt.Sprintf("%d images > %d: extras become follow-up comment links", nImages, lim.MaxImages)})
		}
	}

	if network == models.NetworkYouTube && nVideos == 0 {
		addErr("YouTube requires exactly 1 video -> attach a video or skip YouTube targets")
	}
	if network == models.NetworkYouTube && title == "" {
		addErr("YouTube requires a Title (<=100 chars) -> fill the Title field")
	}
	if network == models.NetworkPinterest && title == "" {
		res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "needs_title",
			Detail: "Pinterest works best with a Title: first 100 chars of text will be used"})
	}
	if network == models.NetworkPinterest && !hasLink {
		res.Adaptations = append(res.Adaptations, Adaptation{Network: network, Action: "missing_link",
			Detail: "Pinterest prefers a destination link: add Link field for click-through"})
	}
	if network == models.NetworkInstagram && nMedia == 0 {
		// already errored; no extra note needed
		_ = hasLink // links in caption aren't clickable on IG -> handled in publisher (first comment)
	}

	return res
}

// TrimTo trims text to max runes.
func TrimTo(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
