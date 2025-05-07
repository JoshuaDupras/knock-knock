package server

import (
	"backend/api"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

func TestE2E_Server(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		StartServer()
	}()

	time.Sleep(500 * time.Millisecond)

	// Sanity check: /ping
	resp, err := http.Get("http://localhost:8080/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	// Create JWT for user1 and user2
	tokenStr1 := mustGenerateToken("user1")
	tokenStr2 := mustGenerateToken("user2")

	ws1 := mustDialWebSocket(t, tokenStr1)
	defer ws1.Close()
	ws2 := mustDialWebSocket(t, tokenStr2)
	defer ws2.Close()

	// Drain group broadcast for both
	_ = mustReadMessage(t, ws1)
	_ = mustReadMessage(t, ws2)

	// user1 sends message to user2
	msg := map[string]string{
		"to":      "user2",
		"message": "hello from user1",
	}
	err = ws1.WriteJSON(msg)
	if err != nil {
		t.Fatalf("user1 failed to send: %v", err)
	}

	// user2 should receive it
	ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var incoming map[string]string
	err = ws2.ReadJSON(&incoming)
	if err != nil {
		t.Fatalf("user2 failed to receive message: %v", err)
	}

	// Validate contents
	if incoming["from"] != "user1" || incoming["message"] != "hello from user1" {
		t.Errorf("unexpected message: %+v", incoming)
	}

	// Clean close
	_ = ws1.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	_ = ws2.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
}

func mustGenerateToken(username string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(5 * time.Minute).Unix(),
	})
	str, err := token.SignedString(jwtSecret)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate token: %v", err))
	}
	return str
}

func mustDialWebSocket(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	url := fmt.Sprintf("ws://localhost:8080/ws?token=%s", token)
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	return ws
}

func mustReadMessage(t *testing.T, ws *websocket.Conn) map[string]interface{} {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]interface{}
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}
	return msg
}

func TestAuthEndpoints(t *testing.T) {
	// Start test server
	mux := http.NewServeMux()
	mux.HandleFunc("/login", LoginHandler)
	mux.HandleFunc("/me", UserInfoHandler)
	mux.HandleFunc("/logout", LogoutHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	client := server.Client()

	// === Step 1: Login with valid credentials ===
	loginReq := api.LoginRequest{
		Username:          "user1",
		PlaintextPassword: "password", // matches the bcrypt hash
	}
	body, _ := json.Marshal(loginReq)

	resp, err := client.Post(server.URL+"/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on login, got %d", resp.StatusCode)
	}

	var loginResp api.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Invalid login response JSON: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("Expected non-empty JWT token")
	}
	t.Logf("Received token: %s", loginResp.Token)

	// === Step 2: Access /me with token ===
	req, _ := http.NewRequest("GET", server.URL+"/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)

	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request to /me failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK from /me, got %d", resp2.StatusCode)
	}

	var userInfo map[string]string
	if err := json.NewDecoder(resp2.Body).Decode(&userInfo); err != nil {
		t.Fatalf("Invalid /me JSON response: %v", err)
	}
	if userInfo["username"] != "user1" {
		t.Errorf("Expected username user1, got %s", userInfo["username"])
	}

	// === Step 3: Logout with same token ===
	req3, _ := http.NewRequest("POST", server.URL+"/logout", nil)
	req3.Header.Set("Authorization", loginResp.Token) // note: logout handler uses raw token

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("Request to /logout failed: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK from /logout, got %d", resp3.StatusCode)
	}
}
