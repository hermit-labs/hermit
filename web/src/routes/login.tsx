import { useEffect, useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import { KeyRound } from 'lucide-react'
import { authApi, getToken, setToken } from '#/api'
import type { AuthProvider } from '#/api'

type LoginSearch = {
  redirect?: string
}

function normalizeRedirect(rawRedirect?: string): string {
  if (!rawRedirect || !rawRedirect.startsWith('/')) return '/'
  if (rawRedirect.startsWith('//')) return '/'
  if (rawRedirect.startsWith('/login')) return '/'
  return rawRedirect
}

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>): LoginSearch => ({
    redirect:
      typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: LoginPage,
})

function LoginPage() {
  const { redirect } = Route.useSearch()
  const redirectTarget = normalizeRedirect(redirect)
  const [tab, setTab] = useState<'standard' | 'ldap'>('standard')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [tokenValue, setTokenValue] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [providers, setProviders] = useState<AuthProvider[]>([])
  const [showTokenInput, setShowTokenInput] = useState(false)

  useEffect(() => {
    if (getToken()) {
      window.location.replace(redirectTarget)
      return
    }

    authApi
      .getProviders()
      .then((r) => setProviders(r.providers))
      .catch(() => {})
  }, [redirectTarget])

  const hasLDAP = providers.some((p) => p.type === 'ldap')

  const handlePasswordLogin = async () => {
    setLoading(true)
    setError('')
    try {
      const loginFn = tab === 'ldap' ? authApi.ldapLogin : authApi.login
      const result = await loginFn(username, password)
      setToken(result.token)
      window.location.replace(redirectTarget)
    } catch (e: any) {
      setError(e.message || 'Invalid username or password')
    } finally {
      setLoading(false)
    }
  }

  const handleTokenLogin = () => {
    const trimmed = tokenValue.trim()
    if (!trimmed) return
    setToken(trimmed)
    window.location.replace(redirectTarget)
  }

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setError('')
  }

  return (
    <div className="min-h-[70vh] flex items-center justify-center px-4">
      <div className="card w-full max-w-sm bg-base-200 shadow-xl border border-base-300">
        <div className="card-body">
          <div className="text-center mb-3">
            <img src="/hermit-crab.svg" alt="hermit" className="h-12 w-12 mx-auto mb-3" />
            <h1 className="font-bold text-xl">Sign in to Hermit</h1>
          </div>

          {!showTokenInput && (
            <>
              {hasLDAP && (
                <div role="tablist" className="tabs tabs-bordered mb-3">
                  <button
                    role="tab"
                    className={`tab ${tab === 'standard' ? 'tab-active' : ''}`}
                    onClick={() => {
                      setTab('standard')
                      resetForm()
                    }}
                  >
                    Standard
                  </button>
                  <button
                    role="tab"
                    className={`tab ${tab === 'ldap' ? 'tab-active' : ''}`}
                    onClick={() => {
                      setTab('ldap')
                      resetForm()
                    }}
                  >
                    LDAP
                  </button>
                </div>
              )}

              {error && (
                <div className="alert alert-error mb-3 text-sm py-2">{error}</div>
              )}

              <form
                className="flex flex-col gap-3"
                onSubmit={(e) => {
                  e.preventDefault()
                  handlePasswordLogin()
                }}
              >
                <input
                  type="text"
                  placeholder={tab === 'ldap' ? 'LDAP Username' : 'Username'}
                  className="input input-bordered w-full"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoFocus
                  autoComplete="username"
                />
                <input
                  type="password"
                  placeholder="Password"
                  className="input input-bordered w-full"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                />
                <button
                  type="submit"
                  className="btn btn-primary w-full"
                  disabled={loading || !username.trim() || !password.trim()}
                >
                  {loading && <span className="loading loading-spinner loading-sm" />}
                  Sign in
                </button>
              </form>
            </>
          )}

          {showTokenInput && (
            <div className="flex flex-col gap-3">
              {error && (
                <div className="alert alert-error mb-1 text-sm py-2">{error}</div>
              )}
              <input
                type="password"
                value={tokenValue}
                onChange={(e) => setTokenValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && tokenValue.trim()) handleTokenLogin()
                }}
                placeholder="Paste API token..."
                className="input input-bordered w-full font-mono"
                autoFocus
              />
              <button
                className="btn btn-primary w-full"
                onClick={handleTokenLogin}
                disabled={!tokenValue.trim()}
              >
                <KeyRound className="h-4 w-4" />
                Authenticate
              </button>
            </div>
          )}

          <div className="mt-4 text-center">
            <button
              className="link link-hover text-xs text-base-content/50"
              onClick={() => {
                setShowTokenInput(!showTokenInput)
                setError('')
              }}
            >
              {showTokenInput ? 'Back to sign in' : 'Use API token instead'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
