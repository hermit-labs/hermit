import { formatDate } from '#/api'
import type { SkillVersionSummary } from '#/api'

export function VersionList({
  versions,
  latestVersion,
}: {
  versions: SkillVersionSummary[]
  latestVersion?: string
}) {
  if (versions.length === 0) {
    return (
      <div className="text-center py-12 text-base-content/40 text-sm">
        No versions found
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {versions.map((version) => (
        <div
          key={version.version}
          className="p-4 bg-base-200 rounded-lg border border-base-300"
        >
          <div className="flex justify-between items-start">
            <div>
              <h3 className="font-semibold text-sm">v{version.version}</h3>
              <p className="text-xs text-base-content/50 mt-0.5">
                {formatDate(version.createdAt)}
              </p>
            </div>
            {version.version === latestVersion && (
              <span className="badge badge-primary badge-sm">latest</span>
            )}
          </div>
          {version.changelog && (
            <p className="text-sm mt-2 text-base-content/60">
              {version.changelog.split('\n')[0]}
            </p>
          )}
        </div>
      ))}
    </div>
  )
}
