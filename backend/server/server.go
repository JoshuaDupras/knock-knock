package server

import (
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// Logger instance (can be set to Text or JSON based on environment)
var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug, // Change to LevelInfo to reduce verbosity
}))

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ChatServer implements the generated API interface
type ChatServer struct{}

var wsMutex sync.Mutex
var activeUsers = make(map[string]bool)                   // Tracks active users
var userGroups = make(map[string][]string)                // GroupID to list of users

// WebSocket Handler
func ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		logger.Warn("Missing token", slog.String("path", r.URL.Path))
		return
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		logger.Warn("Invalid JWT", slog.String("error", err.Error()))
		return
	}

	username, ok := claims["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		logger.Warn("Missing username claim")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	// Store WebSocket connection
	wsMutex.Lock()
	activeUserConnections[username] = conn
	activeUsers[username] = true
	wsMutex.Unlock()

	logger.Info("WebSocket connected", slog.String("username", username))

	// Broadcast updated groups
	broadcastUserGroups()

	defer func() {
		wsMutex.Lock()
		delete(activeUserConnections, username)
		delete(activeUsers, username)
		wsMutex.Unlock()
		logger.Info("WebSocket disconnected", slog.String("username", username))
		broadcastUserGroups()
		conn.Close()
	}()

	// Listen for incoming messages
	for {
		var msg map[string]string
		err := conn.ReadJSON(&msg)
		if err != nil {
			logger.Debug("Error reading message", slog.String("username", username), slog.String("error", err.Error()))
			break
		}

		recipient := msg["to"]
		message := msg["message"]

		logger.Debug("Incoming message",
			slog.String("from", username),
			slog.String("to", recipient),
			slog.String("message", message),
		)

		// Send private message if recipient exists
		wsMutex.Lock()
		if recipientConn, exists := activeUserConnections[recipient]; exists {
			err := recipientConn.WriteJSON(map[string]string{"from": username, "message": message})
			if err != nil {
				logger.Warn("Failed to deliver message",
					slog.String("to", recipient),
					slog.String("error", err.Error()))
			}
		} else {
			logger.Debug("Recipient not connected", slog.String("recipient", recipient))
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

	for username, conn := range activeUserConnections {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			logger.Warn("Failed to send user list", slog.String("username", username), slog.String("error", err.Error()))
		}
	}

	logger.Info("Broadcasted user list", slog.Int("userCount", len(users)))
}

func startGroupShuffle() {
	for {
		time.Sleep(3 * time.Minute)
		wsMutex.Lock()
		shuffleUsersIntoGroups()
		broadcastUserGroups()
		wsMutex.Unlock()
	}
}

func shuffleUsersIntoGroups() {
	users := make([]string, 0, len(activeUsers))
	for u := range activeUsers {
		users = append(users, u)
	}
	rand.Shuffle(len(users), func(i, j int) { users[i], users[j] = users[j], users[i] })

	userGroups = make(map[string][]string)
	for i, user := range users {
		groupID := "group" + string(rune(i/2)) // Group of 2 for now
		userGroups[groupID] = append(userGroups[groupID], user)
	}

	logger.Debug("Users shuffled into groups", slog.Int("userCount", len(users)), slog.Int("groupCount", len(userGroups)))
}

func broadcastUserGroups() {
	message, _ := json.Marshal(map[string]interface{}{
		"type":   "userGroups",
		"groups": userGroups,
	})

	for username, conn := range activeUserConnections {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			logger.Warn("Failed to broadcast group update", slog.String("username", username), slog.String("error", err.Error()))
		}
	}

	logger.Info("Broadcasted user groups", slog.Int("groupCount", len(userGroups)))
}

// Ping Endpoint
func GetPing(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"ping": "pong"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logger.Debug("Ping response sent")
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			logger.Debug("CORS preflight", slog.String("path", r.URL.Path))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// StartServer initializes the HTTP + WebSocket server
func StartServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", GetPing)
	mux.HandleFunc("/login", LoginHandler)
	mux.HandleFunc("/me", UserInfoHandler)
	mux.HandleFunc("/logout", LogoutHandler)
	mux.HandleFunc("/ws", ChatWebSocket)

	handler := enableCORS(mux)

	logger.Info("Server starting", slog.String("address", "http://0.0.0.0:8080"))
	if err := http.ListenAndServe("0.0.0.0:8080", handler); err != nil {
		logger.Error("Server failed", slog.String("error", err.Error()))
	}
}
