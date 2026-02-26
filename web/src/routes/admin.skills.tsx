import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Trash2, RotateCcw, AlertCircle, ArrowLeft } from 'lucide-react'
import { api } from '#/api'
import type { APIError, SkillSummary } from '#/api'

export const Route = createFileRoute('/admin/skills')({
  component: SkillManagementPage,
})

function SkillManagementPage() {
  const [sort, setSort] = useState('updated')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin-skills', sort],
    queryFn: () => api.listSkills({ sort, limit: 50 }),
  })

  const handleDelete = async (slug: string) => {
    setError(null)
    try {
      await api.deleteSkill(slug)
      setSuccess(`Deleted skill: ${slug}`)
      setConfirmDelete(null)
      refetch()
    } catch (err) {
      setError((err as APIError).message)
    }
  }

  const handleRestore = async (slug: string) => {
    setError(null)
    try {
      await api.undeleteSkill(slug)
      setSuccess(`Restored skill: ${slug}`)
      refetch()
    } catch (err) {
      setError((err as APIError).message)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-2xl font-bold">Skill Management</h1>
      </div>

      <div className="flex gap-2">
        <select
          value={sort}
          onChange={(e) => setSort(e.target.value)}
          className="select select-bordered flex-1 max-w-xs"
        >
          <option value="updated">Updated</option>
          <option value="downloads">Downloads</option>
          <option value="stars">Stars</option>
          <option value="trending">Trending</option>
        </select>
        <button onClick={() => refetch()} className="btn btn-ghost">
          Refresh
        </button>
      </div>

      {error && (
        <div className="alert alert-error">
          <AlertCircle className="h-4 w-4" />
          <span>{error}</span>
        </div>
      )}

      {success && (
        <div className="alert alert-success">
          <span>{success}</span>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="table">
          <thead>
            <tr>
              <th>Skill</th>
              <th>Slug</th>
              <th>Downloads</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={4} className="text-center py-8">
                  Loading skills…
                </td>
              </tr>
            ) : data?.items.length === 0 ? (
              <tr>
                <td
                  colSpan={4}
                  className="text-center py-8 text-base-content/60"
                >
                  No skills found
                </td>
              </tr>
            ) : (
              data?.items.map((skill) => (
                <tr key={skill.slug}>
                  <td>
                    <div>
                      <div className="font-semibold">{skill.displayName}</div>
                      <div className="text-sm text-base-content/60">
                        {skill.summary}
                      </div>
                    </div>
                  </td>
                  <td>
                    <code className="text-sm bg-base-300 px-2 py-1 rounded">
                      {skill.slug}
                    </code>
                  </td>
                  <td>{skill.stats.downloads.toLocaleString()}</td>
                  <td>
                    <div className="flex gap-2">
                      <SkillActions
                        skill={skill}
                        onDelete={handleDelete}
                        onRestore={handleRestore}
                        confirmDelete={confirmDelete}
                        setConfirmDelete={setConfirmDelete}
                      />
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function SkillActions({
  skill,
  onDelete,
  onRestore,
  confirmDelete,
  setConfirmDelete,
}: {
  skill: SkillSummary
  onDelete: (slug: string) => Promise<void>
  onRestore: (slug: string) => Promise<void>
  confirmDelete: string | null
  setConfirmDelete: (slug: string | null) => void
}) {
  const [isDeleting, setIsDeleting] = useState(false)
  const [isRestoring, setIsRestoring] = useState(false)

  if (confirmDelete === skill.slug) {
    return (
      <div className="flex gap-2 items-center">
        <span className="text-sm">Delete this skill?</span>
        <button
          onClick={async () => {
            setIsDeleting(true)
            try {
              await onDelete(skill.slug)
            } finally {
              setIsDeleting(false)
            }
          }}
          className="btn btn-error btn-xs"
          disabled={isDeleting}
        >
          {isDeleting ? 'Deleting…' : 'Confirm'}
        </button>
        <button
          onClick={() => setConfirmDelete(null)}
          className="btn btn-ghost btn-xs"
          disabled={isDeleting}
        >
          Cancel
        </button>
      </div>
    )
  }

  return (
    <>
      <button
        onClick={() => setConfirmDelete(skill.slug)}
        className="btn btn-error btn-sm btn-outline"
      >
        <Trash2 className="h-4 w-4" />
      </button>
      <button
        onClick={async () => {
          setIsRestoring(true)
          try {
            await onRestore(skill.slug)
          } finally {
            setIsRestoring(false)
          }
        }}
        className="btn btn-warning btn-sm btn-outline"
        disabled={isRestoring}
      >
        <RotateCcw className="h-4 w-4" />
      </button>
    </>
  )
}
