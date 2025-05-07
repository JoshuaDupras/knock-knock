"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { AppBar, Box, Button, Container, IconButton, List, ListItem, Paper, TextField, Toolbar, Typography } from "@mui/material";
import ExitToAppIcon from "@mui/icons-material/ExitToApp";
import SendIcon from "@mui/icons-material/Send";

const API_URL = "http://172.22.223.245:8080";
const WS_URL = "ws://172.22.223.245:8080/ws";

const Chat: React.FC = () => {
  const [messages, setMessages] = useState<{ from: string; message: string }[]>([]);
  const [input, setInput] = useState<string>("");
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [username, setUsername] = useState<string | null>(null);
  const [allUsers, setAllUsers] = useState<string[]>([]);
  const [users, setUsers] = useState<string[]>([]);
  const [userCount, setUserCount] = useState<number>(0);
  const [groups, setGroups] = useState({});
  const [selectedUser, setSelectedUser] = useState<string | null>(null);
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      alert("Not authenticated! Redirecting to login...");
      router.push("/login");
      return;
    }

    fetch(`${API_URL}/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        setUsername(data.username);
      })
      .catch(() => {
        alert("Session expired. Please log in again.");
        localStorage.removeItem("token");
        router.push("/login");
      });

    const socket = new WebSocket(`${WS_URL}?token=${token}`);
    socket.onopen = () => console.log("WebSocket connected");

    socket.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === "userList") {
        setAllUsers(data.users);
      } else {
        setMessages((prev) => [...prev, data]);
      }
    };

    socket.onclose = () => console.log("WebSocket disconnected");
    setWs(socket);

    return () => {
      socket.close();
    };
  }, [router]);

  useEffect(() => {
    if (username) {
      const filteredUsers = allUsers.filter((user) => user !== username);
      setUsers(filteredUsers);
      setUserCount(allUsers.length);
    }
  }, [username, allUsers]);

  const sendMessage = () => {
    if (ws && input.trim() && selectedUser) {
      const messageData = JSON.stringify({ to: selectedUser, message: input });
      ws.send(messageData);
      setMessages((prev) => [...prev, { from: "You", message: input }]);
      setInput("");
    }
  };

  const handleLogout = () => {
    const token = localStorage.getItem("token");
    if (!token) return;

    fetch(`${API_URL}/logout`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    }).then(() => {
      localStorage.removeItem("token");
      router.push("/login");
    });
  };

  return (
    <Box sx={{ bgcolor: "#121212", color: "white", height: "100vh", display: "flex", flexDirection: "column", alignItems: "center" }}>
      {/* Header Bar */}
      <AppBar position="static" sx={{ bgcolor: "#1e1e1e" }}>
        <Toolbar sx={{ display: "flex", justifyContent: "space-between" }}>
          <Typography variant="h6">Logged in as: {username}</Typography>
          <Typography variant="h6">Online: {userCount}</Typography>
          <IconButton color="inherit" onClick={handleLogout}>
            <ExitToAppIcon />
          </IconButton>
        </Toolbar>
      </AppBar>

      {/* Content */}
      <Container maxWidth="sm" sx={{ flexGrow: 1, mt: 2 }}>
        {/* Active Users List */}
        <Paper sx={{ bgcolor: "#1e1e1e", p: 2, mb: 2, borderRadius: 2 }}>
          <Typography variant="h6">Other Users ({userCount-1})</Typography>
          <List>
            {users.map((user, index) => (
              <ListItem
                key={index}
                sx={{
                  bgcolor: selectedUser === user ? "#2196F3" : "transparent",
                  color: selectedUser === user ? "white" : "gray",
                  cursor: "pointer",
                  "&:hover": { bgcolor: "#333" },
                  p: 1,
                  borderRadius: 1,
                }}
                onClick={() => setSelectedUser(user)}
              >
                {user}
              </ListItem>
            ))}
          </List>
        </Paper>

        {/* Chat Messages */}
        <Paper sx={{ bgcolor: "#1e1e1e", p: 2, mb: 2, height: "300px", overflowY: "auto", borderRadius: 2 }}>
          <Typography variant="h6">Chat</Typography>
          {messages.map((msg, index) => (
            <Box
              key={index}
              sx={{
                bgcolor: msg.from === "You" ? "#4CAF50" : "#333",
                color: "white",
                p: 1,
                m: 1,
                borderRadius: 2,
                alignSelf: msg.from === "You" ? "flex-end" : "flex-start",
                textAlign: msg.from === "You" ? "right" : "left",
              }}
            >
              <Typography variant="body2" sx={{ fontWeight: "bold" }}>{msg.from}:</Typography>
              <Typography variant="body1">{msg.message}</Typography>
            </Box>
          ))}
        </Paper>

        {/* Chat Input */}
        <Box sx={{ display: "flex", alignItems: "center" }}>
          <TextField
            fullWidth
            variant="outlined"
            sx={{
              bgcolor: "white",
              borderRadius: "4px",
              input: { color: "black" },
            }}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && sendMessage()}
          />
          <Button
            variant="contained"
            sx={{ bgcolor: "#2196F3", color: "white", ml: 1, p: 2 }}
            onClick={sendMessage}
          >
            <SendIcon />
          </Button>
        </Box>
      </Container>
    </Box>
  );
};

export default Chat;
