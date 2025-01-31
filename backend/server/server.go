package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ChatServer implements the generated API interface
type ChatServer struct{}

// ChatWebSocket handles WebSocket connections
func ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Println("Attempting WebSocket Upgrade...")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	log.Println("WebSocket client connected")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		fmt.Println("received message:", string(msg))

	    response := string(msg) + " received"

		fmt.Println("responding with message:", response)

		// Echo message back to the client
		err = conn.WriteMessage(websocket.TextMessage, []byte(response))
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}
}

// Ping responds with a JSON "pong"
func GetPing(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"ping": "pong"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StartServer initializes the HTTP + WebSocket server
func StartServer() {
	mux := http.NewServeMux()

	// Register REST API endpoints
	mux.HandleFunc("/ping", GetPing)

	// Register WebSocket manually (don't use api.HandlerFromMux here)
	mux.HandleFunc("/ws", ChatWebSocket)

	// Start the HTTP + WebSocket server
	log.Println("Server running on http://0.0.0.0:8080 (HTTP + WebSockets)")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", mux))
}
