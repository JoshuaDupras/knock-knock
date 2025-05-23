// App.tsx
import React from 'react'
import Navigator from './src/navigation'
import { AuthProvider } from './src/auth/AuthContext'

export default function App() {
  return (
    <AuthProvider>
      <Navigator />
    </AuthProvider>
  )
}
