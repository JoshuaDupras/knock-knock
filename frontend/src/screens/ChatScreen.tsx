// src/screens/ChatScreen.tsx
import React, { useEffect, useRef, useState } from 'react'
import {
  View, Text, TextInput, Button,
  FlatList, StyleSheet, ActivityIndicator,
} from 'react-native'
import { NativeStackScreenProps } from '@react-navigation/native-stack'
import { RootStackParamList } from '../navigation'
import { ChatMessage } from '../api/index'
import { useAuth } from '../auth/AuthContext'

type Props = NativeStackScreenProps<RootStackParamList, 'Chat'>

export default function ChatScreen({ route }: Props) {
  const { wsUrl }         = route.params
  const [msgs, setMsgs]   = useState<ChatMessage[]>([])
  const [paired, setPaired]           = useState(false)
  const [expiresAt, setExpiresAt]     = useState<Date | null>(null)
  const [timeLeft, setTimeLeft]       = useState<number>(0)
  const [waitingText, setWaitingText] = useState('Waiting to be paired…')
  const [convId, setConvId]           = useState<string | null>(null)
  const [showPairNotif, setShowPairNotif] = useState(false)    // ← new
  const wsRef  = useRef<WebSocket | null>(null)
  const input  = useRef<TextInput>(null)
  const { api } = useAuth()

  // ---------------- countdown ----------------
  useEffect(() => {
    if (!expiresAt) return
    console.log('[Countdown] Timer started. Expires at:', expiresAt)
    const id = setInterval(() => {
      const diff = Math.floor((expiresAt.getTime() - Date.now()) / 1000)
      setTimeLeft(Math.max(diff, 0))
      // console.log('[Countdown] Time left:', Math.max(diff, 0))
    }, 1000)
    return () => {
      clearInterval(id)
      console.log('[Countdown] Timer cleared.')
    }
  }, [expiresAt])

  // ---------------- websocket ----------------
  useEffect(() => {
    console.log('[WebSocket] Connecting to:', wsUrl)
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[WebSocket] Connection opened.')
      setPaired(false)
      setWaitingText('Waiting to be paired…')
    }

    ws.onmessage = ev => {
      console.log('[WebSocket] Message received:', ev.data)
      const msg = JSON.parse(ev.data) as ChatMessage

      switch (msg.type) {
        case 'paired':
          if (!(msg.expiresAt)) {
            console.error('[WebSocket] Paired event without expiresAt:', msg)
            return
          }

          console.log('[WebSocket] Paired event:', msg)
          setPaired(true)
          setMsgs([])
          setExpiresAt(new Date(msg.expiresAt))
          setConvId(msg.conversationId)

          // ← show notification for 2s
          setShowPairNotif(true)
          setTimeout(() => setShowPairNotif(false), 2000)
          break

        case 'time_up':
          console.log('[WebSocket] Time up event:', msg)
          setPaired(false)
          setExpiresAt(null)
          setWaitingText('Round ended – re-queueing…')
          break

        case 'chat':
          console.log('[WebSocket] Chat message:', msg)
          setMsgs(m => [...m, msg])
          break

        default:
          console.log('[WebSocket] Unknown message type:', msg)
      }
    }

    ws.onerror = (err) => {
      console.log('[WebSocket] Error:', err)
    }

    ws.onclose = (ev) => {
      console.log('[WebSocket] Connection closed:', ev)
      if (!paired) {
        setWaitingText('Disconnected… retrying')
        setTimeout(() => {
          setWaitingText('Reconnecting…')
          setPaired(false)
        }, 2000)
      }
    }

    return () => {
      console.log('[WebSocket] Cleaning up, closing connection.')
      ws.close()
    }
  }, [wsUrl])

  // ---------------- helpers ----------------
  const send = (text: string) => {
    if (!paired || !text.trim() || !convId) {
      console.log('[Send] skipping send: not paired, no text, or no convId.')
      return
    }
    const payload = {
      type: 'chat' as const,
      conversationId: convId,
      message: text.trim(),
      timestamp: new Date().toISOString(),
    }
    console.log('[Send] Sending message:', payload)
    wsRef.current?.send(JSON.stringify(payload))
  }

  const skip = () => {
    console.log('[Skip] Skip button pressed.')
    api.sessionSkipPost().catch((err)=>{
      console.log('[Skip] Error skipping:', err)
    })
  }

  // ---------------- render ------------------
  return (
    <View style={styles.container}>

      {/* ← Paired notification banner */}
      {showPairNotif && (
        <View style={styles.pairNotif}>
          <Text style={styles.pairNotifText}>Paired! 🎉</Text>
        </View>
      )}

      {!paired ? (
        <View style={styles.waiting}>
          <ActivityIndicator size="large" />
          <Text style={styles.waitingText}>{waitingText}</Text>
          <Button title="Skip to front" onPress={skip}/>
        </View>
      ) : (
        <>
          <Text style={styles.timer}>⌛ {timeLeft}s left</Text>
          <FlatList
            data={msgs}
            keyExtractor={(_, i) => i.toString()}
            renderItem={({ item }) => (
              <View style={styles.msg}>
                <Text>{item.message}</Text>
                <Text style={styles.ts}>
                  {new Date(item.timestamp!).toLocaleTimeString()}
                </Text>
              </View>
            )}
            style={styles.list}
          />
          <TextInput
            ref={input}
            onSubmitEditing={e => {
              send(e.nativeEvent.text)
              input.current?.clear()
            }}
            style={styles.input}
            placeholder="Type your message…"
          />
          <Button title="Skip Now" onPress={skip}/>
        </>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  container:{flex:1,padding:16},
  waiting:{flex:1,alignItems:'center',justifyContent:'center'},
  waitingText:{marginTop:16,fontSize:16,color:'#555'},
  timer:{alignSelf:'center',marginBottom:4,fontWeight:'bold'},
  list:{flex:1,marginBottom:8},
  msg:{marginVertical:4},
  ts:{fontSize:10,color:'#666'},
  input:{borderWidth:1,padding:8,borderRadius:4,marginBottom:8},

  // ← new styles for the pairing notification
  pairNotif: {
    position: 'absolute',
    top: 16,
    alignSelf: 'center',
    backgroundColor: '#4caf50',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 4,
    zIndex: 1,
  },
  pairNotifText: {
    color: 'white',
    fontWeight: 'bold',
  },
})
