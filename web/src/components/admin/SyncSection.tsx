import { useState, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  RefreshCw,
  Plus,
  Trash2,
  Globe,
  ToggleLeft,
  ToggleRight,
  AlertCircle,
  CheckCircle,
  Settings,
  Save,
} from 'lucide-react'
import { api } from '#/api'
import type { APIError, SyncSource, ProxySyncConfig } from '#/api'

export function SyncSection() {
  const queryClient = useQueryClient()
  const [showAddForm, setShowAddForm] = useState(false)
  const [newName, setNewName] = useState('')
  const [newUrl, setNewUrl] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const { data: sourcesData, isLoading: sourcesLoading } = useQuery({
    queryKey: ['sync-sources'],
    queryFn: () => api.listSyncSources(),
  })

  const { data: syncStatus } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => api.getSyncStatus(),
    refetchInterval: 3_000,
  })

  const triggerMutation = useMutation({
    mutationFn: () => api.triggerSync(),
    onSuccess: (data) => {
      setSuccess(data.message)
      setError(null)
      queryClient.invalidateQueries({ queryKey: ['sync-status'] })
    },
    onError: (err: APIError) => {
      setError(err.message)
      setSuccess(null)
    },
  })

  const addMutation = useMutation({
    mutationFn: ({ name, upstreamUrl }: { name: string; upstreamUrl: string }) =>
      api.addSyncSource(name, upstreamUrl),
    onSuccess: () => {
      setNewName('')
      setNewUrl('')
      setShowAddForm(false)
      setSuccess('Sync source added')
      setError(null)
      queryClient.invalidateQueries({ queryKey: ['sync-sources'] })
      queryClient.invalidateQueries({ queryKey: ['admin-stats'] })
    },
    onError: (err: APIError) => {
      setError(err.message)
      setSuccess(null)
    },
  })

  const removeMutation = useMutation({
    mutationFn: (id: string) => api.removeSyncSource(id),
    onSuccess: () => {
      setSuccess('Sync source removed')
      setError(null)
      queryClient.invalidateQueries({ queryKey: ['sync-sources'] })
      queryClient.invalidateQueries({ queryKey: ['admin-stats'] })
    },
    onError: (err: APIError) => {
      setError(err.message)
      setSuccess(null)
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.toggleSyncSource(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sync-sources'] })
      queryClient.invalidateQueries({ queryKey: ['admin-stats'] })
    },
    onError: (err: APIError) => setError(err.message),
  })

  const sources = sourcesData?.sources ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Sync Sources</h2>
        <div className="flex gap-2">
          <button
            className="btn btn-sm btn-outline"
            onClick={() => setShowAddForm(!showAddForm)}
          >
            <Plus className="h-4 w-4" />
            Add Source
          </button>
          <button
            className={`btn btn-sm btn-primary ${syncStatus?.running ? 'btn-disabled' : ''}`}
            onClick={() => triggerMutation.mutate()}
            disabled={triggerMutation.isPending || syncStatus?.running}
          >
            <RefreshCw className={`h-4 w-4 ${syncStatus?.running ? 'animate-spin' : ''}`} />
            {syncStatus?.running ? 'Syncing…' : 'Sync Now'}
          </button>
        </div>
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

      {syncStatus && (
        <SyncStatusCard
          configured={syncStatus.configured}
          running={syncStatus.running}
          lastResult={syncStatus.lastResult}
          lastError={syncStatus.lastError}
        />
      )}

      <SyncConfigCard
        onSuccess={(msg) => { setSuccess(msg); setError(null) }}
        onError={(msg) => { setError(msg); setSuccess(null) }}
      />

      {showAddForm && (
        <div className="card bg-base-200 border">
          <div className="card-body">
            <h3 className="font-semibold">Add Sync Source</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                addMutation.mutate({ name: newName.trim(), upstreamUrl: newUrl.trim() })
              }}
              className="flex flex-col sm:flex-row gap-3"
            >
              <input
                type="text"
                placeholder="Name (e.g. proxy-2)"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="input input-bordered flex-1"
                required
              />
              <input
                type="url"
                placeholder="Upstream URL (e.g. https://clawhub.ai)"
                value={newUrl}
                onChange={(e) => setNewUrl(e.target.value)}
                className="input input-bordered flex-2"
                required
              />
              <div className="flex gap-2">
                <button type="submit" className="btn btn-primary" disabled={addMutation.isPending}>
                  {addMutation.isPending ? 'Adding…' : 'Add'}
                </button>
                <button type="button" className="btn btn-ghost" onClick={() => setShowAddForm(false)}>
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="card bg-base-200 border">
        <div className="card-body">
          {sourcesLoading ? (
            <div className="flex justify-center py-8">
              <span className="loading loading-dots loading-md" />
            </div>
          ) : sources.length === 0 ? (
            <div className="text-center py-8 text-base-content/60">
              <Globe className="h-8 w-8 mx-auto mb-2 opacity-40" />
              <p>No sync sources configured</p>
              <p className="text-sm">Add a proxy upstream to start mirroring skills</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Upstream URL</th>
                    <th>Skills</th>
                    <th>Status</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sources.map((source) => (
                    <SyncSourceRow
                      key={source.id}
                      source={source}
                      onRemove={(id) => removeMutation.mutate(id)}
                      onToggle={(id, enabled) => toggleMutation.mutate({ id, enabled })}
                      isRemoving={removeMutation.isPending}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function SyncSourceRow({
  source,
  onRemove,
  onToggle,
  isRemoving,
}: {
  source: SyncSource
  onRemove: (id: string) => void
  onToggle: (id: string, enabled: boolean) => void
  isRemoving: boolean
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)

  return (
    <tr>
      <td className="font-medium">{source.name}</td>
      <td className="max-w-64 truncate text-sm text-base-content/70">
        <a href={source.upstreamUrl} target="_blank" rel="noreferrer" className="link link-hover">
          {source.upstreamUrl}
        </a>
      </td>
      <td className="tabular-nums">{source.skillCount.toLocaleString()}</td>
      <td>
        <button
          onClick={() => onToggle(source.id, !source.enabled)}
          className={`btn btn-ghost btn-xs gap-1 ${source.enabled ? 'text-success' : 'text-error'}`}
        >
          {source.enabled ? (
            <><ToggleRight className="h-4 w-4" /> Enabled</>
          ) : (
            <><ToggleLeft className="h-4 w-4" /> Disabled</>
          )}
        </button>
      </td>
      <td>
        {confirmDelete ? (
          <div className="flex gap-1 items-center">
            <span className="text-xs">Delete?</span>
            <button onClick={() => { onRemove(source.id); setConfirmDelete(false) }} className="btn btn-error btn-xs" disabled={isRemoving}>Yes</button>
            <button onClick={() => setConfirmDelete(false)} className="btn btn-ghost btn-xs">No</button>
          </div>
        ) : (
          <button onClick={() => setConfirmDelete(true)} className="btn btn-ghost btn-xs text-error">
            <Trash2 className="h-4 w-4" />
          </button>
        )}
      </td>
    </tr>
  )
}

function SyncStatusCard({
  configured,
  running,
  lastResult,
  lastError,
}: {
  configured: boolean
  running: boolean
  lastResult: { Repositories: number; Skills: number; Versions: number; Cached: number; Failed: number } | null
  lastError: string
}) {
  if (!configured) {
    return (
      <div className="card bg-warning/10 border border-warning/30">
        <div className="card-body py-3 flex-row items-center gap-3">
          <AlertCircle className="h-5 w-5 text-warning shrink-0" />
          <div>
            <p className="font-medium text-sm">Sync is not configured</p>
            <p className="text-xs text-base-content/60">
              Add a sync source and use &ldquo;Sync Now&rdquo; to start
            </p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={`card border ${running ? 'bg-info/10 border-info/30' : lastError ? 'bg-error/10 border-error/30' : 'bg-success/10 border-success/30'}`}>
      <div className="card-body py-3">
        <div className="flex items-center gap-3">
          {running ? (
            <RefreshCw className="h-5 w-5 text-info animate-spin shrink-0" />
          ) : lastError ? (
            <AlertCircle className="h-5 w-5 text-error shrink-0" />
          ) : (
            <CheckCircle className="h-5 w-5 text-success shrink-0" />
          )}
          <div className="flex-1">
            <p className="font-medium text-sm">
              {running ? 'Sync in progress…' : lastError ? 'Last sync had errors' : 'Sync idle'}
            </p>
            {lastError && <p className="text-xs text-error mt-1 line-clamp-2">{lastError}</p>}
          </div>
          {lastResult && !running && (
            <div className="flex gap-4 text-xs text-base-content/60">
              <span><strong>{lastResult.Repositories}</strong> repos</span>
              <span><strong>{lastResult.Skills}</strong> skills</span>
              <span><strong>{lastResult.Cached}</strong> cached</span>
              {lastResult.Failed > 0 && (
                <span className="text-error"><strong>{lastResult.Failed}</strong> failed</span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function SyncConfigCard({
  onSuccess,
  onError,
}: {
  onSuccess: (msg: string) => void
  onError: (msg: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const { data: config, isLoading } = useQuery({
    queryKey: ['proxy-sync-config'],
    queryFn: () => api.getProxySyncConfig(),
  })

  const [form, setForm] = useState<ProxySyncConfig>({
    page_size: 100,
    concurrency: 4,
  })

  useEffect(() => {
    if (config) setForm(config)
  }, [config])

  const queryClient = useQueryClient()

  const saveMutation = useMutation({
    mutationFn: () => api.saveProxySyncConfig(form),
    onSuccess: () => {
      onSuccess('Sync configuration saved')
      queryClient.invalidateQueries({ queryKey: ['proxy-sync-config'] })
    },
    onError: (err: APIError) => onError(err.message),
  })

  if (isLoading) return null

  return (
    <div className="card bg-base-200 border">
      <div className="card-body py-3">
        <button
          className="flex items-center justify-between w-full text-left"
          onClick={() => setExpanded(!expanded)}
        >
          <h3 className="font-semibold flex items-center gap-2 text-sm">
            <Settings className="h-4 w-4" />
            Sync Settings
          </h3>
          <span className="text-xs text-base-content/40">
            {expanded ? '▲' : '▼'}
          </span>
        </button>

        {expanded && (
          <form
            className="mt-3 space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              saveMutation.mutate()
            }}
          >
            <div className="grid grid-cols-2 gap-3">
              <div className="form-control">
                <label className="label py-0.5">
                  <span className="label-text text-xs">Page Size</span>
                </label>
                <input
                  type="number"
                  min={1}
                  className="input input-bordered input-sm"
                  value={form.page_size}
                  onChange={(e) => setForm({ ...form, page_size: parseInt(e.target.value) || 100 })}
                />
              </div>
              <div className="form-control">
                <label className="label py-0.5">
                  <span className="label-text text-xs">Concurrency</span>
                </label>
                <input
                  type="number"
                  min={1}
                  className="input input-bordered input-sm"
                  value={form.concurrency}
                  onChange={(e) => setForm({ ...form, concurrency: parseInt(e.target.value) || 4 })}
                />
              </div>
            </div>

            <button
              type="submit"
              className="btn btn-primary btn-sm"
              disabled={saveMutation.isPending}
            >
              <Save className="h-3.5 w-3.5" />
              {saveMutation.isPending ? 'Saving…' : 'Save Settings'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
