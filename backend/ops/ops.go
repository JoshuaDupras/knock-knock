// backend/ops/ops.go
// Ephemeral Chat reference server that conforms to the OpenAPI spec in api/.
// Focus: minimal but functional flows for /session/anonymous, /account/register,
// /login, /me, /session/skip, /ping, and the WebSocket chat endpoint.
// Users start anonymous, are paired 1‑on‑1 for 3‑minute rounds, may skip, and
// may register a unique username to persist.

package ops

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"backend/api"
)

// ─── CONFIG ────────────────────────────────────────────────────────────────
var (
    jwtKey           = []byte("super‑secret‑dev‑key")
    anonSessionTTL   = 100 * 365 * 24 * time.Hour
    registeredTTL    = 24 * time.Hour
    roundDuration    = 3 * time.Minute
    skipCooldown     = 10 * time.Second // throttle rapid skips → 429
)

// ─── MODELS ────────────────────────────────────────────────────────────────

type user struct {
    ID           string
    Username     string // empty until registered
    lastSkipTime time.Time
    conn         *websocket.Conn
}

type conversation struct {
    ID           string
    Participants []*user // always size ≥2 (1‑on‑1 for now)
    timer        *time.Timer
    expiresAt    time.Time
}

// ─── IN‑MEMORY STORE ───────────────────────────────────────────────────────

var (
    mu            sync.RWMutex
    usersByID     = map[string]*user{}
    usersByName   = map[string]*user{}
    waitingQueue  []*user
    conversations = map[string]*conversation{}
)

// ─── HELPERS ───────────────────────────────────────────────────────────────

func genID() string {
    slog.Info("Generating new ID")
    b := make([]byte, 12)
    _, _ = rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}

func issueJWT(u *user, ttl time.Duration) (string, error) {
    slog.Info("Issuing JWT", "userID", u.ID, "username", u.Username, "ttl", ttl)
    claims := jwt.MapClaims{
        "sub":      u.ID,
        "username": u.Username,
        "exp":      time.Now().Add(ttl).Unix(),
    }
    return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtKey)
}

func userFromJWT(tokenStr string) (*user, error) {
    slog.Info("Parsing JWT", "token", tokenStr)
    tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return jwtKey, nil })
    if err != nil || !tok.Valid {
        slog.Error("Invalid JWT", "error", err)
        return nil, err
    }
    claims := tok.Claims.(jwt.MapClaims)
    id := claims["sub"].(string)
    u := usersByID[id]
    slog.Info("User retrieved from JWT", "userID", id)
    return u, nil
}

