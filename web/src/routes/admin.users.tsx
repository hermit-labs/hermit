import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  UserPlus,
  Shield,
  ShieldOff,
  Trash2,
  KeyRound,
  UserCog,
  Check,
  X,
  AlertCircle,
  Users,
  Ban,
  CheckCircle,
  Eye,
  EyeOff,
} from 'lucide-react'
import { userApi } from '#/api'
import type { APIError, LocalUser } from '#/api'

export const Route = createFileRoute('/admin/users')({
  component: UserManagementPage,
})

function UserManagementPage() {
  const queryClient = useQueryClient()
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [editingUser, setEditingUser] = useState<string | null>(null)
  const [resetPwUser, setResetPwUser] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => userApi.list(),
  })

  const users = data?.users ?? []

  const flash = (msg: string) => {
    setSuccess(msg)
    setError(null)
    setTimeout(() => setSuccess(null), 3000)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold">User Management</h1>
            <p className="text-sm text-base-content/60">
              Create and manage local user accounts
            </p>
          </div>
        </div>
        <button
          className="btn btn-primary btn-sm"
          onClick={() => {
            setShowCreateForm(!showCreateForm)
            setError(null)
          }}
        >
          <UserPlus className="h-4 w-4" />
          New User
        </button>
      </div>

      {/* Alerts */}
      {error && (
        <div className="alert alert-error text-sm">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{error}</span>
          <button
            className="btn btn-ghost btn-xs"
            onClick={() => setError(null)}
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      )}
      {success && (
        <div className="alert alert-success text-sm">
          <CheckCircle className="h-4 w-4 shrink-0" />
          <span>{success}</span>
        </div>
      )}

      {/* Create user form */}
      {showCreateForm && (
        <CreateUserForm
          onSuccess={(username) => {
            setShowCreateForm(false)
            flash(`User "${username}" created`)
            queryClient.invalidateQueries({ queryKey: ['admin-users'] })
          }}
          onError={(msg) => setError(msg)}
          onCancel={() => setShowCreateForm(false)}
        />
      )}

      {/* User list */}
      <div className="card bg-base-200 border">
        <div className="card-body">
          <div className="flex items-center justify-between">
            <h2 className="card-title text-base">
              <Users className="h-4 w-4 text-primary" />
              All Users
            </h2>
            {users.length > 0 && (
              <div className="badge badge-ghost badge-sm">
                {users.length}
              </div>
            )}
          </div>

          {isLoading && (
            <div className="flex justify-center py-12">
              <span className="loading loading-spinner loading-md" />
            </div>
          )}

          {!isLoading && users.length === 0 && (
            <div className="flex flex-col items-center justify-center py-12 gap-3 text-base-content/40">
              <Users className="h-10 w-10" />
              <p className="text-sm">No users yet</p>
            </div>
          )}

          {users.length > 0 && (
            <div className="space-y-2 mt-2">
              {users.map((u) => (
                <div key={u.id}>
                  <UserRow
                    user={u}
                    isEditing={editingUser === u.id}
                    isResettingPw={resetPwUser === u.id}
                    onEdit={() => {
                      setEditingUser(
                        editingUser === u.id ? null : u.id,
                      )
                      setResetPwUser(null)
                    }}
                    onResetPw={() => {
                      setResetPwUser(
                        resetPwUser === u.id ? null : u.id,
                      )
                      setEditingUser(null)
                    }}
                    onSuccess={(msg) => {
                      flash(msg)
                      setEditingUser(null)
                      setResetPwUser(null)
                      queryClient.invalidateQueries({
                        queryKey: ['admin-users'],
                      })
                    }}
                    onError={(msg) => setError(msg)}
                    onCancelEdit={() => setEditingUser(null)}
                    onCancelReset={() => setResetPwUser(null)}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

/* ---- Create User Form ---- */

function CreateUserForm({
  onSuccess,
  onError,
  onCancel,
}: {
  onSuccess: (username: string) => void
  onError: (msg: string) => void
  onCancel: () => void
}) {
  const [form, setForm] = useState({
    username: '',
    password: '',
    display_name: '',
    email: '',
    is_admin: false,
  })
  const [showPw, setShowPw] = useState(false)

  const mutation = useMutation({
    mutationFn: () => userApi.create(form),
    onSuccess: () => onSuccess(form.username),
    onError: (err: APIError) => onError(err.message),
  })

  return (
    <div className="card bg-base-200 border">
      <div className="card-body gap-4">
        <h3 className="card-title text-base">
          <UserPlus className="h-4 w-4 text-primary" />
          Create New User
        </h3>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            mutation.mutate()
          }}
          className="grid grid-cols-1 sm:grid-cols-2 gap-4"
        >
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Username *</legend>
            <input
              type="text"
              placeholder="e.g. alice"
              className="input input-bordered input-sm w-full"
              value={form.username}
              onChange={(e) =>
                setForm({ ...form, username: e.target.value })
              }
              required
            />
          </fieldset>
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Password *</legend>
            <div className="join w-full">
              <input
                type={showPw ? 'text' : 'password'}
                placeholder="Min 6 characters"
                className="input input-bordered input-sm join-item flex-1"
                value={form.password}
                onChange={(e) =>
                  setForm({ ...form, password: e.target.value })
                }
                required
                minLength={6}
              />
              <button
                type="button"
                className="btn btn-ghost btn-sm join-item"
                onClick={() => setShowPw(!showPw)}
              >
                {showPw ? (
                  <EyeOff className="h-3.5 w-3.5" />
                ) : (
                  <Eye className="h-3.5 w-3.5" />
                )}
              </button>
            </div>
          </fieldset>
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Display Name</legend>
            <input
              type="text"
              placeholder="e.g. Alice Smith"
              className="input input-bordered input-sm w-full"
              value={form.display_name}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
            />
          </fieldset>
          <fieldset className="fieldset">
            <legend className="fieldset-legend text-xs">Email</legend>
            <input
              type="email"
              placeholder="e.g. alice@company.com"
              className="input input-bordered input-sm w-full"
              value={form.email}
              onChange={(e) =>
                setForm({ ...form, email: e.target.value })
              }
            />
          </fieldset>
          <div className="sm:col-span-2">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                className="toggle toggle-sm toggle-primary"
                checked={form.is_admin}
                onChange={(e) =>
                  setForm({ ...form, is_admin: e.target.checked })
                }
              />
              <div>
                <span className="text-sm font-medium">Administrator</span>
                <p className="text-xs text-base-content/50">
                  Full access to all admin features
                </p>
              </div>
            </label>
          </div>
          <div className="sm:col-span-2 flex gap-2 justify-end">
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={onCancel}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-sm"
              disabled={mutation.isPending}
            >
              {mutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <UserPlus className="h-3.5 w-3.5" />
              )}
              Create User
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

/* ---- User Row ---- */

function UserRow({
  user,
  isEditing,
  isResettingPw,
  onEdit,
  onResetPw,
  onSuccess,
  onError,
  onCancelEdit,
  onCancelReset,
}: {
  user: LocalUser
  isEditing: boolean
  isResettingPw: boolean
  onEdit: () => void
  onResetPw: () => void
  onSuccess: (msg: string) => void
  onError: (msg: string) => void
  onCancelEdit: () => void
  onCancelReset: () => void
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => userApi.remove(user.id),
    onSuccess: () => onSuccess(`User "${user.username}" deleted`),
    onError: (err: APIError) => onError(err.message),
  })

  return (
    <div
      className={`rounded-lg border transition-colors ${user.disabled
        ? 'bg-base-100/50 border-base-300 opacity-60'
        : 'bg-base-100 border-base-300 hover:border-base-content/20'
        }`}
    >
      {/* Main row */}
      <div className="flex items-center gap-3 p-3">
        <div
          className={`rounded-lg p-2 ${user.is_admin
            ? 'bg-error/10 text-error'
            : 'bg-primary/10 text-primary'
            }`}
        >
          {user.is_admin ? (
            <Shield className="h-4 w-4" />
          ) : (
            <UserCog className="h-4 w-4" />
          )}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <p className="font-medium text-sm truncate">
              {user.username}
            </p>
            {user.is_admin && (
              <span className="badge badge-error badge-xs">Admin</span>
            )}
            {user.disabled && (
              <span className="badge badge-warning badge-xs">
                Disabled
              </span>
            )}
          </div>
          <div className="flex items-center gap-3 text-xs text-base-content/50 mt-0.5">
            {user.display_name && <span>{user.display_name}</span>}
            {user.email && (
              <span className="truncate">{user.email}</span>
            )}
            <span>
              Joined{' '}
              {new Date(user.created_at).toLocaleDateString()}
            </span>
          </div>
        </div>
        <div className="flex gap-1">
          <button
            className="btn btn-ghost btn-xs tooltip"
            data-tip="Edit"
            onClick={onEdit}
          >
            <UserCog className="h-3.5 w-3.5" />
          </button>
          <button
            className="btn btn-ghost btn-xs tooltip"
            data-tip="Reset password"
            onClick={onResetPw}
          >
            <KeyRound className="h-3.5 w-3.5" />
          </button>
          {confirmDelete ? (
            <div className="flex items-center gap-1">
              <span className="text-xs text-error">Delete?</span>
              <button
                className="btn btn-error btn-xs"
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
              >
                <Check className="h-3 w-3" />
              </button>
              <button
                className="btn btn-ghost btn-xs"
                onClick={() => setConfirmDelete(false)}
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ) : (
            <button
              className="btn btn-ghost btn-xs text-error tooltip"
              data-tip="Delete"
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Edit panel */}
      {isEditing && (
        <EditUserPanel
          user={user}
          onSuccess={onSuccess}
          onError={onError}
          onCancel={onCancelEdit}
        />
      )}

      {/* Reset password panel */}
      {isResettingPw && (
        <ResetPasswordPanel
          user={user}
          onSuccess={onSuccess}
          onError={onError}
          onCancel={onCancelReset}
        />
      )}
    </div>
  )
}

/* ---- Edit User Panel ---- */

function EditUserPanel({
  user,
  onSuccess,
  onError,
  onCancel,
}: {
  user: LocalUser
  onSuccess: (msg: string) => void
  onError: (msg: string) => void
  onCancel: () => void
}) {
  const [form, setForm] = useState({
    display_name: user.display_name,
    email: user.email,
    is_admin: user.is_admin,
    disabled: user.disabled,
  })

  const mutation = useMutation({
    mutationFn: () => userApi.update(user.id, form),
    onSuccess: () => onSuccess(`User "${user.username}" updated`),
    onError: (err: APIError) => onError(err.message),
  })

  return (
    <div className="border-t border-base-300 p-3 bg-base-200/50">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          mutation.mutate()
        }}
        className="grid grid-cols-1 sm:grid-cols-2 gap-3"
      >
        <fieldset className="fieldset">
          <legend className="fieldset-legend text-xs">Display Name</legend>
          <input
            type="text"
            className="input input-bordered input-sm w-full"
            value={form.display_name}
            onChange={(e) =>
              setForm({ ...form, display_name: e.target.value })
            }
          />
        </fieldset>
        <fieldset className="fieldset">
          <legend className="fieldset-legend text-xs">Email</legend>
          <input
            type="email"
            className="input input-bordered input-sm w-full"
            value={form.email}
            onChange={(e) =>
              setForm({ ...form, email: e.target.value })
            }
          />
        </fieldset>
        <div className="flex gap-6 sm:col-span-2">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              className="toggle toggle-sm toggle-primary"
              checked={form.is_admin}
              onChange={(e) =>
                setForm({ ...form, is_admin: e.target.checked })
              }
            />
            <span className="text-sm flex items-center gap-1">
              <Shield className="h-3.5 w-3.5" />
              Admin
            </span>
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              className="toggle toggle-sm toggle-warning"
              checked={form.disabled}
              onChange={(e) =>
                setForm({ ...form, disabled: e.target.checked })
              }
            />
            <span className="text-sm flex items-center gap-1">
              {form.disabled ? (
                <Ban className="h-3.5 w-3.5" />
              ) : (
                <ShieldOff className="h-3.5 w-3.5" />
              )}
              Disabled
            </span>
          </label>
        </div>
        <div className="sm:col-span-2 flex gap-2 justify-end">
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="btn btn-primary btn-xs"
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <Check className="h-3 w-3" />
            )}
            Save
          </button>
        </div>
      </form>
    </div>
  )
}

/* ---- Reset Password Panel ---- */

function ResetPasswordPanel({
  user,
  onSuccess,
  onError,
  onCancel,
}: {
  user: LocalUser
  onSuccess: (msg: string) => void
  onError: (msg: string) => void
  onCancel: () => void
}) {
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)

  const mutation = useMutation({
    mutationFn: () => userApi.resetPassword(user.id, password),
    onSuccess: () =>
      onSuccess(`Password reset for "${user.username}"`),
    onError: (err: APIError) => onError(err.message),
  })

  return (
    <div className="border-t border-base-300 p-3 bg-base-200/50">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          mutation.mutate()
        }}
        className="flex items-end gap-3"
      >
        <fieldset className="fieldset flex-1">
          <legend className="fieldset-legend text-xs">
            New password for <strong>{user.username}</strong>
          </legend>
          <div className="join w-full">
            <input
              type={showPw ? 'text' : 'password'}
              placeholder="Min 6 characters"
              className="input input-bordered input-sm join-item flex-1"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={6}
            />
            <button
              type="button"
              className="btn btn-ghost btn-sm join-item"
              onClick={() => setShowPw(!showPw)}
            >
              {showPw ? (
                <EyeOff className="h-3.5 w-3.5" />
              ) : (
                <Eye className="h-3.5 w-3.5" />
              )}
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={onCancel}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-sm"
              disabled={mutation.isPending || password.length < 6}
            >
              {mutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <KeyRound className="h-3.5 w-3.5" />
              )}
              Reset
            </button>
          </div>
        </fieldset>

      </form>
    </div>
  )
}
