import { useState, useRef } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Upload, X, FileText, CheckCircle2, KeyRound } from 'lucide-react'
import type { APIError } from '#/api'
import { api, formatBytes, getToken } from '#/api'

export const Route = createFileRoute('/publish')({
  component: PublishPage,
})

function PublishPage() {
  const navigate = useNavigate()
  const token = getToken()
  const [form, setForm] = useState({
    slug: '',
    displayName: '',
    version: '',
    changelog: '',
    summary: '',
    tags: 'latest',
  })
  const [files, setFiles] = useState<File[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<{
    ok: boolean
    skillId?: string
    error?: string
  } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  if (!token)
    return (
      <div className="text-center py-16 max-w-md mx-auto">
        <div className="bg-base-200 rounded-2xl p-8 inline-block mb-4">
          <KeyRound className="h-10 w-10 mx-auto text-base-content/40" />
        </div>
        <h1 className="text-2xl font-bold mb-2">Authentication required</h1>
        <p className="text-base-content/60 mb-4">
          Set your API token to publish skills
        </p>
        <Link to="/" className="btn btn-outline">
          Back home
        </Link>
      </div>
    )

  if (result?.ok)
    return (
      <div className="text-center py-16 max-w-md mx-auto">
        <div className="bg-success/10 rounded-2xl p-8 inline-block mb-4">
          <CheckCircle2 className="h-10 w-10 mx-auto text-success" />
        </div>
        <h1 className="text-2xl font-bold mb-2">Published!</h1>
        <div className="flex gap-2 justify-center mt-6">
          <button
            className="btn btn-primary"
            onClick={() =>
              navigate({ to: '/skills/$slug', params: { slug: form.slug } })
            }
          >
            View
          </button>
          <button
            className="btn btn-ghost"
            onClick={() => {
              setResult(null)
              setForm({
                slug: '',
                displayName: '',
                version: '',
                changelog: '',
                summary: '',
                tags: 'latest',
              })
              setFiles([])
            }}
          >
            Publish another
          </button>
        </div>
      </div>
    )

  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault()
        if (!files.length) return

        setSubmitting(true)
        try {
          const res = await api.publishSkill(
            {
              slug: form.slug,
              displayName: form.displayName,
              version: form.version,
              changelog: form.changelog,
              summary: form.summary || undefined,
              tags: form.tags
                .split(',')
                .map((t) => t.trim())
                .filter(Boolean),
            },
            files,
          )
          setResult({ ok: true, skillId: res.skillId })
        } catch (err) {
          setResult({ ok: false, error: (err as APIError).message })
        } finally {
          setSubmitting(false)
        }
      }}
      className="max-w-2xl space-y-4"
    >
      {result?.error && <div className="alert alert-error">{result.error}</div>}

      <div className="card bg-base-200">
        <div className="card-body">
          <h2 className="card-title">Metadata</h2>
          <div className="grid grid-cols-2 gap-3">
            <input
              type="text"
              placeholder="Slug"
              value={form.slug}
              onChange={(e) => setForm({ ...form, slug: e.target.value })}
              className="input input-bordered"
              required
            />
            <input
              type="text"
              placeholder="Display name"
              value={form.displayName}
              onChange={(e) =>
                setForm({ ...form, displayName: e.target.value })
              }
              className="input input-bordered"
              required
            />
            <input
              type="text"
              placeholder="Version"
              value={form.version}
              onChange={(e) => setForm({ ...form, version: e.target.value })}
              className="input input-bordered"
              required
            />
            <input
              type="text"
              placeholder="Tags (comma-separated)"
              value={form.tags}
              onChange={(e) => setForm({ ...form, tags: e.target.value })}
              className="input input-bordered"
            />
          </div>
          <textarea
            placeholder="Summary"
            value={form.summary}
            onChange={(e) => setForm({ ...form, summary: e.target.value })}
            className="textarea textarea-bordered"
          />
          <textarea
            placeholder="Changelog"
            value={form.changelog}
            onChange={(e) => setForm({ ...form, changelog: e.target.value })}
            className="textarea textarea-bordered"
            required
          />
        </div>
      </div>

      <div className="card bg-base-200">
        <div className="card-body">
          <h2 className="card-title">Files</h2>
          <div
            className="border-2 border-dashed rounded p-6 text-center cursor-pointer hover:border-primary"
            onClick={() => fileInputRef.current?.click()}
            onDrop={(e) => {
              e.preventDefault()
              setFiles((p) => [...p, ...Array.from(e.dataTransfer.files)])
            }}
            onDragOver={(e) => e.preventDefault()}
          >
            <Upload className="h-6 w-6 mx-auto mb-2 text-base-content/50" />
            <p className="text-sm">Drop files or click to select</p>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={(e) => {
                if (e.target.files)
                  setFiles((p) => [...p, ...Array.from(e.target.files!)])
              }}
            />
          </div>
          {files.length > 0 && (
            <div className="space-y-1">
              {files.map((f) => (
                <div
                  key={f.name}
                  className="flex justify-between items-center text-sm bg-base-300 p-2 rounded"
                >
                  <span className="flex items-center gap-2">
                    <FileText className="h-4 w-4" /> {f.name}
                  </span>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-base-content/50">
                      {formatBytes(f.size)}
                    </span>
                    <button
                      type="button"
                      onClick={() =>
                        setFiles((p) => p.filter((x) => x.name !== f.name))
                      }
                      className="btn btn-ghost btn-xs btn-circle"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="flex gap-2">
        <button
          type="submit"
          className="btn btn-primary"
          disabled={submitting || !files.length}
        >
          {submitting ? 'Publishing…' : 'Publish'}
        </button>
        <Link to="/" className="btn btn-ghost">
          Cancel
        </Link>
      </div>
    </form>
  )
}
