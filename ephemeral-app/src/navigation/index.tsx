// src/navigation/index.tsx
import React from 'react'
import { NavigationContainer } from '@react-navigation/native'
import { createNativeStackNavigator } from '@react-navigation/native-stack'
import LobbyScreen from '../screens/LobbyScreen'
import ChatScreen from '../screens/ChatScreen'
// import RegisterScreen from '../screens/RegisterScreen'

export type RootStackParamList = {
  Lobby: undefined
  Chat: { token: string; wsUrl: string; ttl: number }
//   Register: { token: string }
}

const Stack = createNativeStackNavigator<RootStackParamList>()

export default function Navigator() {
  return (
    <NavigationContainer>
      <Stack.Navigator initialRouteName="Lobby">
        <Stack.Screen name="Lobby" component={LobbyScreen} />
        <Stack.Screen name="Chat" component={ChatScreen} />
        {/* <Stack.Screen name="Register" component={RegisterScreen} /> */}
      </Stack.Navigator>
    </NavigationContainer>
  )
}
