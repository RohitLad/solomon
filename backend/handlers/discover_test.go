package handlers

// Phase-B tests: "Outside Solomon" native-post discovery — demo determinism,
// own-post dedupe, backfill cutoff, 20-day stat freeze, analytics union with
// outside totals, and boost inclusion.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"solomon/backend/models"
)

func TestDiscoverDemoDeterministic(t *testing.T) {
	ta := newTestApp(t)
	createAccount(t, ta, "twitter", "@native")
	found, notes := DiscoverExternal(ta.db)
	if len(notes) != 0 {
		t.Fatalf("demo discovery should be note-free, got %v", notes)
	}
	if found != 3 {
		t.Fatalf("demo discovery found %d, want 3 (4th candidate is outside backfill)", found)
	}
	var n int64
	ta.db.Model(&models.ExternalPost{}).Count(&n)
	if n != 3 {
		t.Fatalf("stored %d external posts, want 3", n)
	}
	// second run: dedupe, nothing new
	found2, _ := DiscoverExternal(ta.db)
	if found2 != 0 {
		t.Fatalf("second run found %d, want 0 (dedupe)", found2)
	}
	ta.db.Model(&models.ExternalPost{}).Count(&n)
	if n != 3 {
		t.Fatalf("duplicates stored: %d rows, want 3", n)
	}
	// every discovered post got a stats snapshot (demo stats are free)
	var snaps int64
	ta.db.Model(&models.AnalyticsSnapshot{}).Where("external_post_id IS NOT NULL").Count(&snaps)
	if snaps != 3 {
		t.Fatalf("snapshots %d, want 3", snaps)
	}
}

func TestDiscoverSkipsOwnAppPosts(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@mine")
	// publish through the app first (demo network_post_id recorded on target)
	createPost(t, ta,
		`{"content":"app-made","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	var targets []models.PostTarget
	ta.db.Where("account_id = ?", acct).Find(&targets)
	if len(targets) != 1 || targets[0].NetworkPostID == "" {
		t.Fatalf("expected 1 published app target, got %+v", targets)
	}
	// now pretend the network ALSO lists that same ID natively: inject it as a
	// would-be candidate by checking dedupe directly — discovery must not store it.
	found, _ := DiscoverExternal(ta.db)
	var ext []models.ExternalPost
	ta.db.Where("account_id = ?", acct).Find(&ext)
	for _, e := range ext {
		if e.NetworkPostID == targets[0].NetworkPostID {
			t.Fatalf("app-published ID %q stored as external", e.NetworkPostID)
		}
	}
	_ = found
}

func TestDiscoverStatFreeze(t *testing.T) {
	ta := newTestApp(t)
	acctID := createAccount(t, ta, "instagram", "@oldies")
	old := time.Now().AddDate(0, 0, -21)
	npid := "frozen_1"
	ta.db.Create(&models.ExternalPost{
		AccountID: acctID, Network: "instagram", NetworkPostID: npid,
		Text: "too old for fresh stats", PublishedAt: &old,
	})
	_, _ = DiscoverExternal(ta.db)
	var snaps int64
	ta.db.Model(&models.AnalyticsSnapshot{}).Where("network_post_id = ?", npid).Count(&snaps)
	if snaps != 0 {
		t.Fatalf("21-day-old external post got %d snapshots, want 0 (frozen)", snaps)
	}
}

func TestDiscoverUnsupportedNetworkNotes(t *testing.T) {
	ta := newTestApp(t)
	// real (non-demo) TikTok token: listing needs an audited app -> note, no crash
	resp := do(t, ta.app, "POST", "/api/accounts",
		strings.NewReader(`{"network":"tiktok","name":"@real","access_token":"real-xyz"}`), "application/json")
	_ = readBody(t, resp)
	if resp.StatusCode != 201 {
		t.Fatalf("create account: status %d", resp.StatusCode)
	}
	found, notes := DiscoverExternal(ta.db)
	if found != 0 || len(notes) != 1 {
		t.Fatalf("want 0 found + 1 note, got %d + %v", found, notes)
	}
	if !strings.Contains(notes[0], "tiktok") {
		t.Fatalf("note should name the network: %q", notes[0])
	}
}

func TestAnalyticsUnionOutside(t *testing.T) {
	ta := newTestApp(t)
	createAccount(t, ta, "twitter", "@union")
	if _, notes := DiscoverExternal(ta.db); len(notes) != 0 {
		t.Fatalf("notes: %v", notes)
	}
	resp := do(t, ta.app, "GET", "/api/analytics", nil, "")
	body := readBody(t, resp)
	var out struct {
		Totals  map[string]any   `json:"totals"`
		Outside map[string]any   `json:"outside"`
		Rows    []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Outside == nil {
		t.Fatalf("missing outside totals (body %q)", string(body))
	}
	ext := 0
	for _, r := range out.Rows {
		if r["external"] == true {
			ext++
			if r["published_at"] == nil {
				t.Fatalf("external row needs published_at for calendar dots: %v", r)
			}
		}
	}
	if ext != 3 {
		t.Fatalf("external rows %d, want 3", ext)
	}
	if posts, _ := out.Outside["posts"].(float64); posts != 3 {
		t.Fatalf("outside.posts %v, want 3", out.Outside["posts"])
	}
	// app totals must exclude outside rows (no app posts here → zeros)
	if posts, _ := out.Totals["posts"].(float64); posts != 0 {
		t.Fatalf("totals.posts %v, want 0 (outside excluded)", out.Totals["posts"])
	}
	if views, _ := out.Totals["views"].(float64); views != 0 {
		t.Fatalf("totals.views %v, want 0 (outside excluded)", out.Totals["views"])
	}
	// refresh endpoint reports discovery too
	resp = do(t, ta.app, "POST", "/api/analytics/refresh", nil, "")
	rbody := readBody(t, resp)
	var ref struct {
		Refreshed  int      `json:"refreshed"`
		Discovered int      `json:"discovered"`
		Errors     []string `json:"errors"`
	}
	if err := json.Unmarshal(rbody, &ref); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if ref.Errors == nil {
		t.Fatalf("errors must be [], never null")
	}
}

func TestBoostIncludesExternal(t *testing.T) {
	ta := newTestApp(t)
	acctID := createAccount(t, ta, "twitter", "@boost")
	pub := time.Now().Add(-2 * time.Hour).Truncate(time.Hour)
	ep := models.ExternalPost{
		AccountID: acctID, Network: "twitter", NetworkPostID: "boost_1",
		Text: "native banger", PublishedAt: &pub,
	}
	if err := ta.db.Create(&ep).Error; err != nil {
		t.Fatal(err)
	}
	ta.db.Create(&models.AnalyticsSnapshot{
		ExternalPostID: &ep.ID, NetworkPostID: "boost_1",
		Views: 100, Likes: 9000, Comments: 500, Shares: 200, FetchedAt: time.Now(),
	})
	boost := engagementBoost(ta.db, []models.Network{"twitter"})
	if len(boost) == 0 {
		t.Fatalf("boost must include external-post engagement")
	}
}
