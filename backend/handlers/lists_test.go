package handlers

// Regression tests for the "can't access property map, exp is null" frontend crash.
//
// Root cause: Go's encoding/json serializes a nil slice as `null`, not `[]`.
// On a fresh DB every list endpoint returned `null`, and the Svelte frontend
// calls `.map()` / `.length` / `{#each}` on those payloads. These tests pin
// the contract: empty results MUST be JSON arrays (`[]`), never `null`.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/db"
	"solomon/backend/models"
)

// testApp wires the same handlers as main.go against a throwaway DB.
type testApp struct {
	app *fiber.App
	db  *gorm.DB
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	database, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if sqlDB, err := database.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	accounts := &AccountHandler{DB: database}
	posts := &PostHandler{DB: database}
	tokens := &TokenHandler{DB: database}
	analytics := &AnalyticsHandler{DB: database}
	schedule := &ScheduleHandler{DB: database}
	bulk := &BulkHandler{DB: database}
	evergreen := &EvergreenHandler{DB: database}

	app := fiber.New()
	api := app.Group("/api")
	api.Get("/accounts", accounts.List)
	api.Post("/accounts", accounts.Create)
	api.Get("/accounts/expiring", tokens.Expiring)
	api.Post("/accounts/refresh", tokens.RefreshNow)
	api.Get("/posts", posts.List)
	api.Post("/posts", posts.Create)
	api.Post("/posts/preview", posts.Preview)
	api.Post("/posts/bulk", bulk.Import)
	api.Get("/schedule/suggest", schedule.Suggest)
	api.Get("/analytics", analytics.List)
	api.Post("/analytics/refresh", analytics.Refresh)
	api.Get("/evergreen", evergreen.List)
	return &testApp{app: app, db: database}
}

func do(t *testing.T, app *fiber.App, method, url string, body io.Reader, ctype string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// assertJSONArray fails if the raw body is not a JSON array (notably `null`).
func assertJSONArray(t *testing.T, body []byte, what string) []json.RawMessage {
	t.Helper()
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("%s: not a JSON array: %v (body %q)", what, err, string(body))
	}
	if arr == nil {
		t.Fatalf("%s: decoded as null — must be [] (body %q)", what, string(body))
	}
	return arr
}

// Every list endpoint on an empty DB must return [] — never null.
func TestEmptyDBListEndpointsReturnArrays(t *testing.T) {
	ta := newTestApp(t)
	for _, url := range []string{
		"/api/accounts",
		"/api/posts",
		"/api/accounts/expiring",
		"/api/evergreen",
	} {
		resp := do(t, ta.app, "GET", url, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("GET %s: status %d (body %q)", url, resp.StatusCode, string(body))
		}
		if arr := assertJSONArray(t, body, "GET "+url); len(arr) != 0 {
			t.Fatalf("GET %s: expected empty array, got %d items", url, len(arr))
		}
	}
}

func TestAnalyticsEmptyReturnsRowsArray(t *testing.T) {
	ta := newTestApp(t)
	resp := do(t, ta.app, "GET", "/api/analytics", nil, "")
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d (body %q)", resp.StatusCode, string(body))
	}
	var out struct {
		Totals map[string]any    `json:"totals"`
		Rows   []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %q)", err, string(body))
	}
	if out.Rows == nil {
		t.Fatalf("rows decoded as null — must be [] (body %q)", string(body))
	}
}

func TestScheduleSuggestReturnsArray(t *testing.T) {
	ta := newTestApp(t)
	resp := do(t, ta.app, "GET", "/api/schedule/suggest?networks=twitter&count=1", nil, "")
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d (body %q)", resp.StatusCode, string(body))
	}
	var out struct {
		Suggestions []json.RawMessage `json:"suggestions"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %q)", err, string(body))
	}
	if out.Suggestions == nil {
		t.Fatalf("suggestions decoded as null — must be [] (body %q)", string(body))
	}
}

func TestPreviewWithNoTargetsReturnsArray(t *testing.T) {
	ta := newTestApp(t)
	resp := do(t, ta.app, "POST", "/api/posts/preview",
		strings.NewReader(`{"content":"hello","targets":[]}`), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d (body %q)", resp.StatusCode, string(body))
	}
	var out struct {
		Targets []json.RawMessage `json:"targets"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %q)", err, string(body))
	}
	if out.Targets == nil {
		t.Fatalf("targets decoded as null — must be [] (body %q)", string(body))
	}
}

