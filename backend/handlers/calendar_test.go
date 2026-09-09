package handlers

// Phase-A tests: calendar reschedule (PATCH), split delete semantics
// (hard for draft/scheduled, soft + history for published), and the guards
// that keep deleted posts out of the scheduler/evergreen/boost.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"solomon/backend/models"
)

func createAccount(t *testing.T, ta *testApp, network, name string) string {
	t.Helper()
	resp := do(t, ta.app, "POST", "/api/accounts",
		strings.NewReader(`{"network":"`+network+`","name":"`+name+`"}`), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != 201 {
		t.Fatalf("create account: status %d (body %q)", resp.StatusCode, string(body))
	}
	var acct struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &acct); err != nil || acct.ID == "" {
		t.Fatalf("decode account: %v (body %q)", err, string(body))
	}
	return acct.ID
}

func createPost(t *testing.T, ta *testApp, payload string, wantStatus int) string {
	t.Helper()
	resp := do(t, ta.app, "POST", "/api/posts", strings.NewReader(payload), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != wantStatus {
		t.Fatalf("create post: status %d, want %d (body %q)", resp.StatusCode, wantStatus, string(body))
	}
	if wantStatus != 201 {
		return ""
	}
	var p struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &p); err != nil || p.ID == "" {
		t.Fatalf("decode post: %v (body %q)", err, string(body))
	}
	return p.ID
}

func getPosts(t *testing.T, ta *testApp, query string) []map[string]any {
	t.Helper()
	resp := do(t, ta.app, "GET", "/api/posts"+query, nil, "")
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/posts%s: status %d (body %q)", query, resp.StatusCode, string(body))
	}
	var posts []map[string]any
	if err := json.Unmarshal(body, &posts); err != nil {
		t.Fatalf("decode posts: %v (body %q)", err, string(body))
	}
	if posts == nil {
		t.Fatalf("posts decoded as null — must be []")
	}
	return posts
}

