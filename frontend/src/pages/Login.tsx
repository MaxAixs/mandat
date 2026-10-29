import { useState, type FormEvent } from 'react'
import { LoaderCircle, LockKeyhole } from 'lucide-react'
import { Link, Navigate, useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import useAuth from '@/hooks/use-auth'

export default function Login() {
  const { user, loading, login } = useAuth()
  const navigate = useNavigate()
  const [loginValue, setLoginValue] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (loading) {
    return (
      <div role="status" className="flex min-h-screen items-center justify-center bg-mandat-bg text-mandat-muted">
        Перевірка авторизації…
      </div>
    )
  }

  if (user) return <Navigate to="/admin" replace />

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setBusy(true)
    setError(null)

    try {
      await login(loginValue, password)
      navigate('/admin', { replace: true })
    } catch {
      setError('Невірний логін або пароль. Спробуйте ще раз.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-mandat-bg px-4 py-10 text-mandat-text">
      <div className="w-full max-w-md border border-mandat-line bg-mandat-surface p-6 shadow-2xl shadow-black/30 md:p-8">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex size-12 items-center justify-center bg-mandat-red oswald text-2xl font-bold text-white">
            М
          </div>
          <div className="text-[11px] font-semibold tracking-[0.22em] text-mandat-red-bright">
            МАНДАТ / АДМІНПАНЕЛЬ
          </div>
          <h1 className="mt-2 text-3xl font-medium tracking-wide">Вхід для редакції</h1>
          <p className="mt-2 text-sm text-mandat-muted">
            Увійдіть за допомогою свого логіна та пароля.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="admin-login" className="text-xs tracking-wide text-mandat-muted">ЛОГІН</Label>
            <Input
              id="admin-login"
              name="login"
              autoComplete="username"
              required
              autoFocus
              disabled={busy}
              value={loginValue}
              onChange={(event) => setLoginValue(event.target.value)}
              className="h-11 rounded-none border-mandat-line bg-mandat-bg text-mandat-text focus-visible:ring-mandat-red-bright/30"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="admin-password" className="text-xs tracking-wide text-mandat-muted">ПАРОЛЬ</Label>
            <Input
              id="admin-password"
              name="password"
              type="password"
              autoComplete="current-password"
              required
              disabled={busy}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="h-11 rounded-none border-mandat-line bg-mandat-bg text-mandat-text focus-visible:ring-mandat-red-bright/30"
            />
          </div>

          {error && <p role="alert" className="bg-mandat-red/10 p-3 text-sm text-mandat-red-bright">{error}</p>}

          <Button
            type="submit"
            disabled={busy}
            className="h-11 w-full rounded-none bg-mandat-red text-white hover:bg-mandat-red-bright"
          >
            {busy ? <LoaderCircle className="size-4 animate-spin" /> : <LockKeyhole className="size-4" />}
            {busy ? 'Вхід…' : 'Увійти'}
          </Button>
        </form>

        <div className="mt-6 text-center">
          <Link to="/" className="text-xs text-mandat-muted transition-colors hover:text-mandat-text">
            ← Повернутися на сайт
          </Link>
        </div>
      </div>
    </main>
  )
}
