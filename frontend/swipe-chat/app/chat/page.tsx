"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";

const WS_URL = "ws://localhost:8080/ws";

const Chat: React.FC = () => {
  const [messages, setMessages] = useState<string[]>([]);
  const [input, setInput] = useState<string>("");
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [username, setUsername] = useState<string>("");
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      alert("Not authenticated! Redirecting to login...");
      window.location.href = "/login";
      return;
    }

    // Fetch the logged-in user's info
    fetch("http://localhost:8080/me", {
      headers: { Authorization: token },
    })
      .then((res) => res.json())
      .then((data) => setUsername(data.username))
      .catch(() => {
        alert("Session expired. Please log in again.");
        localStorage.removeItem("token");
        window.location.href = "/login";
      });

    const socket = new WebSocket(`${WS_URL}?token=${token}`);
    socket.onopen = () => socket.send(token);

    socket.onmessage = (event) => {
      setMessages((prev) => [...prev, event.data]);
    };

    socket.onclose = () => console.log("WebSocket disconnected");

    setWs(socket);

    return () => socket.close();
  }, []);

  const sendMessage = () => {
    if (ws && input.trim()) {
      ws.send(input);
      setMessages((prev) => [...prev, `You: ${input}`]);
      setInput("");
    }
  };

  const handleLogout = () => {
    localStorage.removeItem("token");
    router.push("/login");
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
        {messages.map((msg, index) => (
          <div key={index} className="p-2 border-b text-black">{msg}</div>
        ))}
      </div>
      <div className="flex mt-4 w-full max-w-lg">
        <input
          type="text"
          className="flex-1 p-2 border rounded-l-lg text-black bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={(e) => e.key === "Enter" && sendMessage()}
        />
        <button className="p-2 bg-blue-500 text-white rounded-r-lg hover:bg-blue-600" onClick={sendMessage}>
          Send
        </button>
      </div>
    </div>
  );
};

export default Chat;
