import { createContext } from 'react'

import type { AuthUser } from '@/types'

export interface AuthContextValue {
  user: AuthUser | null
  loading: boolean
  login: (login: string, password: string) => Promise<void>
  logout: () => Promise<void>
  clearUser: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)
