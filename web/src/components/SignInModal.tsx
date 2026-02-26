import { useState, useEffect } from 'react'
import { KeyRound } from 'lucide-react'
import { authApi } from '#/api'
import type { AuthProvider } from '#/api'

export function SignInModal({
  onLogin,
  onClose,
}: {
  onLogin: (token: string) => void
  onClose: () => void
}) {
  const [tab, setTab] = useState<'standard' | 'ldap'>('standard')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [tokenValue, setTokenValue] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [providers, setProviders] = useState<AuthProvider[]>([])
  const [showTokenInput, setShowTokenInput] = useState(false)

  useEffect(() => {
    authApi
      .getProviders()
      .then((r) => setProviders(r.providers))
      .catch(() => {})
  }, [])

  const hasLDAP = providers.some((p) => p.type === 'ldap')

  const handleLogin = async () => {
    setLoading(true)
    setError('')
    try {
      const loginFn = tab === 'ldap' ? authApi.ldapLogin : authApi.login
      const result = await loginFn(username, password)
      onLogin(result.token)
    } catch (e: any) {
      setError(e.message || 'Invalid username or password')
    } finally {
      setLoading(false)
    }
  }

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setError('')
  }

  return (
    <dialog open className="modal modal-open">
      <div className="modal-box max-w-sm">
        <div className="text-center mb-5">
          <img src="/hermit-crab.svg" alt="hermit" className="h-12 w-12 mx-auto mb-3" />
          <h3 className="font-bold text-xl">Sign in to Hermit</h3>
        </div>

        {!showTokenInput && (
          <>
            {hasLDAP && (
              <div role="tablist" className="tabs tabs-bordered mb-4">
                <button
                  role="tab"
                  className={`tab ${tab === 'standard' ? 'tab-active' : ''}`}
                  onClick={() => { setTab('standard'); resetForm() }}
                >
                  Standard
                </button>
                <button
                  role="tab"
                  className={`tab ${tab === 'ldap' ? 'tab-active' : ''}`}
                  onClick={() => { setTab('ldap'); resetForm() }}
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
                handleLogin()
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
                if (e.key === 'Enter' && tokenValue.trim()) onLogin(tokenValue)
              }}
              placeholder="Paste API token…"
              className="input input-bordered w-full font-mono"
              autoFocus
            />
            <button
              className="btn btn-primary w-full"
              onClick={() => { if (tokenValue.trim()) onLogin(tokenValue) }}
              disabled={!tokenValue.trim()}
            >
              <KeyRound className="h-4 w-4" />
              Authenticate
            </button>
          </div>
        )}

        <div className="mt-5 text-center">
          <button
            className="link link-hover text-xs text-base-content/40"
            onClick={() => {
              setShowTokenInput(!showTokenInput)
              setError('')
            }}
          >
            {showTokenInput ? 'Back to sign in' : 'Use API token instead'}
          </button>
        </div>

        <div className="modal-action justify-center mt-1">
          <button className="btn btn-ghost btn-sm" onClick={onClose}>
            Cancel
          </button>
        </div>
      </div>
      <div className="modal-backdrop" onClick={onClose} />
    </dialog>
  )
}
