package networks

// AI caption variants + per-network hashtags.
// Works with ZERO config (offline template generator) and upgrades to a
// real LLM when AI_API_KEY is set (any OpenAI-compatible chat-completions
// endpoint via AI_BASE_URL / AI_MODEL).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"solomon/backend/models"
)

var aiClient = &http.Client{Timeout: 60 * time.Second}

// hashtag banks per network (max tags each network tolerates well).
var hashtagBank = map[models.Network][]string{
	models.NetworkTwitter:   {"#marketing", "#socialmedia", "#growth", "#startup"},
	models.NetworkFacebook:  {"#smallbusiness", "#marketing"},
	models.NetworkInstagram: {"#instagood", "#marketing", "#smallbusiness", "#contentcreator", "#reels", "#growth", "#branding", "#socialmediatips", "#entrepreneur", "#creator"},
	models.NetworkYouTube:   {"#shorts", "#howto"},
	models.NetworkTikTok:    {"#fyp", "#viral", "#tiktoktips", "#creator", "#trending"},
	models.NetworkLinkedIn:  {"#marketing", "#leadership", "#growth", "#startup"},
	models.NetworkPinterest: {"#ideas", "#inspiration", "#howto"},
}

func hashtagsFor(n models.Network, max int) string {
	bank := hashtagBank[n]
	if len(bank) > max {
		bank = bank[:max]
	}
	return strings.Join(bank, " ")
}

type CaptionVariant struct {
	Tone    string `json:"tone"`
	Text    string `json:"text"`
	Hashtags string `json:"hashtags"`
}

// GenerateCaptions returns 3 variants (professional / casual / punchy).
// Falls back to offline templates if no AI key or the API call fails.
func GenerateCaptions(text string, network models.Network) ([]CaptionVariant, string, error) {
	ifaka := strings.TrimSpace(text)
	if ifaka == "" {
		return nil, "", fmt.Errorf("text required")
	}
	if key := os.Getenv("AI_API_KEY"); key != "" {
		if v, src, err := llmCaptions(key, ifaka, network); err == nil {
			return v, src, nil
		}
		// fall through to offline on error
	}
	return offlineCaptions(ifaka, network), "offline-templates", nil
}

func offlineCaptions(text string, network models.Network) []CaptionVariant {
	oneLine := strings.Join(strings.Fields(text), " ")
	mk := func(tone, body string) CaptionVariant {
		maxTags := 3
		if network == models.NetworkInstagram || network == models.NetworkTikTok {
			maxTags = 8
		}
		v := CaptionVariant{Tone: tone, Text: body, Hashtags: hashtagsFor(network, maxTags)}
		// keep X variants post-ready
		if network == models.NetworkTwitter {
			v.Text = TrimTo(body, 240)
		}
		return v
	}
	return []CaptionVariant{
		mk("professional", oneLine),
		mk("casual", "Quick share 👇 "+oneLine),
		mk("punchy", strings.ToUpper(firstSentence(oneLine))),
	}
}

func firstSentence(s string) string {
	for i, r := range s {
		if r == '.' || r == '!' || r == '?' {
			return strings.TrimSpace(s[:i+1])
		}
	}
	return s
}

func llmCaptions(key, text string, network models.Network) ([]CaptionVariant, string, error) {
	base := os.Getenv("AI_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1/chat/completions"
	}
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	prompt := fmt.Sprintf(`Rewrite this social post for %s in 3 tones (professional, casual, punchy). Keep each under %d chars. Reply ONLY as JSON array like [{"tone":"professional","text":"..."},...]. Post: %s`,
		network, Limits()[network].MaxChars, text)
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.8,
	})
	req, _ := http.NewRequest("POST", base, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := aiClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 {
		return nil, "", fmt.Errorf("ai: bad response")
	}
	raw := strings.TrimSpace(out.Choices[0].Message.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	var parsed []CaptionVariant
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed) == 0 {
		return nil, "", fmt.Errorf("ai: could not parse variants")
	}
	maxTags := 3
	if network == models.NetworkInstagram || network == models.NetworkTikTok {
		maxTags = 8
	}
	for i := range parsed {
		parsed[i].Hashtags = hashtagsFor(network, maxTags)
	}
	return parsed, "llm:" + model, nil
}