// Both refresh endpoints must return errors: [] — never null — because the
// frontend calls r.errors.length / .join() on them.
func TestRefreshEndpointsReturnErrorsArray(t *testing.T) {
	ta := newTestApp(t)
	for _, url := range []string{"/api/analytics/refresh", "/api/accounts/refresh"} {
		resp := do(t, ta.app, "POST", url, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("POST %s: status %d (body %q)", url, resp.StatusCode, string(body))
		}
		var out struct {
			Errors []json.RawMessage `json:"errors"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("POST %s: decode: %v (body %q)", url, err, string(body))
		}
		if out.Errors == nil {
			t.Fatalf("POST %s: errors decoded as null — must be [] (body %q)", url, string(body))
		}
	}
}

// A bulk import that matches no accounts must still return errors as an array.
func TestBulkImportErrorsAreArray(t *testing.T) {
	ta := newTestApp(t)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(fw, "title,content,link,scheduled_at,accounts\nT1,Bulk one,,,\n")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	resp := do(t, ta.app, "POST", "/api/posts/bulk", &buf, w.FormDataContentType())
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d (body %q)", resp.StatusCode, string(body))
	}
	var out struct {
		Created int               `json:"created"`
		Skipped int               `json:"skipped"`
		Errors  []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %q)", err, string(body))
	}
	if out.Errors == nil {
		t.Fatalf("errors decoded as null — must be [] (body %q)", string(body))
	}
	if out.Skipped == 0 {
		t.Fatalf("expected the unmatched row to be skipped (body %q)", string(body))
	}
}

// A post's nested targets/media must be arrays, never null: the frontend
// does p.media.length and {#each p.targets}.
func TestPostListNestsArraysNotNull(t *testing.T) {
	ta := newTestApp(t)
	resp := do(t, ta.app, "POST", "/api/accounts",
		strings.NewReader(`{"network":"twitter","name":"@test"}`), "application/json")
	acctBody := readBody(t, resp)
	if resp.StatusCode != 201 {
		t.Fatalf("create account: status %d (body %q)", resp.StatusCode, string(acctBody))
	}
	var acct struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(acctBody, &acct); err != nil || acct.ID == "" {
		t.Fatalf("decode account: %v (body %q)", err, string(acctBody))
	}

	resp = do(t, ta.app, "POST", "/api/posts",
		strings.NewReader(`{"content":"hello","targets":[{"account_id":"`+acct.ID+`"}]}`),
		"application/json")
	if resp.StatusCode != 201 {
		t.Fatalf("create post: status %d (body %q)", resp.StatusCode, string(readBody(t, resp)))
	}
	_ = resp.Body.Close()

	resp = do(t, ta.app, "GET", "/api/posts", nil, "")
	body := readBody(t, resp)
	var posts []struct {
		ID      string            `json:"id"`
		Targets []json.RawMessage `json:"targets"`
		Media   []json.RawMessage `json:"media"`
	}
	if err := json.Unmarshal(body, &posts); err != nil {
		t.Fatalf("decode: %v (body %q)", err, string(body))
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d (body %q)", len(posts), string(body))
	}
	if posts[0].Targets == nil {
		t.Fatalf("post.targets is null — must be [] (body %q)", string(body))
	}
	if posts[0].Media == nil {
		t.Fatalf("post.media is null — must be [] (body %q)", string(body))
	}
}

// An expired account must show up in /expiring as a real array (non-empty path).
func TestExpiringIncludesExpiredAccount(t *testing.T) {
	ta := newTestApp(t)
	past := time.Now().Add(-time.Hour)
	if err := ta.db.Create(&models.SocialAccount{
		Network: "twitter", Name: "@old", AccessToken: "demo-twitter", ExpiresAt: &past,
	}).Error; err != nil {
		t.Fatalf("seed expired account: %v", err)
	}
	resp := do(t, ta.app, "GET", "/api/accounts/expiring", nil, "")
	body := readBody(t, resp)
	arr := assertJSONArray(t, body, "GET /api/accounts/expiring")
	if len(arr) != 1 {
		t.Fatalf("expected 1 expiring account, got %d (body %q)", len(arr), string(body))
	}
}
