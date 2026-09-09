package networks

import (
	"testing"

	"solomon/backend/models"
)

// Demo/empty tokens never hit the network: "" + nil, so account creation
// can't fail on avatars and the UI falls back to initials.
func TestFetchAvatarDemo(t *testing.T) {
	for _, n := range models.AllNetworks() {
		for _, tok := range []string{"", "demo-twitter"} {
			url, err := FetchAvatar(n, tok, "", "")
			if err != nil {
				t.Fatalf("%s token %q: unexpected error %v", n, tok, err)
			}
			if url != "" {
				t.Fatalf("%s token %q: want empty url, got %q", n, tok, url)
			}
		}
	}
}

func TestFetchAvatarNeedsIDs(t *testing.T) {
	// Real token but missing IDs/scopes -> clear errors, never panics.
	cases := []struct {
		network models.Network
		extra   string
		extID   string
	}{
		{models.NetworkTwitter, "", ""},
		{models.NetworkInstagram, "", ""},
		{models.NetworkFacebook, "", ""}, // "me" path will fail offline -> error either way
		{models.NetworkTikTok, "", ""},
		{"myspace", "", ""},
	}
	for _, c := range cases {
		url, err := FetchAvatar(c.network, "real-token", c.extra, c.extID)
		if c.network == "myspace" && err == nil {
			t.Fatalf("unknown network must error")
		}
		_ = url
		if err == nil && c.network != "myspace" {
			t.Logf("%s unexpectedly succeeded (live network reachable?)", c.network)
		}
	}
	// Twitter without a numeric user ID must fail fast, no HTTP attempt.
	if _, err := FetchAvatar(models.NetworkTwitter, "real-token", "", ""); err == nil {
		t.Fatalf("twitter without external ID must error")
	}
}
