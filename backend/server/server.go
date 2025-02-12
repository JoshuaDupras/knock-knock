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

// Handle WebSocket connection
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

	username := claims["username"].(string)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	// Store WebSocket connection for messaging
	wsMutex.Lock()
	activeUserConnections[username] = conn
	wsMutex.Unlock()

	log.Println(username, "connected to WebSocket")

	defer func() {
		wsMutex.Lock()
		delete(activeUserConnections, username) // Remove WebSocket connection on disconnect
		delete(activeUserConnectionsMap, username) // Remove from active users list
		wsMutex.Unlock()
		conn.Close()
		log.Println(username, "disconnected from WebSocket")
	}()

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



// Ping responds with a JSON "pong"
func GetPing(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"ping": "pong"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
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
	mux.HandleFunc("/users", GetactiveUserConnectionsHandler)

	// Protect WebSocket with JWT middleware
	mux.HandleFunc("/ws", JWTMiddleware(ChatWebSocket))

	// Wrap CORS middleware around handlers
	handler := enableCORS(mux)

	// Start the HTTP + WebSocket server
	log.Println("Server running on http://0.0.0.0:8080 (HTTP + WebSockets)")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", handler))
}
