import type { ReactNode } from 'react'
import { Navigate } from 'react-router'

import useAuth from '@/hooks/use-auth'

export default function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div role="status" className="flex min-h-screen items-center justify-center bg-mandat-bg text-mandat-muted">
        Перевірка авторизації…
      </div>
    )
  }

  return user ? children : <Navigate to="/admin/login" replace />
}
