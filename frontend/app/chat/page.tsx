"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";

const API_URL = "http://172.22.223.245:8080";
const WS_URL = "ws://172.22.223.245:8080/ws";

const Chat: React.FC = () => {
  const [messages, setMessages] = useState<{ from: string; message: string }[]>([]);
  const [input, setInput] = useState<string>("");
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [username, setUsername] = useState<string>("");
  const [users, setUsers] = useState<string[]>([]);
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
      .then((data) => setUsername(data.username))
      .catch(() => {
        alert("Session expired. Please log in again.");
        localStorage.removeItem("token");
        router.push("/login");
      });

    const fetchUsers = () => {
      fetch(`${API_URL}/users`, {
        headers: { Authorization: `Bearer ${token}` },
      })
        .then((res) => res.json())
        .then((data) => setUsers(data.users.map((user: any) => user.username)));
    };

    fetchUsers();
    const interval = setInterval(fetchUsers, 5000);

    const socket = new WebSocket(`${WS_URL}?token=${token}`);
    socket.onopen = () => console.log("WebSocket connected");
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setMessages((prev) => [...prev, data]);
    };
    socket.onclose = () => console.log("WebSocket disconnected");
    setWs(socket);

    return () => {
      socket.close();
      clearInterval(interval);
    };
  }, [router]);

  const sendMessage = () => {
    if (ws && input.trim() && selectedUser) {
      const messageData = JSON.stringify({ to: selectedUser, message: input });
      ws.send(messageData);
      setMessages((prev) => [...prev, { from: "You", message: input }]);
      setInput("");
    } else {
      console.log("WebSocket not connected or no recipient selected");
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
    <div className="flex flex-col items-center w-full h-screen p-4">
      <div className="w-full max-w-lg flex justify-between items-center mb-4">
        <span className="text-lg font-bold">Logged in as: {username}</span>
        <button className="p-2 bg-red-500 text-white rounded-lg" onClick={handleLogout}>
          Log Out
        </button>
      </div>
      <div className="w-full max-w-lg border p-4 rounded-lg shadow-md h-96 overflow-y-auto">
        <h3 className="text-lg font-bold mb-2">Active Users</h3>
        <ul className="mb-4">
          {users.map((user, index) => (
            <li
              key={index}
              className={`p-1 cursor-pointer ${selectedUser === user ? "bg-blue-300" : "hover:bg-gray-200"}`}
              onClick={() => setSelectedUser(user)}
            >
              {user}
            </li>
          ))}
        </ul>
        <h3 className="text-lg font-bold mb-2">Chat</h3>
        {messages.map((msg, index) => (
          <div key={index} className="p-2 border-b text-black">
            <strong>{msg.from}: </strong> {msg.message}
          </div>
        ))}
      </div>
      <div className="flex mt-4 w-full max-w-lg">
        <input
          type="text"
          className="flex-1 p-2 border rounded-l-lg text-black bg-white"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={(e) => e.key === "Enter" && sendMessage()}
        />
        <button className="p-2 bg-blue-500 text-white rounded-r-lg" onClick={sendMessage}>
          Send
        </button>
      </div>
    </div>
  );
};

export default Chat;