func TestPatchReschedule(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@cal")
	id := createPost(t, ta,
		`{"content":"move me","scheduled_at":"2030-06-01T10:00:00Z","targets":[{"account_id":"`+acct+`"}]}`,
		201)

	// move to a new date
	resp := do(t, ta.app, "PATCH", "/api/posts/"+id,
		strings.NewReader(`{"scheduled_at":"2030-07-04T15:30:00Z"}`), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("PATCH: status %d (body %q)", resp.StatusCode, string(body))
	}
	var moved struct {
		Status      string  `json:"status"`
		ScheduledAt *string `json:"scheduled_at"`
	}
	if err := json.Unmarshal(body, &moved); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if moved.Status != "scheduled" || moved.ScheduledAt == nil || !strings.Contains(*moved.ScheduledAt, "2030-07-04") {
		t.Fatalf("unexpected move result: %+v", moved)
	}

	// clear the date -> back to draft
	resp = do(t, ta.app, "PATCH", "/api/posts/"+id,
		strings.NewReader(`{"scheduled_at":null}`), "application/json")
	body = readBody(t, resp)
	var cleared struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(body, &cleared)
	if resp.StatusCode != 200 || cleared.Status != "draft" {
		t.Fatalf("clear date: status %d (body %q)", resp.StatusCode, string(body))
	}

	// unknown id -> 404
	resp = do(t, ta.app, "PATCH", "/api/posts/nope",
		strings.NewReader(`{"scheduled_at":"2030-01-01T00:00:00Z"}`), "application/json")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("PATCH missing: status %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestPatchRejectsPublished(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calpub")
	id := createPost(t, ta,
		`{"content":"live already","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "PATCH", "/api/posts/"+id,
		strings.NewReader(`{"scheduled_at":"2030-01-01T00:00:00Z"}`), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != 400 {
		t.Fatalf("PATCH published: status %d, want 400 (body %q)", resp.StatusCode, string(body))
	}
}

func TestDeleteDraftIsHard(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@caldel")
	id := createPost(t, ta,
		`{"content":"drop me","targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "DELETE", "/api/posts/"+id, nil, "")
	body := readBody(t, resp)
	if resp.StatusCode != 200 || strings.Contains(string(body), `"soft_deleted":true`) {
		t.Fatalf("DELETE draft: status %d (body %q)", resp.StatusCode, string(body))
	}
	if posts := getPosts(t, ta, ""); len(posts) != 0 {
		t.Fatalf("expected 0 posts, got %d", len(posts))
	}
	if posts := getPosts(t, ta, "?include_deleted=1"); len(posts) != 0 {
		t.Fatalf("hard-deleted post must not reappear, got %d", len(posts))
	}
	var n int64
	ta.db.Model(&models.PostTarget{}).Where("post_id = ?", id).Count(&n)
	if n != 0 {
		t.Fatalf("targets not cleaned: %d rows", n)
	}
}

func TestDeletePublishedIsSoftAndKeepsAnalytics(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calsoft")
	id := createPost(t, ta,
		`{"content":"keep my stats","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)

	resp := do(t, ta.app, "DELETE", "/api/posts/"+id, nil, "")
	body := readBody(t, resp)
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"soft_deleted":true`) {
		t.Fatalf("DELETE published: status %d (body %q)", resp.StatusCode, string(body))
	}

	// hidden by default, visible with the flag + deleted_at set
	if posts := getPosts(t, ta, ""); len(posts) != 0 {
		t.Fatalf("soft-deleted post must hide by default, got %d", len(posts))
	}
	posts := getPosts(t, ta, "?include_deleted=1")
	if len(posts) != 1 || posts[0]["deleted_at"] == nil {
		t.Fatalf("soft-deleted post must show with deleted_at (got %v)", posts)
	}

	// analytics keeps the row, flagged
	resp = do(t, ta.app, "GET", "/api/analytics", nil, "")
	abody := readBody(t, resp)
	var analytics struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(abody, &analytics); err != nil {
		t.Fatalf("decode analytics: %v", err)
	}
	if len(analytics.Rows) != 1 || analytics.Rows[0]["deleted"] != true {
		t.Fatalf("analytics must keep 1 deleted-flagged row (got %v)", analytics.Rows)
	}

	// stat refresh freezes deleted history (no new app snapshots;
	// discovery may still add outside-Solomon rows, covered elsewhere)
	var before int64
	ta.db.Model(&models.AnalyticsSnapshot{}).Where("external_post_id IS NULL").Count(&before)
	resp = do(t, ta.app, "POST", "/api/analytics/refresh", nil, "")
	_ = readBody(t, resp)
	var after int64
	ta.db.Model(&models.AnalyticsSnapshot{}).Where("external_post_id IS NULL").Count(&after)
	if after != before {
		t.Fatalf("refresh must not add snapshots for deleted posts (%d -> %d)", before, after)
	}
}

func TestDeletePrunesEvergreenPool(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calgreen")
	id := createPost(t, ta,
		`{"content":"pool member","targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "POST", "/api/evergreen",
		strings.NewReader(`{"name":"r","pool_post_ids":["`+id+`"],"account_ids":["`+acct+`"],"interval_hours":24}`),
		"application/json")
	if resp.StatusCode != 201 {
		t.Fatalf("create rule: status %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = do(t, ta.app, "DELETE", "/api/posts/"+id, nil, "")
	_ = readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE pool post: status %d", resp.StatusCode)
	}
	var rule models.EvergreenRule
	if err := ta.db.First(&rule).Error; err != nil {
		t.Fatalf("load rule: %v", err)
	}
	if strings.Contains(rule.PoolPostIDs, id) {
		t.Fatalf("pool still references deleted post: %q", rule.PoolPostIDs)
	}
	// empty pool must not run, churn, or stall
	if n := RunDueRules(ta.db); n != 0 {
		t.Fatalf("RunDueRules ran %d, want 0 on empty pool", n)
	}
}

func TestEvergreenSkipsSoftDeletedPool(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calgreen2")
	id := createPost(t, ta,
		`{"content":"pool pub","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "POST", "/api/evergreen",
		strings.NewReader(`{"name":"r","pool_post_ids":["`+id+`"],"account_ids":["`+acct+`"],"interval_hours":24}`),
		"application/json")
	_ = resp.Body.Close()
	// force due now
	past := time.Now().Add(-time.Hour)
	ta.db.Model(&models.EvergreenRule{}).Where("name = ?", "r").Update("next_run_at", &past)

	// soft-delete the pool post -> rule must skip it, not stall or publish
	resp = do(t, ta.app, "DELETE", "/api/posts/"+id, nil, "")
	_ = readBody(t, resp)
	if n := RunDueRules(ta.db); n != 0 {
		t.Fatalf("RunDueRules published %d from a deleted pool, want 0", n)
	}
}

func TestSchedulerQuerySkipsDeleted(t *testing.T) {	ta := newTestApp(t)
	past := time.Now().Add(-time.Hour)
	gone := time.Now()
	ta.db.Create(&models.Post{Content: "ghost", Status: models.StatusScheduled, ScheduledAt: &past, DeletedAt: &gone})
	var due []models.Post
	ta.db.Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ? AND deleted_at IS NULL",
		models.StatusScheduled, time.Now()).Find(&due)
	if len(due) != 0 {
		t.Fatalf("scheduler query must skip soft-deleted posts, got %d", len(due))
	}
}

// Hard delete removes media files nobody else references, but keeps files
// still referenced (evergreen copies share FilePath).
func TestDeleteDraftCleansMediaFiles(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calmedia")
	solo := writeTempMedia(t, "solo-")
	shared := writeTempMedia(t, "shared-")

	id1 := createPost(t, ta,
		`{"content":"with media","targets":[{"account_id":"`+acct+`"}]}`,
		201)
	id2 := createPost(t, ta,
		`{"content":"shares media","targets":[{"account_id":"`+acct+`"}]}`,
		201)
	ta.db.Create(&models.MediaAsset{PostID: id1, FilePath: solo, MediaType: models.MediaImage})
	ta.db.Create(&models.MediaAsset{PostID: id1, FilePath: shared, MediaType: models.MediaImage})
	ta.db.Create(&models.MediaAsset{PostID: id2, FilePath: shared, MediaType: models.MediaImage})

	resp := do(t, ta.app, "DELETE", "/api/posts/"+id1, nil, "")
	_ = readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE: status %d", resp.StatusCode)
	}
	if _, err := os.Stat(solo); !os.IsNotExist(err) {
		t.Fatalf("unreferenced media file not removed: %s", solo)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Fatalf("shared media file must survive: %v", err)
	}
	var n int64
	ta.db.Model(&models.MediaAsset{}).Where("post_id = ?", id1).Count(&n)
	if n != 0 {
		t.Fatalf("media rows not cleaned: %d", n)
	}
	ta.db.Model(&models.MediaAsset{}).Where("post_id = ?", id2).Count(&n)
	if n != 1 {
		t.Fatalf("sibling media row must survive, got %d", n)
	}
}

func writeTempMedia(t *testing.T, prefix string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), prefix+"*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

// Deleted posts stop steering Best-time suggestions.
func TestBoostExcludesDeleted(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calboost")
	id := createPost(t, ta,
		`{"content":"steering post","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	// Seed one snapshot directly (no refresh, so no outside-Solomon rows).
	var tgt models.PostTarget
	if err := ta.db.Where("post_id = ?", id).First(&tgt).Error; err != nil {
		t.Fatalf("load target: %v", err)
	}
	now := time.Now()
	ta.db.Create(&models.AnalyticsSnapshot{
		TargetID: tgt.ID, NetworkPostID: tgt.NetworkPostID,
		Views: 10, Likes: 50, Comments: 5, Shares: 2, FetchedAt: now,
	})
	if boost := engagementBoost(ta.db, []models.Network{"twitter"}); len(boost) == 0 {
		t.Fatalf("setup broken: boost must see the published post")
	}
	resp := do(t, ta.app, "DELETE", "/api/posts/"+id, nil, "")
	_ = readBody(t, resp)
	if boost := engagementBoost(ta.db, []models.Network{"twitter"}); len(boost) != 0 {
		t.Fatalf("boost must ignore soft-deleted posts, got %v", boost)
	}
}

// A rule whose pool mixes a soft-deleted post with a live one republishes
// the live one (and advances past the dead entry instead of stalling).
func TestEvergreenMixedPoolAdvances(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@calmix")
	gone := createPost(t, ta,
		`{"content":"dead pool","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	liveID := createPost(t, ta,
		`{"content":"live pool","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "POST", "/api/evergreen",
		strings.NewReader(`{"name":"mix","pool_post_ids":["`+gone+`","`+liveID+`"],"account_ids":["`+acct+`"],"interval_hours":24}`),
		"application/json")
	_ = resp.Body.Close()
	past := time.Now().Add(-time.Hour)
	ta.db.Model(&models.EvergreenRule{}).Where("name = ?", "mix").Update("next_run_at", &past)

	resp = do(t, ta.app, "DELETE", "/api/posts/"+gone, nil, "")
	_ = readBody(t, resp)
	if n := RunDueRules(ta.db); n != 1 {
		t.Fatalf("RunDueRules ran %d, want 1 (live pool post)", n)
	}
	var posts int64
	ta.db.Model(&models.Post{}).Count(&posts)
	if posts != 3 { // 2 originals (1 soft-deleted, still stored) + 1 republished copy
		t.Fatalf("posts %d, want 3", posts)
	}
	var rule models.EvergreenRule
	ta.db.Where("name = ?", "mix").First(&rule)
	if rule.Cursor != 1 {
		t.Fatalf("cursor %d, want 1 (advanced)", rule.Cursor)
	}
}