// ─── PAIRING LOGIC ─────────────────────────────────────────────────────────
func tryPair() {
    slog.Info("Attempting to pair users")
    mu.Lock()
    defer mu.Unlock()

    // 1) Log current queue
    slog.Info("Current waiting queue", "queueLength", len(waitingQueue), "userIDs", func() []string {
        ids := make([]string, len(waitingQueue))
        for i, u := range waitingQueue {
            ids[i] = u.ID
        }
        return ids
    }())

    // 2) Keep pairing as long as we can find two connected users
    for {
        n := len(waitingQueue)
        var i, j int
        found := false

        // find two distinct indices whose conn != nil
        for x := 0; x < n-1 && !found; x++ {
            u1 := waitingQueue[x]
            if u1.conn == nil {
                continue
            }
            for y := x + 1; y < n; y++ {
                u2 := waitingQueue[y]
                if u2.conn == nil {
                    continue
                }
                if u1.ID != u2.ID {
                    i, j = x, y
                    found = true
                    break
                }
            }
        }

        if !found {
            slog.Info("Not enough connected users to pair; stopping")
            break
        }

        // 3) Extract the two users and remove them (higher index first)
        a, b := waitingQueue[i], waitingQueue[j]
        slog.Info("Pairing users", "userA", a.ID, "userB", b.ID)
        if j > i {
            waitingQueue = append(waitingQueue[:j], waitingQueue[j+1:]...)
            waitingQueue = append(waitingQueue[:i], waitingQueue[i+1:]...)
        } else {
            waitingQueue = append(waitingQueue[:i], waitingQueue[i+1:]...)
            waitingQueue = append(waitingQueue[:j], waitingQueue[j+1:]...)
        }

        // 4) Create and record the conversation
        conv := &conversation{
            ID:           genID(),
            Participants: []*user{a, b},
            expiresAt:    time.Now().Add(roundDuration),
        }
        conversations[conv.ID] = conv
        slog.Info("Created conversation", "conversationID", conv.ID)

        // 5) Notify both participants
        now := time.Now().UTC()
        for _, p := range conv.Participants {
            text := fmt.Sprintf("paired with %s", func() string {
                if conv.Participants[0] == p {
                    return conv.Participants[1].ID
                }
                return conv.Participants[0].ID
            }())
            msg := api.ChatMessage{
                Type:           api.Paired,
                ConversationId: conv.ID,
                Message:        &text,
                Timestamp:      &now,
                ExpiresAt:      &conv.expiresAt,
            }
            // p.conn is guaranteed non-nil here
            if err := p.conn.WriteJSON(msg); err != nil {
                slog.Error("Failed to send pairing notification", "userID", p.ID, "error", err)
            } else {
                slog.Info("Sent pairing notification", "userID", p.ID)
            }
        }

        // 6) Schedule automatic timeout
        conv.timer = time.AfterFunc(roundDuration, func() {
            slog.Info("Conversation timed out", "conversationID", conv.ID)
            mu.Lock()
            delete(conversations, conv.ID)
            waitingQueue = append(waitingQueue, a, b)
            mu.Unlock()

            // send time_up to anyone still connected
            now := time.Now().UTC()
            notify := api.ChatMessage{
                Type:           api.TimeUp,
                ConversationId: conv.ID,
                Timestamp:      &now,
            }
            for _, p := range []*user{a, b} {
                if p.conn != nil {
                    if err := p.conn.WriteJSON(notify); err != nil {
                        slog.Warn("Failed to send time_up", "userID", p.ID, "error", err)
                    }
                }
            }

            // try to form new pairs
            tryPair()
        })

        // loop around to see if we can pair more users...
    }
}




// ─── SERVER IMPLEMENTATION (api.ServerInterface) ──────────────────────────

// Server implements every handler in api.ServerInterface.
type Server struct{}

// Compile‑time proof that *Server satisfies the interface.
var _ api.ServerInterface = (*Server)(nil)

// constructor – makes it easy for main/server package
func New() *Server { return &Server{} }

