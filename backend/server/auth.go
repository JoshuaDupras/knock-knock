package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"backend/api"
)

var jwtSecret = []byte("supersecretkey") // Change to a secure env-based key

var testHash = "$2a$10$bm3n66QHwjr78N1rnyg2tuXeWJfJiJhajtd9yL2V3Y3b9B5ZvZQeW" // bcrypt of "password"

var users = map[string]api.User{
	"admin": {
		Id:           "0",
		Username:     "admin",
		PasswordHash: testHash,
	},
	"user1": {
		Id:           "1",
		Username:     "user1",
		PasswordHash: testHash,
	},
	"user2": {
		Id:           "2",
		Username:     "user2",
		PasswordHash: testHash,
	},
}

// Get user info from JWT
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		logger.Warn("Missing Authorization header")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		logger.Warn("Invalid Authorization format")
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		logger.Warn("Invalid token", slog.String("error", err.Error()))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := claims["username"].(string)
	logger.Info("Authenticated /me", slog.String("username", username))

	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

// Mutex to prevent concurrent map writes
var activeUserConnectionsMutex sync.Mutex
var activeUserConnections = make(map[string]*websocket.Conn) // Stores WebSocket connections

func GetactiveUserConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	activeUserConnectionsMutex.Lock()
	defer activeUserConnectionsMutex.Unlock()

	users := []api.User{}
	for username := range activeUserConnections {
		users = append(users, api.User{Id: username, Username: username})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]api.User{"users": users})

	logger.Debug("Fetched active user connections", slog.Int("count", len(users)))
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("Failed to decode login request", slog.String("error", err.Error()))
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, exists := users[req.Username]
	if !exists {
		logger.Warn("Invalid login attempt", slog.String("username", req.Username))
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.PlaintextPassword)); err != nil {
		logger.Warn("Incorrect password", slog.String("username", req.Username))
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token, err := generateJWT(req.Username)
	if err != nil {
		logger.Error("JWT generation failed", slog.String("username", req.Username), slog.String("error", err.Error()))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	logger.Info("Login successful", slog.String("username", req.Username))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.LoginResponse{Token: token})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		logger.Warn("Logout called with no token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		logger.Warn("Logout with invalid token", slog.String("error", err.Error()))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username, ok := claims["username"].(string)
	if !ok {
		logger.Warn("Logout token missing username claim")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	activeUserConnectionsMutex.Lock()
	delete(activeUserConnections, username)
	activeUserConnectionsMutex.Unlock()

	logger.Info("User logged out", slog.String("username", username))
	w.WriteHeader(http.StatusOK)
}

func generateJWT(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash password", slog.String("error", err.Error()))
		return "", err
	}
	logger.Debug("Password hashed")
	return string(hashedPassword), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.URL.Query().Get("token")
		if tokenString == "" {
			logger.Warn("WebSocket JWT missing")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			logger.Warn("WebSocket JWT invalid", slog.String("error", err.Error()))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		username := claims["username"].(string)
		logger.Info("WebSocket JWT authorized", slog.String("username", username))
		next(w, r)
	}
}
