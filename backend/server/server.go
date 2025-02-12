package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ChatServer implements the generated API interface
type ChatServer struct{}

var wsMutex sync.Mutex
var activeUsers = make(map[string]bool)                      // Tracks active users

// WebSocket Handler
func ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username, ok := claims["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	// Store WebSocket connection
	wsMutex.Lock()
	activeUserConnections[username] = conn
	activeUsers[username] = true
	wsMutex.Unlock()

	log.Println(username, "connected to WebSocket")

	// Broadcast active users
	broadcastUserList()

	defer func() {
		// Cleanup on disconnect
		wsMutex.Lock()
		delete(activeUserConnections, username)
		delete(activeUsers, username)
		wsMutex.Unlock()

		conn.Close()
		log.Println(username, "disconnected from WebSocket")

		// Broadcast updated user list
		broadcastUserList()
	}()

	// Listen for incoming messages
	for {
		var msg map[string]string
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		recipient := msg["to"]
		message := msg["message"]

		// Send private message if recipient exists
		wsMutex.Lock()
		if recipientConn, exists := activeUserConnections[recipient]; exists {
			recipientConn.WriteJSON(map[string]string{"from": username, "message": message})
		}
		wsMutex.Unlock()
	}
}

// Broadcast active users to all clients
func broadcastUserList() {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	users := []string{}
	for user := range activeUsers {
		users = append(users, user)
	}

	message, _ := json.Marshal(map[string]interface{}{
		"type":  "userList",
		"users": users,
		"count": len(users),
	})

	// Send update to all clients
	for _, conn := range activeUserConnections {
		conn.WriteMessage(websocket.TextMessage, message)
	}
}

// Ping Endpoint
func GetPing(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"ping": "pong"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// StartServer initializes the HTTP + WebSocket server
func StartServer() {
	mux := http.NewServeMux()

	// Register REST API endpoints
	mux.HandleFunc("/ping", GetPing)

	// Register authentication endpoints
	mux.HandleFunc("/login", LoginHandler)
	mux.HandleFunc("/me", UserInfoHandler)
	mux.HandleFunc("/logout", LogoutHandler)

	// WebSocket endpoint
	mux.HandleFunc("/ws", ChatWebSocket)

	// Wrap CORS middleware
	handler := enableCORS(mux)

	log.Println("Server running on http://0.0.0.0:8080 (HTTP + WebSockets)")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", handler))
}
