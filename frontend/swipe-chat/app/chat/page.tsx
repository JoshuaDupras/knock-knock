"use client"; // Required for Next.js App Router

import { useState, useEffect } from "react";

const Chat: React.FC = () => {
  const [messages, setMessages] = useState<string[]>([]);
  const [input, setInput] = useState<string>("");
  const [ws, setWs] = useState<WebSocket | null>(null);

  useEffect(() => {
    const socket = new WebSocket("ws://localhost:8080/ws"); // Replace with Go server URL

    socket.onopen = () => console.log("Connected to WebSocket server");
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

  return (
    <div className="flex flex-col items-center w-full h-screen p-4">
      <div className="w-full max-w-lg border p-4 rounded-lg shadow-md h-96 overflow-y-auto">
        {messages.map((msg, index) => (
          <div key={index} className="p-2 border-b">{msg}</div>
        ))}
      </div>
      <div className="flex mt-4 w-full max-w-lg">
        <input
          type="text"
          className="flex-1 p-2 border rounded-l-lg"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={(e) => e.key === "Enter" && sendMessage()}
        />
        <button
          className="p-2 bg-blue-500 text-white rounded-r-lg"
          onClick={sendMessage}
        >
          Send
        </button>
      </div>
    </div>
  );
};

export default Chat;
