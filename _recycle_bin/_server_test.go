package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

// Test GetPing endpoint returns correct pong response
func TestGetPing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	GetPing(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal("invalid JSON response")
	}

	if body["ping"] != "pong" {
		t.Errorf("expected ping=pong, got %v", body)
	}
}

// Test CORS middleware adds expected headers
func TestEnableCORS(t *testing.T) {
	handler := enableCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS header missing")
	}
}

// Test shuffleUsersIntoGroups assigns users correctly
func TestShuffleUsersIntoGroups(t *testing.T) {
	wsMutex.Lock()
	activeUsers = map[string]bool{
		"alice": true,
		"bob":   true,
		"carol": true,
		"dave":  true,
	}
	wsMutex.Unlock()

	shuffleUsersIntoGroups()

	if len(userGroups) == 0 {
		t.Error("userGroups should not be empty after shuffle")
	}

	total := 0
	for _, group := range userGroups {
		total += len(group)
	}

	if total != 4 {
		t.Errorf("expected 4 users grouped, got %d", total)
	}
}

// Dummy WebSocketConn mock to prevent actual network writes
type dummyConn struct{}

func (d *dummyConn) WriteMessage(messageType int, data []byte) error { return nil }
func (d *dummyConn) WriteJSON(v interface{}) error                    { return nil }
func (d *dummyConn) Close() error                                     { return nil }
func (d *dummyConn) ReadJSON(v interface{}) error                     { return io.EOF } // simulate disconnect

// Test broadcastUserList does not panic with dummy conns
func TestBroadcastUserListSafe(t *testing.T) {
	wsMutex.Lock()
	activeUsers = map[string]bool{
		"alice": true,
		"bob":   true,
	}
	activeUserConnections = map[string]*websocket.Conn{
		"alice": nil, // simulate nil or closed conn
	}
	wsMutex.Unlock()

	// Should not panic
	broadcastUserList()
}

// Test broadcastUserGroups does not panic with dummy conns
func TestBroadcastUserGroupsSafe(t *testing.T) {
	wsMutex.Lock()
	activeUserConnections = map[string]*websocket.Conn{
		"bob": nil,
	}
	userGroups = map[string][]string{
		"group0": {"alice", "bob"},
	}
	wsMutex.Unlock()

	broadcastUserGroups()
}
