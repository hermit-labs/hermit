import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { Change } from 'diff'
import { diffLines } from 'diff'
import {
  ChevronDown,
  ChevronRight,
  FilePlus2,
  FileX2,
  FileEdit,
  FileText,
  Equal,
} from 'lucide-react'
import { api } from '#/api'
import type { SkillVersionSummary, SkillFile } from '#/api'
import { SkeletonLines } from './SkeletonLines'

type FileStatus = 'added' | 'removed' | 'modified' | 'unchanged'

interface DiffFile {
  path: string
  status: FileStatus
}

function useVersionFiles(slug: string, version: string | undefined) {
  return useQuery({
    queryKey: ['version-detail', slug, version],
    queryFn: () => api.getVersion(slug, version!),
    enabled: !!version,
  })
}

function useFileContent(
  slug: string,
  path: string | null,
  version: string | undefined,
) {
  return useQuery({
    queryKey: ['file-content', slug, path, version],
    queryFn: () => api.getFileContent(slug, path!, version),
    enabled: !!(path && version),
  })
}

function computeDiffFiles(
  oldFiles: SkillFile[],
  newFiles: SkillFile[],
): DiffFile[] {
  const oldPaths = new Set(oldFiles.map((f) => f.path))
  const newPaths = new Set(newFiles.map((f) => f.path))
  const allPaths = new Set([...oldPaths, ...newPaths])

  const result: DiffFile[] = []
  for (const path of allPaths) {
    if (!oldPaths.has(path)) {
      result.push({ path, status: 'added' })
    } else if (!newPaths.has(path)) {
      result.push({ path, status: 'removed' })
    } else {
      const oldSize = oldFiles.find((f) => f.path === path)?.size
      const newSize = newFiles.find((f) => f.path === path)?.size
      result.push({
        path,
        status: oldSize !== newSize ? 'modified' : 'modified',
      })
    }
  }

  const order: Record<FileStatus, number> = {
    modified: 0,
    added: 1,
    removed: 2,
    unchanged: 3,
  }
  result.sort((a, b) => order[a.status] - order[b.status] || a.path.localeCompare(b.path))
  return result
}

const STATUS_CONFIG: Record<
  FileStatus,
  { icon: typeof FileEdit; label: string; badgeClass: string }
> = {
  added: { icon: FilePlus2, label: 'Added', badgeClass: 'badge-success' },
  removed: { icon: FileX2, label: 'Removed', badgeClass: 'badge-error' },
  modified: { icon: FileEdit, label: 'Modified', badgeClass: 'badge-warning' },
  unchanged: { icon: Equal, label: 'Unchanged', badgeClass: 'badge-ghost' },
}

export function VersionCompare({
  slug,
  versions,
  latestVersion,
}: {
  slug: string
  versions: SkillVersionSummary[]
  latestVersion?: string
}) {
  const sortedVersions = useMemo(
    () => [...versions].sort((a, b) => b.createdAt - a.createdAt),
    [versions],
  )

  const latestIdx = sortedVersions.findIndex(
    (v) => v.version === latestVersion,
  )
  const defaultNew = sortedVersions[latestIdx >= 0 ? latestIdx : 0]?.version
  const defaultOld =
    sortedVersions[latestIdx >= 0 ? latestIdx + 1 : 1]?.version

  const [newVersion, setNewVersion] = useState<string | undefined>(defaultNew)
  const [oldVersion, setOldVersion] = useState<string | undefined>(defaultOld)

  const { data: newVerData, isLoading: loadingNew } = useVersionFiles(
    slug,
    newVersion,
  )
  const { data: oldVerData, isLoading: loadingOld } = useVersionFiles(
    slug,
    oldVersion,
  )

  const diffFiles = useMemo(() => {
    if (!newVerData?.version.files || !oldVerData?.version.files) return []
    return computeDiffFiles(
      oldVerData.version.files ?? [],
      newVerData.version.files ?? [],
    )
  }, [newVerData, oldVerData])

  const changedFiles = diffFiles.filter((f) => f.status !== 'unchanged')
  const stats = useMemo(() => {
    const s = { added: 0, removed: 0, modified: 0, unchanged: 0 }
    for (const f of diffFiles) s[f.status]++
    return s
  }, [diffFiles])

  if (sortedVersions.length < 2) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-2 text-base-content/40">
        <FileText className="h-8 w-8" />
        <p className="text-sm font-medium">Not enough versions to compare</p>
        <p className="text-xs">Publish at least 2 versions to use compare.</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Version selectors */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-base-content/50">Base</span>
          <select
            className="select select-bordered select-sm font-mono text-xs"
            value={newVersion ?? ''}
            onChange={(e) => setNewVersion(e.target.value || undefined)}
          >
            <option value="">Select version</option>
            {sortedVersions.map((v) => (
              <option key={v.version} value={v.version}>
                v{v.version}
                {v.version === latestVersion ? ' (latest)' : ''}
              </option>
            ))}
          </select>
        </div>

        <span className="text-base-content/30 text-lg">←</span>

        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-base-content/50">
            Compare
          </span>
          <select
            className="select select-bordered select-sm font-mono text-xs"
            value={oldVersion ?? ''}
            onChange={(e) => setOldVersion(e.target.value || undefined)}
          >
            <option value="">Select version</option>
            {sortedVersions.map((v) => (
              <option key={v.version} value={v.version}>
                v{v.version}
              </option>
            ))}
          </select>
        </div>
      </div>

      {(loadingNew || loadingOld) && (
        <div className="p-6">
          <SkeletonLines lines={6} height="h-3" />
        </div>
      )}

      {!loadingNew && !loadingOld && oldVersion && newVersion && (
        <>
          {/* Summary bar */}
          <div className="flex items-center gap-3 text-xs text-base-content/60 px-1">
            {stats.modified > 0 && (
              <span className="flex items-center gap-1">
                <FileEdit className="h-3.5 w-3.5 text-warning" />
                {stats.modified} modified
              </span>
            )}
            {stats.added > 0 && (
              <span className="flex items-center gap-1">
                <FilePlus2 className="h-3.5 w-3.5 text-success" />
                {stats.added} added
              </span>
            )}
            {stats.removed > 0 && (
              <span className="flex items-center gap-1">
                <FileX2 className="h-3.5 w-3.5 text-error" />
                {stats.removed} removed
              </span>
            )}
            {changedFiles.length === 0 && (
              <span className="text-base-content/40">
                No differences found between these versions
              </span>
            )}
          </div>

          {/* Diff panels */}
          <div className="border border-base-300 rounded-lg overflow-hidden divide-y divide-base-300">
            {changedFiles.map((file) => (
              <FileDiffPanel
                key={file.path}
                slug={slug}
                file={file}
                oldVersion={oldVersion}
                newVersion={newVersion}
              />
            ))}
          </div>
        </>
      )}
    </div>
  )
}

