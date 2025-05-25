// src/api/index.ts  (hand-written façade)

// ➊ Imports from generated code
import { Configuration }   from '../api-client/runtime'
import { DefaultApi }      from '../api-client/apis'
export type { ChatMessage } from '../api-client/models/ChatMessage';
export *                   from '../api-client/models'   // re-export types
export { DefaultApi, Configuration }

// ➋ Auth token plumbing
let authToken: string | null = null
export const setAuthToken = (t: string|null) => { authToken = t }

// ➌ Singleton client
const cfg = new Configuration({
  basePath: process.env.REACT_APP_API_BASE ?? '/api',
  accessToken: async () => authToken ?? '',
})
export const api = new DefaultApi(cfg)

// helper
export const wsUrlFromAnonymous = (url: string) =>
  url.replace(/^http/, 'ws')
