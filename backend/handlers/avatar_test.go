package handlers

// Avatar serialization: avatar_url flows through every account payload
// (demo accounts carry ""), and analytics rows carry it for the UI avatars.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountAvatarSerialized(t *testing.T) {
	ta := newTestApp(t)
	resp := do(t, ta.app, "POST", "/api/accounts",
		strings.NewReader(`{"network":"instagram","name":"@pics"}`), "application/json")
	body := readBody(t, resp)
	if resp.StatusCode != 201 {
		t.Fatalf("status %d (body %q)", resp.StatusCode, string(body))
	}
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if _, ok := created["avatar_url"]; !ok {
		t.Fatalf("create response missing avatar_url (body %q)", string(body))
	}
	if created["avatar_url"] != "" {
		t.Fatalf("demo account avatar_url %q, want empty (initials UI)", created["avatar_url"])
	}

	resp = do(t, ta.app, "GET", "/api/accounts", nil, "")
	lbody := readBody(t, resp)
	var list []map[string]any
	if err := json.Unmarshal(lbody, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 account, got %d", len(list))
	}
	if _, ok := list[0]["avatar_url"]; !ok {
		t.Fatalf("list row missing avatar_url: %v", list[0])
	}
	if _, ok := list[0]["access_token"]; ok {
		t.Fatalf("tokens must stay blanked: %v", list[0])
	}
}

func TestAnalyticsRowAvatarSerialized(t *testing.T) {
	ta := newTestApp(t)
	acct := createAccount(t, ta, "twitter", "@avrow")
	createPost(t, ta,
		`{"content":"avatar row","publish_now":true,"targets":[{"account_id":"`+acct+`"}]}`,
		201)
	resp := do(t, ta.app, "GET", "/api/analytics", nil, "")
	body := readBody(t, resp)
	var out struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range out.Rows {
		if r["external"] == true {
			continue
		}
		found = true
		if _, ok := r["avatar_url"]; !ok {
			t.Fatalf("app analytics row missing avatar_url: %v", r)
		}
	}
	if !found {
		t.Fatalf("no app analytics rows")
	}
}
