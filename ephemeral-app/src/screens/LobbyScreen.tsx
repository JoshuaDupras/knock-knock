// src/screens/LobbyScreen.tsx
import React, { useEffect } from 'react'
import { View, ActivityIndicator, StyleSheet } from 'react-native'
import { NativeStackNavigationProp } from '@react-navigation/native-stack'
import { RootStackParamList } from '../navigation'
import { useAuth } from '../auth/AuthContext'

type NavProp = NativeStackNavigationProp<RootStackParamList, 'Lobby'>

export default function LobbyScreen({ navigation }: { navigation: NavProp }) {
  const { api, setToken } = useAuth()

  useEffect(() => {
    ;(async () => {
      // 1) hit anonymous-session
      const resp = await api.sessionAnonymousPost()
      // 2) stash the JWT
      setToken(resp.token)
      // 3) go to Chat
      navigation.replace('Chat', {
        token: resp.token,
        wsUrl: resp.websocketUrl,
        ttl:   resp.expiresInSeconds,
      })
    })().catch(err => {
      console.error('failed to connect anonymously', err)
    })
  }, [api, navigation, setToken])

  return (
    <View style={styles.container}>
      <ActivityIndicator size="large" />
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', alignItems: 'center' },
})