// POST /session/anonymous
func (s *Server) PostSessionAnonymous(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling POST /session/anonymous")
    // var req api.AnonymousSessionRequest
    // if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    //     slog.Error("Failed to decode request", "error", err)
    //     http.Error(w, err.Error(), http.StatusBadRequest)
    //     return
    // }

    u := &user{ID: genID()}
    mu.Lock()
    usersByID[u.ID] = u
    waitingQueue = append(waitingQueue, u)
    mu.Unlock()

    token, _ := issueJWT(u, anonSessionTTL)
    tryPair()

    resp := api.AnonymousSessionResponse{
        Token:            token,
        WebsocketUrl:     "ws://" + r.Host + "/ws/chat?token=" + token,
        ExpiresInSeconds: int32(anonSessionTTL.Seconds()),
        ConversationId:   nil,
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(resp)
    slog.Info("Anonymous session created", "userID", u.ID)
}

// POST /account/register
func (s *Server) PostAccountRegister(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling POST /account/register")
    var req api.RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        slog.Error("Failed to decode request", "error", err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    mu.Lock()
    if _, exists := usersByName[req.Username]; exists {
        mu.Unlock()
        slog.Warn("Username already exists", "username", req.Username)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusConflict)
        _ = json.NewEncoder(w).Encode(api.Error{Error: "username_exists"})
        return
    }
    bearer := r.Header.Get("Authorization")
    var u *user
    if strings.HasPrefix(bearer, "Bearer ") {
        u, _ = userFromJWT(strings.TrimPrefix(bearer, "Bearer "))
    }
    if u == nil {
        u = &user{ID: genID()}
        usersByID[u.ID] = u
    }
    u.Username = req.Username
    usersByName[u.Username] = u
    mu.Unlock()

    tok, _ := issueJWT(u, registeredTTL)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(api.AuthResponse{Token: tok})
    slog.Info("User registered", "userID", u.ID, "username", u.Username)
}

// POST /login — demo: username only, no password DB
func (s *Server) PostLogin(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling POST /login")
    var req api.LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        slog.Error("Failed to decode request", "error", err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    mu.RLock()
    u := usersByName[req.Username]
    mu.RUnlock()
    if u == nil {
        slog.Warn("Invalid login credentials", "username", req.Username)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        _ = json.NewEncoder(w).Encode(api.Error{Error: "invalid_credentials"})
        return
    }
    tok, _ := issueJWT(u, registeredTTL)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(api.AuthResponse{Token: tok})
    slog.Info("User logged in", "userID", u.ID, "username", u.Username)
}

// GET /me
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling GET /me")
    bearer := r.Header.Get("Authorization")
    if !strings.HasPrefix(bearer, "Bearer ") {
        slog.Warn("Missing token in request")
        http.Error(w, "missing token", http.StatusUnauthorized)
        return
    }
    u, err := userFromJWT(strings.TrimPrefix(bearer, "Bearer "))
    if err != nil || u == nil {
        slog.Warn("Invalid token", "error", err)
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(api.User{Id: u.ID, Username: u.Username})
    slog.Info("User info retrieved", "userID", u.ID, "username", u.Username)
}

// GET /ping
func (s *Server) GetPing(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling GET /ping")
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(api.Pong{Ping: "pong"})
}

// POST /session/skip
func (s *Server) PostSessionSkip(w http.ResponseWriter, r *http.Request) {
    slog.Info("Handling POST /session/skip")
    bearer := r.Header.Get("Authorization")
    if !strings.HasPrefix(bearer, "Bearer ") {
        slog.Warn("Missing token in request")
        http.Error(w, "", http.StatusUnauthorized)
        return
    }
    u, err := userFromJWT(strings.TrimPrefix(bearer, "Bearer "))
    if err != nil || u == nil {
        slog.Warn("Invalid token", "error", err)
        http.Error(w, "", http.StatusUnauthorized)
        return
    }

    mu.Lock()
    if time.Since(u.lastSkipTime) < skipCooldown {
        mu.Unlock()
        slog.Warn("Skip rate limited", "userID", u.ID)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        _ = json.NewEncoder(w).Encode(api.Error{Error: "skip_rate_limited"})
        return
    }
    u.lastSkipTime = time.Now()
    for id, c := range conversations {
        for i, p := range c.Participants {
            if p == u {
                c.Participants = append(c.Participants[:i], c.Participants[i+1:]...)
            }
        }
        if len(c.Participants) < 2 {
            c.timer.Stop()
            delete(conversations, id)
            waitingQueue = append(waitingQueue, c.Participants...)
        }
    }
    waitingQueue = append(waitingQueue, u)
    mu.Unlock()
    tryPair()
    w.WriteHeader(http.StatusNoContent)
    slog.Info("User skipped session", "userID", u.ID)
}

// GET /ws/chat
func (s *Server) GetWsChat(w http.ResponseWriter, r *http.Request, params api.GetWsChatParams) {
    slog.Info("Handling GET /ws/chat", "token", params.Token)
    u, err := userFromJWT(params.Token)
    if err != nil || u == nil {
        slog.Warn("Invalid token", "error", err)
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }
    upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        slog.Error("Failed to upgrade connection", "error", err)
        return
    }
    u.conn = conn
    slog.Info("WebSocket connection established", "userID", u.ID)

    go tryPair()

    go func() {
        defer func() {
            conn.Close()
            u.conn = nil
            slog.Info("WebSocket connection closed", "userID", u.ID)
        }()
        for {
            var msg api.ChatMessage
            err := conn.ReadJSON(&msg)
            if err != nil {
                // If the client closed normally (EOF, close frame, going away), log at Info
                if websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
                    // truly unexpected
                    slog.Warn("WebSocket read error", "userID", u.ID, "error", err)
                } else {
                    // normal shutdown
                    slog.Info("WebSocket connection closed", "userID", u.ID, "reason", err)
                }
                return
            }
            now := time.Now().UTC()
            msg.Timestamp = &now
            mu.RLock()
            conv := conversations[msg.ConversationId]
            mu.RUnlock()
            if conv == nil {
                slog.Warn("Conversation not found", "conversationID", msg.ConversationId)
                continue
            }
            for _, p := range conv.Participants {
                if p.conn != nil {
                    _ = p.conn.WriteJSON(msg)
                }
            }
        }
    }()
}
