// // src/screens/RegisterScreen.tsx
// import React, { useState } from 'react'
// import { View, TextInput, Button, Text, StyleSheet } from 'react-native'
// import { NativeStackScreenProps } from '@react-navigation/native-stack'
// import { RootStackParamList } from '../navigation'
// import { registerUser } from '../api'
// import { useAuth } from '../auth/AuthContext'

// type Props = NativeStackScreenProps<RootStackParamList, 'Register'>

// export default function RegisterScreen({ route, navigation }: Props) {
//   const { token } = route.params
//   const [username, setUser] = useState('')
//   const [password, setPass] = useState('')
//   const [err, setErr] = useState('')

//   const { api, token: anonTok, setToken } = useAuth()

//   const submit = async () => {
//     try {
//        const res = await api.accountRegisterPost({
//          registerRequest: { username, password }
//        })
//        setToken(res.token)             // replace anon token with long-lived one
//        navigation.goBack()             // back to Chat
//     } catch (e: any) {
//        setErr(e.error === 'username_exists' ? 'Username taken' : e.message)
//     }
//   }

//   return (
//     <View style={styles.container}>
//       <TextInput
//         placeholder="Username"
//         value={username}
//         onChangeText={setUser}
//         style={styles.input}
//       />
//       <TextInput
//         placeholder="Password"
//         secureTextEntry
//         value={password}
//         onChangeText={setPass}
//         style={styles.input}
//       />
//       {!!err && <Text style={styles.error}>{err}</Text>}
//       <Button title="Keep Chatting" onPress={submit} />
//     </View>
//   )
// }

// const styles = StyleSheet.create({
//   container: { flex:1, justifyContent:'center', padding:16 },
//   input: { borderWidth:1, padding:8, marginBottom:8 },
//   error: { color:'red', marginBottom:8 },
// })
