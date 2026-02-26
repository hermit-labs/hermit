import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Shield,
  Save,
  Trash2,
  ToggleLeft,
  ToggleRight,
  AlertCircle,
  CheckCircle,
  Lock,
} from 'lucide-react'
import { authConfigApi } from '#/api'
import type { APIError, AuthConfigItem } from '#/api'

const LDAP_FIELDS = [
  { key: 'url', label: 'LDAP URL', placeholder: 'ldap://ldap.example.com:389', required: true },
  { key: 'base_dn', label: 'Base DN', placeholder: 'dc=example,dc=com', required: true },
  { key: 'bind_dn', label: 'Bind DN', placeholder: 'cn=readonly,dc=example,dc=com' },
  { key: 'bind_password', label: 'Bind Password', placeholder: '••••••••', sensitive: true },
  { key: 'user_filter', label: 'User Filter', placeholder: '(uid={{.Username}})' },
  { key: 'user_attr', label: 'User Attribute', placeholder: 'uid' },
  { key: 'display_attr', label: 'Display Attribute', placeholder: 'cn' },
] as const

export function AuthConfigSection() {
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['auth-configs'],
    queryFn: () => authConfigApi.list(),
  })

  const configs = data?.configs ?? []
  const ldapConfig = configs.find((c) => c.provider_type === 'ldap')

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Shield className="h-5 w-5" />
        <h2 className="text-xl font-semibold">Authentication Providers</h2>
      </div>

      {error && (
        <div className="alert alert-error">
          <AlertCircle className="h-4 w-4" />
          <span>{error}</span>
          <button className="btn btn-ghost btn-xs" onClick={() => setError(null)}>✕</button>
        </div>
      )}
      {success && (
        <div className="alert alert-success">
          <CheckCircle className="h-4 w-4" />
          <span>{success}</span>
          <button className="btn btn-ghost btn-xs" onClick={() => setSuccess(null)}>✕</button>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-8">
          <span className="loading loading-dots loading-md" />
        </div>
      ) : (
        <div className="max-w-lg">
          <ProviderCard
            type="ldap"
            title="LDAP"
            icon={<Lock className="h-5 w-5" />}
            fields={LDAP_FIELDS}
            existing={ldapConfig}
            extraBoolFields={[
              { key: 'start_tls', label: 'StartTLS' },
              { key: 'skip_verify', label: 'Skip TLS Verify' },
            ]}
            onSuccess={(msg) => {
              setSuccess(msg)
              setError(null)
              queryClient.invalidateQueries({ queryKey: ['auth-configs'] })
            }}
            onError={(msg) => {
              setError(msg)
              setSuccess(null)
            }}
          />
        </div>
      )}
    </div>
  )
}

function ProviderCard({
  type,
  title,
  icon,
  fields,
  existing,
  extraBoolFields,
  onSuccess,
  onError,
}: {
  type: string
  title: string
  icon: React.ReactNode
  fields: readonly { key: string; label: string; placeholder?: string; required?: boolean; sensitive?: boolean }[]
  existing?: AuthConfigItem
  extraBoolFields?: { key: string; label: string }[]
  onSuccess: (msg: string) => void
  onError: (msg: string) => void
}) {
  const existingConfig = existing?.config ?? {}
  const [enabled, setEnabled] = useState(existing?.enabled ?? false)
  const [formData, setFormData] = useState<Record<string, string>>(() => {
    const data: Record<string, string> = {}
    for (const f of fields) {
      const val = existingConfig[f.key]
      data[f.key] = typeof val === 'string' ? (f.sensitive && val === '••••••••' ? '' : val) : ''
    }
    return data
  })
  const [boolData, setBoolData] = useState<Record<string, boolean>>(() => {
    const data: Record<string, boolean> = {}
    if (extraBoolFields) {
      for (const f of extraBoolFields) {
        data[f.key] = !!existingConfig[f.key]
      }
    }
    return data
  })

  const saveMutation = useMutation({
    mutationFn: () => {
      const config: Record<string, any> = {}
      for (const f of fields) {
        if (f.sensitive && formData[f.key] === '') {
          if (existingConfig[f.key] && existingConfig[f.key] !== '••••••••') {
            config[f.key] = existingConfig[f.key]
          }
        } else {
          config[f.key] = formData[f.key]
        }
      }
      if (extraBoolFields) {
        for (const f of extraBoolFields) {
          config[f.key] = boolData[f.key]
        }
      }
      if (typeof config.admin_groups === 'string') {
        config.admin_groups = config.admin_groups.split(',').map((s: string) => s.trim()).filter(Boolean)
      }
      return authConfigApi.save(type, enabled, config)
    },
    onSuccess: () => onSuccess(`${title} configuration saved`),
    onError: (err: APIError) => onError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: () => authConfigApi.remove(type),
    onSuccess: () => {
      setEnabled(false)
      const reset: Record<string, string> = {}
      for (const f of fields) reset[f.key] = ''
      setFormData(reset)
      onSuccess(`${title} configuration removed`)
    },
    onError: (err: APIError) => onError(err.message),
  })

  return (
    <div className={`card border ${enabled ? 'bg-base-200 border-primary/30' : 'bg-base-200'}`}>
      <div className="card-body">
        <div className="flex items-center justify-between mb-2">
          <h3 className="font-semibold flex items-center gap-2">
            {icon}
            {title}
          </h3>
          <button
            onClick={() => setEnabled(!enabled)}
            className={`btn btn-ghost btn-xs gap-1 ${enabled ? 'text-success' : 'text-base-content/40'}`}
          >
            {enabled ? (
              <><ToggleRight className="h-4 w-4" /> Enabled</>
            ) : (
              <><ToggleLeft className="h-4 w-4" /> Disabled</>
            )}
          </button>
        </div>

        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            saveMutation.mutate()
          }}
        >
          {fields.map((f) => (
            <div key={f.key} className="form-control">
              <label className="label py-0.5">
                <span className="label-text text-xs">
                  {f.label}
                  {f.required && <span className="text-error"> *</span>}
                </span>
              </label>
              <input
                type={f.sensitive ? 'password' : 'text'}
                placeholder={f.placeholder}
                className="input input-bordered input-sm w-full"
                value={formData[f.key] ?? ''}
                onChange={(e) => setFormData({ ...formData, [f.key]: e.target.value })}
              />
            </div>
          ))}

          {extraBoolFields?.map((f) => (
            <label key={f.key} className="flex items-center gap-2 cursor-pointer py-1">
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={boolData[f.key] ?? false}
                onChange={(e) => setBoolData({ ...boolData, [f.key]: e.target.checked })}
              />
              <span className="text-sm">{f.label}</span>
            </label>
          ))}

          <div className="flex gap-2 mt-2">
            <button
              type="submit"
              className="btn btn-primary btn-sm flex-1"
              disabled={saveMutation.isPending}
            >
              <Save className="h-3.5 w-3.5" />
              {saveMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            {existing && (
              <button
                type="button"
                className="btn btn-error btn-outline btn-sm"
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        </form>
      </div>
    </div>
  )
}
