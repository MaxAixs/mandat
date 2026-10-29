import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import { getCurrentUser, login as loginRequest, logout as logoutRequest } from '@/api/auth'
import { AuthContext } from '@/context/auth-context'
import type { AuthUser } from '@/types'

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)

  const clearUser = useCallback(() => setUser(null), [])

  useEffect(() => {
    let active = true

    getCurrentUser()
      .then((currentUser) => {
        if (active) setUser(currentUser)
      })
      .catch(() => {
        if (active) setUser(null)
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    const handleUnauthorized = () => clearUser()
    window.addEventListener('mandat:unauthorized', handleUnauthorized)

    return () => {
      active = false
      window.removeEventListener('mandat:unauthorized', handleUnauthorized)
    }
  }, [clearUser])

  const login = useCallback(async (loginValue: string, password: string) => {
    const authenticatedUser = await loginRequest(loginValue, password)
    setUser(authenticatedUser)
  }, [])

  const logout = useCallback(async () => {
    try {
      await logoutRequest()
    } finally {
      clearUser()
    }
  }, [clearUser])

  const value = useMemo(
    () => ({ user, loading, login, logout, clearUser }),
    [user, loading, login, logout, clearUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
