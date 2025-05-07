package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"backend/api"
	"backend/ops"
)

// helper: fail test on error
func must[T any](t *testing.T, v T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return v
}

func TestFullFlow(t *testing.T) {
	// ── spin up the server in‑process ───────────────────────────────
	impl := ops.New()
	handler := api.Handler(impl) // no CORS for unit tests
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx := context.Background()
	c, _ := api.NewClientWithResponses(ts.URL)

	time.Sleep(3*time.Second)

	// ── 1. anonymous session ───────────────────────────────────────
	resp, err := c.PostSessionAnonymousWithResponse(ctx,
		api.AnonymousSessionRequest{},
	)
	must(t, resp, err) // must(T, error)
	anon := resp.JSON201

	if anon == nil {
		t.Fatalf("expected 201, got %+v", anon)
	}
	if anon.Token == "" || anon.WebsocketUrl == "" {
		t.Fatalf("missing token or ws url")
	}

	// ── 2. /me with the short‑lived token ──────────────────────────
	auth := func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+anon.Token)
		return nil
	}
	meResp, err := c.GetMeWithResponse(ctx, auth)
	me := must(t, meResp, err).JSON200

	if me == nil || me.Id == "" {
		t.Fatalf("/me unexpected payload: %+v", me)
	}

	// 3. register unique username
	regRespWrap, err := c.PostAccountRegisterWithResponse(
		ctx,
		api.RegisterRequest{Username: "alice", Password: "ignored"},
		func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", "Bearer "+anon.Token)
			return nil
		},
	)
	reg := must(t, regRespWrap, err) // must(t, value, err)

	if reg.JSON409 != nil {
		t.Fatalf("username not unique?")
	}
	token := reg.JSON201.Token


	// ── 4. login again (proves /login) ─────────────────────────────
	loginResp, err := c.PostLoginWithResponse(ctx, api.LoginRequest{Username: "alice", Password: "ignored"})
	login := must(t, loginResp, err)
	
	if login.JSON200 == nil || login.JSON200.Token == "" {
		t.Fatalf("login failed: %+v", login)
	}

	// ── 5. GET /ping (no auth) ─────────────────────────────────────
	pingResp, err := c.GetPingWithResponse(ctx)
	ping := must(t, pingResp, err)

	if ping.JSON200.Pong != "pong" {
		t.Fatalf("bad ping payload: %+v", ping.JSON200)
	}

	// ── 6. open WebSocket using token ──────────────────────────────
	wsURL := strings.Replace(anon.WebsocketUrl, "http://", "ws://", 1)
	dialer := websocket.Dialer{}
	ws, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket upgrade failed: %v", err)
	}
	defer ws.Close()

	// expect a “paired” welcome within 2 s
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var welcome api.ChatMessage
	if err := ws.ReadJSON(&welcome); err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if welcome.Type != api.Chat || welcome.Message != "riding solo" {
		t.Fatalf("unexpected ws msg: %+v", welcome)
	}

	// ── 7. /session/skip (should succeed) ──────────────────────────
	skipResp, err := c.PostSessionSkipWithResponse(ctx, func(_ context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
	skip := must(t, skipResp, err)

	if skip.StatusCode() != 204 {
		t.Fatalf("skip failed: %#v", skip)
	}
}
