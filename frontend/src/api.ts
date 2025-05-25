// src/api.ts
const BASE = process.env.REACT_APP_API_BASE ?? '/api';

export interface AnonymousSessionResponse {
  token: string
  websocketUrl: string
  expiresInSeconds: number
  conversationId: string
}

export interface AuthResponse { token: string }


export async function startAnonymous(displayName: string) {
  const resp = await fetch(`${BASE}/session/anonymous`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ displayName }),
  })
  if (!resp.ok) throw await resp.json()
  return resp.json() as Promise<AnonymousSessionResponse>
}

export async function skipSession(token: string) {
  const resp = await fetch(`${BASE}/session/skip`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  })
  if (resp.status === 429) throw await resp.json()
  if (!resp.ok) throw new Error(resp.statusText)
}

export async function registerUser(token: string, username: string, password: string) {
  const resp = await fetch(`${BASE}/account/register`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ username, password }),
  })
  if (resp.status === 409) throw await resp.json()
  if (!resp.ok) throw new Error(resp.statusText)
  return resp.json() as Promise<AuthResponse>
}

export async function loginUser(username: string, password: string) {
  const resp = await fetch(`${BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (resp.status === 401) throw await resp.json()
  if (!resp.ok) throw new Error(resp.statusText)
  return resp.json() as Promise<AuthResponse>
}
