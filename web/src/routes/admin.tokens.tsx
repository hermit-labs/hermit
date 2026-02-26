import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Copy,
  Check,
  Plus,
  Trash2,
  Key,
  Clock,
  AlertCircle,
  ArrowLeft,
  Terminal,
  ShieldAlert,
  KeyRound,
} from 'lucide-react'
import { api } from '#/api'
import type { APIError, PersonalAccessToken } from '#/api'

export const Route = createFileRoute('/admin/tokens')({
  component: TokenManagementPage,
})

function TokenManagementPage() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [createdToken, setCreatedToken] = useState<{
    token: string
    name: string
  } | null>(null)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const tokensQuery = useQuery({
    queryKey: ['my-tokens'],
    queryFn: () => api.listMyTokens(),
  })

  const createMutation = useMutation({
    mutationFn: (tokenName: string) => api.createMyToken(tokenName),
    onSuccess: (result) => {
      setCreatedToken({ token: result.token, name: result.name })
      setName('')
      setError(null)
      queryClient.invalidateQueries({ queryKey: ['my-tokens'] })
    },
    onError: (err: APIError) => {
      setError(err.message)
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (tokenId: string) => api.revokeMyToken(tokenId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my-tokens'] })
    },
  })

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const tokens = tokensQuery.data?.tokens ?? []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <div>
          <h1 className="text-2xl font-bold">Access Tokens</h1>
          <p className="text-sm text-base-content/60">
            Create and manage personal access tokens for the Hermit API & CLI
          </p>
        </div>
      </div>

      {/* Newly created token — prominent banner */}
      {createdToken && (
        <div className="alert alert-success shadow-lg">
          <div className="flex flex-col gap-2 w-full">
            <div className="flex items-center gap-2 font-semibold">
              <Check className="h-5 w-5 shrink-0" />
              Token &ldquo;{createdToken.name}&rdquo; created successfully
            </div>
            <p className="text-xs opacity-80">
              Copy this token now — it won&apos;t be shown again.
            </p>
            <div className="join w-full">
              <input
                type="text"
                value={createdToken.token}
                className="input input-bordered input-sm join-item flex-1 font-mono text-xs bg-success/10"
                readOnly
              />
              <button
                onClick={() => copyToClipboard(createdToken.token)}
                className="btn btn-sm join-item btn-outline"
              >
                {copied ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
                {copied ? 'Copied' : 'Copy'}
              </button>
            </div>
            <button
              onClick={() => {
                setCreatedToken(null)
                setCopied(false)
              }}
              className="btn btn-ghost btn-xs self-end opacity-70"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left column — Create + Usage */}
        <div className="space-y-6 lg:col-span-1">
          {/* Create new token */}
          <div className="card bg-base-200 border">
            <div className="card-body gap-4">
              <h2 className="card-title text-base">
                <Plus className="h-4 w-4 text-primary" />
                Generate new token
              </h2>

              {error && (
                <div className="alert alert-error text-sm py-2">
                  <AlertCircle className="h-4 w-4" />
                  <span>{error}</span>
                </div>
              )}

              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  if (name.trim()) createMutation.mutate(name.trim())
                }}
                className="space-y-3"
              >
                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs">Token name</legend>
                  <input
                    type="text"
                    placeholder="e.g. CI/CD, CLI, my-laptop"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="input input-bordered input-sm w-full"
                    disabled={createMutation.isPending}
                    required
                  />
                </fieldset>
                <button
                  type="submit"
                  className="btn btn-primary btn-sm w-full"
                  disabled={createMutation.isPending || !name.trim()}
                >
                  {createMutation.isPending ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <KeyRound className="h-3.5 w-3.5" />
                  )}
                  {createMutation.isPending ? 'Creating…' : 'Generate'}
                </button>
              </form>
            </div>
          </div>

          {/* Usage instructions */}
          <div className="card bg-base-200 border">
            <div className="card-body gap-3">
              <h3 className="card-title text-base">
                <Terminal className="h-4 w-4 text-primary" />
                Usage
              </h3>
              <p className="text-xs text-base-content/60">
                Include your token in the{' '}
                <code className="badge badge-xs font-mono">Authorization</code>{' '}
                header:
              </p>
              <div className="mockup-code text-xs">
                <pre data-prefix="$">
                  <code>{`curl -H "Authorization: Bearer <token>" \\`}</code>
                </pre>
                <pre data-prefix=" ">
                  <code>{`  http://localhost:8080/api/v1/skills`}</code>
                </pre>
              </div>
            </div>
          </div>

        </div>

        {/* Right column — Token list */}
        <div className="lg:col-span-2">
          <div className="card bg-base-200 border h-full">
            <div className="card-body">
              <div className="flex items-center justify-between">
                <h2 className="card-title text-base">
                  <Key className="h-4 w-4 text-primary" />
                  Active tokens
                </h2>
                {tokens.length > 0 && (
                  <div className="badge badge-ghost badge-sm">
                    {tokens.length}
                  </div>
                )}
              </div>

              {tokensQuery.isLoading && (
                <div className="flex justify-center py-12">
                  <span className="loading loading-spinner loading-md" />
                </div>
              )}

              {!tokensQuery.isLoading && tokens.length === 0 && (
                <div className="flex flex-col items-center justify-center py-12 gap-3 text-base-content/40">
                  <ShieldAlert className="h-10 w-10" />
                  <p className="text-sm">No tokens yet</p>
                  <p className="text-xs">
                    Generate your first token to get started
                  </p>
                </div>
              )}

              {tokens.length > 0 && (
                <div className="space-y-2 mt-2">
                  {tokens.map((t: PersonalAccessToken) => (
                    <div
                      key={t.id}
                      className="flex items-center gap-3 p-3 rounded-lg bg-base-100 border border-base-300 hover:border-base-content/20 transition-colors"
                    >
                      <div className="bg-primary/10 text-primary rounded-lg p-2">
                        <Key className="h-4 w-4" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="font-medium text-sm truncate">
                          {t.name || '(unnamed)'}
                        </p>
                        <div className="flex items-center gap-3 text-xs text-base-content/50 mt-0.5">
                          <span>
                            Created{' '}
                            {new Date(t.created_at).toLocaleDateString()}
                          </span>
                          {t.last_used_at ? (
                            <span className="flex items-center gap-1">
                              <Clock className="h-3 w-3" />
                              Used{' '}
                              {new Date(t.last_used_at).toLocaleDateString()}
                            </span>
                          ) : (
                            <span className="italic">Never used</span>
                          )}
                        </div>
                      </div>
                      <button
                        className="btn btn-ghost btn-sm text-error hover:bg-error/10"
                        onClick={() => {
                          if (confirm(`Revoke token "${t.name || t.id}"?`))
                            revokeMutation.mutate(t.id)
                        }}
                        disabled={revokeMutation.isPending}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        <span className="hidden sm:inline">Revoke</span>
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

