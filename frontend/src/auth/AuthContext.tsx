// src/auth/AuthContext.tsx
import React, { createContext, useContext, useState } from 'react'
import { DefaultApi, Configuration } from '../api/index' 

type AuthCtx = {
  token: string | null
  setToken: (t: string|null) => void
  api: DefaultApi
}

const Ctx = createContext<AuthCtx>(null as any)

export const AuthProvider: React.FC<{children: React.ReactNode}> = ({ children }) => {
  const [token, setToken] = useState<string|null>(null)

  // the generator expects a function that returns the token
  const cfg = new Configuration({
    basePath: process.env.REACT_APP_API_BASE ?? '/api',
    accessToken: async () => token ?? '',
  })

  const value: AuthCtx = {
    token,
    setToken,
    api: new DefaultApi(cfg),
  }

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export const useAuth = () => useContext(Ctx)
