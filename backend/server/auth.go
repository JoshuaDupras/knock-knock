package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"backend/api"
)

var jwtSecret = []byte("supersecretkey") // Change this to a secure env-based key

var testHash = "$2a$10$bm3n66QHwjr78N1rnyg2tuXeWJfJiJhajtd9yL2V3Y3b9B5ZvZQeW" // bcrypt hashed password: "password"

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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Ensure it starts with "Bearer "
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]

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
var activeUserConnectionsMutex sync.Mutex
var activeUserConnections = make(map[string]*websocket.Conn) // Stores WebSocket connections
var activeUserConnectionsMap = make(map[string]bool)         // Tracks active users (logged-in)

// Get active users (returns a list of User objects)
func GetactiveUserConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	activeUserConnectionsMutex.Lock()
	defer activeUserConnectionsMutex.Unlock()

	users := []api.User{}
	for username := range activeUserConnections {
		users = append(users, api.User{Id: username, Username: username}) // Placeholder ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]api.User{"users": users})
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req api.LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    user, exists := users[req.Username]
    if !exists {
        http.Error(w, "Invalid username or password", http.StatusUnauthorized)
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.PlaintextPassword)); err != nil {
        http.Error(w, "Invalid username or password", http.StatusUnauthorized)
        return
    }

    token, err := generateJWT(req.Username)
    if err != nil {
        http.Error(w, "Failed to generate token", http.StatusInternalServerError)
        return
    }

    res := api.LoginResponse{Token: token}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(res)
}


// HashPassword securely hashes a password
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// CheckPasswordHash compares a plain text password with a hashed password
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
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

	username, ok := claims["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Remove user from active list
	activeUserConnectionsMutex.Lock()
	delete(activeUserConnections, username)
	activeUserConnectionsMutex.Unlock()

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