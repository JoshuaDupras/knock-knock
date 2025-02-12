"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

const Login = () => {
  const [username, setUsername] = useState("");
  const [plaintextPassword, setPassword] = useState("");
  const [error, setError] = useState("");
  const router = useRouter();

  const handleLogin = async () => {
    setError("");
    const response = await fetch("http://172.22.223.245:8080/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, plaintextPassword }),
    });

    if (!response.ok) {
      setError("Invalid credentials");
      return;
    }

    const data = await response.json();
    localStorage.setItem("token", data.token);
    router.push("/chat");
  };

  return (
    <div className="flex flex-col items-center justify-center h-screen p-4">
      <h2 className="text-2xl font-bold mb-4">Login</h2>
      <input
        type="text"
        placeholder="Username"
        className="p-2 border rounded-lg mb-2 w-64 text-black bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
        value={username}
        onChange={(e) => setUsername(e.target.value)}
      />
      <input
        type="password"
        placeholder="Password"
        className="p-2 border rounded-lg mb-2 w-64 text-black bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
        value={plaintextPassword}
        onChange={(e) => setPassword(e.target.value)}
      />
      <button
        className="p-2 bg-blue-500 text-white rounded-lg w-64 hover:bg-blue-600"
        onClick={handleLogin}
      >
        Login
      </button>
      {error && <p className="text-red-500 mt-2">{error}</p>}
    </div>
  );
};

export default Login;
