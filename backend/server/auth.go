package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("supersecretkey") // Change this to a secure env-based key

// User struct
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Hardcoded user database (replace with DB later)
var users = map[string]string{
	"admin": "$2a$10$bm3n66QHwjr78N1rnyg2tuXeWJfJiJhajtd9yL2V3Y3b9B5ZvZQeW", // bcrypt hashed password: "password"
	"user1": "$2a$10$bm3n66QHwjr78N1rnyg2tuXeWJfJiJhajtd9yL2V3Y3b9B5ZvZQeW", // bcrypt hashed password: "password"
	"user2": "$2a$10$bm3n66QHwjr78N1rnyg2tuXeWJfJiJhajtd9yL2V3Y3b9B5ZvZQeW", // bcrypt hashed password: "password"
}

// Get user info from JWT
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
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
	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

// Mutex to prevent concurrent map writes
var activeUsersMutex sync.Mutex
var activeUsers = make(map[string]*websocket.Conn) // Stores WebSocket connections
var activeUsersMap = make(map[string]bool)         // Tracks active users (logged-in)

// Get active users
func GetActiveUsersHandler(w http.ResponseWriter, r *http.Request) {
	activeUsersMutex.Lock()
	defer activeUsersMutex.Unlock()

	userList := []string{}
	for user := range activeUsersMap { // Use activeUsersMap to track logged-in users
		userList = append(userList, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"users": userList})
}

// Modify login handler to track logged-in users
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	hashedPassword, exists := users[user.Username]
	if !exists || bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(user.Password)) != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := generateJWT(user.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Mark user as active
	activeUsersMutex.Lock()
	activeUsersMap[user.Username] = true // ✅ Correctly track logged-in users
	activeUsersMutex.Unlock()

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}


// Modify logout handler to remove users from active list
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
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

	// Remove user from active list
	activeUsersMutex.Lock()
	delete(activeUsers, username)
	activeUsersMutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

// Generate JWT token
func generateJWT(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// JWT Middleware for WebSockets
func JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from query parameters instead of headers
		query := r.URL.Query()
		tokenString := query.Get("token")

		if tokenString == "" {
			log.Println("Unauthorized user, no token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			log.Println("Unauthorized user:", claims["username"])
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Println("Authenticated WebSocket user:", claims["username"])
		next(w, r)
	}
}