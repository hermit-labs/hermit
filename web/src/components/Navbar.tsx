import { useState } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import {
  Shield,
  LogIn,
  LogOut,
  User,
  Sun,
  Moon,
  Monitor,
  Lock,
  Eye,
  EyeOff,
  AlertCircle,
  CheckCircle,
} from 'lucide-react'
import { useTheme } from '#/hooks/useTheme'
import { api } from '#/api'
import type { APIError } from '#/api'

const themeOptions = [
  { value: 'system' as const, label: 'System', Icon: Monitor },
  { value: 'light' as const, label: 'Light', Icon: Sun },
  { value: 'dark' as const, label: 'Dark', Icon: Moon },
]

export function Navbar({
  loggedIn,
  onSignOut,
}: {
  loggedIn: boolean
  onSignOut: () => void
}) {
  const { mode, setMode } = useTheme()
  const ActiveIcon = themeOptions.find((o) => o.value === mode)!.Icon
  const [showChangePw, setShowChangePw] = useState(false)
  const currentPath = useRouterState({
    select: (state) =>
      `${state.location.pathname}${state.location.searchStr}${state.location.hash}`,
  })

  return (
    <>
      <header className="navbar bg-base-200 border-b border-base-300 sticky top-0 z-40 px-4">
        <div className="navbar-start">
          <Link to="/" className="flex items-center gap-2 font-bold text-lg">
            <img src="/hermit-crab.svg" alt="hermit" className="h-7 w-7" />
            <span>Hermit</span>
          </Link>
        </div>
        <div className="navbar-end gap-2">
          <div className="flex gap-2">
            <div className="dropdown dropdown-end">
              <div tabIndex={0} role="button" className="btn btn-ghost btn-sm btn-circle" aria-label="Theme">
                <ActiveIcon className="h-4 w-4" />
              </div>
              <ul tabIndex={0} className="dropdown-content menu bg-base-200 rounded-box z-50 w-36 p-2 shadow-lg mt-2">
                {themeOptions.map(({ value, label, Icon }) => (
                  <li key={value}>
                    <button
                      onClick={() => { setMode(value); (document.activeElement as HTMLElement).blur() }}
                      className={mode === value ? 'active' : ''}
                    >
                      <Icon className="h-4 w-4" />
                      {label}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            <Link to="/publish" className="btn btn-ghost btn-sm">
              Publish
            </Link>
            {loggedIn && (
              <Link to="/admin" className="btn btn-ghost btn-sm">
                <Shield className="h-4 w-4" />
                Admin
              </Link>
            )}
            {loggedIn ? (
              <div className="dropdown dropdown-end">
                <div tabIndex={0} role="button" className="btn btn-ghost btn-sm btn-circle">
                  <User className="h-5 w-5" />
                </div>
                <ul tabIndex={0} className="dropdown-content menu bg-base-200 rounded-box z-50 w-44 p-2 shadow-lg mt-2">
                  <li>
                    <button onClick={() => { setShowChangePw(true); (document.activeElement as HTMLElement).blur() }}>
                      <Lock className="h-4 w-4" />
                      Change password
                    </button>
                  </li>
                  <li>
                    <button onClick={onSignOut} className="text-error">
                      <LogOut className="h-4 w-4" />
                      Sign out
                    </button>
                  </li>
                </ul>
              </div>
            ) : (
              <Link
                to="/login"
                search={{ redirect: currentPath }}
                className="btn btn-primary btn-sm"
              >
                <LogIn className="h-4 w-4" />
                Sign in
              </Link>
            )}
          </div>
        </div>
      </header>

      {showChangePw && (
        <ChangePasswordModal onClose={() => setShowChangePw(false)} />
      )}
    </>
  )
}

function ChangePasswordModal({ onClose }: { onClose: () => void }) {
  const [form, setForm] = useState({ oldPassword: '', newPassword: '', confirmPassword: '' })
  const [showOld, setShowOld] = useState(false)
  const [showNew, setShowNew] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const mutation = useMutation({
    mutationFn: () => api.changePassword(form.oldPassword, form.newPassword),
    onSuccess: () => {
      setSuccess(true)
      setError(null)
    },
    onError: (err: APIError) => {
      setError(err.message)
    },
  })

  const canSubmit =
    form.oldPassword.length > 0 &&
    form.newPassword.length >= 6 &&
    form.newPassword === form.confirmPassword &&
    !mutation.isPending

  if (success) {
    return (
      <dialog open className="modal modal-open">
        <div className="modal-box max-w-sm text-center">
          <div className="bg-success/10 rounded-2xl p-6 inline-block mb-4">
            <CheckCircle className="h-10 w-10 text-success" />
          </div>
          <h3 className="font-bold text-lg mb-2">Password changed</h3>
          <p className="text-sm text-base-content/60 mb-4">
            Your password has been updated successfully.
          </p>
          <button className="btn btn-primary btn-sm" onClick={onClose}>
            Done
          </button>
        </div>
        <div className="modal-backdrop" onClick={onClose} />
      </dialog>
    )
  }

  return (
    <dialog open className="modal modal-open">
      <div className="modal-box max-w-sm">
        <h3 className="font-bold text-lg mb-4 flex items-center gap-2">
          <Lock className="h-5 w-5" />
          Change Password
        </h3>

        {error && (
          <div className="alert alert-error text-sm py-2 mb-4">
            <AlertCircle className="h-4 w-4" />
            <span>{error}</span>
          </div>
        )}

        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (form.newPassword !== form.confirmPassword) {
              setError('Passwords do not match')
              return
            }
            mutation.mutate()
          }}
          className="space-y-3"
        >
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Current Password</legend>
            <div className="join w-full">
              <input
                type={showOld ? 'text' : 'password'}
                className="input input-bordered input-sm join-item flex-1"
                value={form.oldPassword}
                onChange={(e) => setForm({ ...form, oldPassword: e.target.value })}
                autoFocus
                required
              />
              <button
                type="button"
                className="btn btn-ghost btn-sm join-item"
                onClick={() => setShowOld(!showOld)}
              >
                {showOld ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            </div>
          </fieldset>
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">New Password</legend>
            <div className="join w-full">
              <input
                type={showNew ? 'text' : 'password'}
                placeholder="Min 6 characters"
                className="input input-bordered input-sm join-item flex-1"
                value={form.newPassword}
                onChange={(e) => setForm({ ...form, newPassword: e.target.value })}
                required
                minLength={6}
              />
              <button
                type="button"
                className="btn btn-ghost btn-sm join-item"
                onClick={() => setShowNew(!showNew)}
              >
                {showNew ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            </div>
          </fieldset>
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Confirm New Password</legend>
            <input
              type="password"
              className={`input input-bordered input-sm w-full ${
                form.confirmPassword && form.confirmPassword !== form.newPassword
                  ? 'input-error'
                  : ''
              }`}
              value={form.confirmPassword}
              onChange={(e) => setForm({ ...form, confirmPassword: e.target.value })}
              required
              minLength={6}
            />
            {form.confirmPassword && form.confirmPassword !== form.newPassword && (
              <p className="text-xs text-error mt-1">Passwords do not match</p>
            )}
          </fieldset>

          <div className="modal-action mt-4">
            <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-sm"
              disabled={!canSubmit}
            >
              {mutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <Lock className="h-3.5 w-3.5" />
              )}
              {mutation.isPending ? 'Changing…' : 'Change Password'}
            </button>
          </div>
        </form>
      </div>
      <div className="modal-backdrop" onClick={onClose} />
    </dialog>
  )
}