function FileDiffPanel({
  slug,
  file,
  oldVersion,
  newVersion,
}: {
  slug: string
  file: DiffFile
  oldVersion: string
  newVersion: string
}) {
  const [expanded, setExpanded] = useState(true)
  const cfg = STATUS_CONFIG[file.status]
  const Icon = cfg.icon

  const { data: oldContent } = useFileContent(
    slug,
    file.status !== 'added' ? file.path : null,
    oldVersion,
  )
  const { data: newContent } = useFileContent(
    slug,
    file.status !== 'removed' ? file.path : null,
    newVersion,
  )

  const changes = useMemo(() => {
    const old = file.status === 'added' ? '' : (oldContent ?? '')
    const cur = file.status === 'removed' ? '' : (newContent ?? '')
    if (old === cur) return null
    return diffLines(old, cur)
  }, [oldContent, newContent, file.status])

  const lineStats = useMemo(() => {
    if (!changes) return { added: 0, removed: 0 }
    let added = 0
    let removed = 0
    for (const c of changes) {
      const count = c.count || 0
      if (c.added) added += count
      if (c.removed) removed += count
    }
    return { added, removed }
  }, [changes])

  return (
    <div>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-left bg-base-200 hover:bg-base-300/50 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5 text-base-content/40 shrink-0" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 text-base-content/40 shrink-0" />
        )}
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className="font-mono text-xs truncate">{file.path}</span>
        <span className={`badge badge-xs ${cfg.badgeClass} ml-1`}>
          {cfg.label}
        </span>
        {lineStats.added > 0 && (
          <span className="text-[11px] font-mono text-success ml-auto">
            +{lineStats.added}
          </span>
        )}
        {lineStats.removed > 0 && (
          <span className="text-[11px] font-mono text-error ml-1">
            -{lineStats.removed}
          </span>
        )}
      </button>

      {expanded && (
        <div className="overflow-x-auto">
          {changes === null ? (
            <div className="px-4 py-3 text-xs text-base-content/40 italic">
              No changes in content
            </div>
          ) : (
            <DiffView changes={changes} />
          )}
        </div>
      )}
    </div>
  )
}

function DiffView({ changes }: { changes: Change[] }) {
  let oldLine = 1
  let newLine = 1

  return (
    <table className="diff-table w-full text-xs font-mono border-collapse">
      <tbody>
        {changes.map((change, ci) => {
          const lines = change.value.replace(/\n$/, '').split('\n')
          if (change.value === '' && lines.length === 1 && lines[0] === '')
            return null

          return lines.map((line, li) => {
            const key = `${ci}-${li}`

            if (change.added) {
              const ln = newLine++
              return (
                <tr key={key} className="diff-added">
                  <td className="diff-gutter diff-gutter-old" />
                  <td className="diff-gutter diff-gutter-new">{ln}</td>
                  <td className="diff-sign">+</td>
                  <td className="diff-code">
                    <pre>{line}</pre>
                  </td>
                </tr>
              )
            }

            if (change.removed) {
              const ln = oldLine++
              return (
                <tr key={key} className="diff-removed">
                  <td className="diff-gutter diff-gutter-old">{ln}</td>
                  <td className="diff-gutter diff-gutter-new" />
                  <td className="diff-sign">-</td>
                  <td className="diff-code">
                    <pre>{line}</pre>
                  </td>
                </tr>
              )
            }

            const oln = oldLine++
            const nln = newLine++
            return (
              <tr key={key} className="diff-context">
                <td className="diff-gutter diff-gutter-old">{oln}</td>
                <td className="diff-gutter diff-gutter-new">{nln}</td>
                <td className="diff-sign" />
                <td className="diff-code">
                  <pre>{line}</pre>
                </td>
              </tr>
            )
          })
        })}
      </tbody>
    </table>
  )
}
