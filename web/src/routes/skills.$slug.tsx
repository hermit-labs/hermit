import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Download, Star, Package, Flag, Terminal, Check, Copy } from 'lucide-react'
import { api, formatDate, timeAgo } from '#/api'
import { MarkdownContent } from '#/components/MarkdownContent'
import { FileExplorer } from '#/components/FileExplorer'
import { VersionList } from '#/components/VersionList'
import { SkeletonLines } from '#/components/SkeletonLines'
import { useState } from 'react'

export const Route = createFileRoute('/skills/$slug')({
  component: SkillDetailPage,
})

function SkillDetailPage() {
  const { slug } = Route.useParams()
  const [activeTab, setActiveTab] = useState<'files' | 'compare' | 'versions'>('files')

  const { data, isLoading, isError } = useQuery({
    queryKey: ['skill', slug],
    queryFn: () => api.getSkill(slug),
  })

  const { data: versionData } = useQuery({
    queryKey: ['skill-version', slug, data?.latestVersion?.version],
    queryFn: () => api.getVersion(slug, data!.latestVersion!.version),
    enabled: !!(data?.latestVersion?.version),
  })

  const { data: versionsListData } = useQuery({
    queryKey: ['skill-versions', slug],
    queryFn: () => api.listVersions(slug, { limit: 50 }),
    enabled: !!slug,
  })

  const versionFiles = versionData?.version.files
  const skillMdPath = versionFiles?.find(
    (f) => f.path === 'SKILL.md' || f.path === 'skills.md',
  )?.path

  const { data: skillMdContent, isLoading: loadingSkillMd } = useQuery({
    queryKey: ['skill-md', slug, data?.latestVersion?.version],
    queryFn: () => api.getFileContent(slug, skillMdPath!, data!.latestVersion!.version),
    enabled: !!(skillMdPath && data?.latestVersion?.version),
  })

  if (isLoading) return <div className="text-center py-16">Loading…</div>
  if (isError) return <div className="alert alert-error">Failed to load skill</div>
  if (!data) return <div>Not found</div>

  const { skill, latestVersion, owner } = data
  const fullVersion = versionData?.version
  const tags = Object.entries(skill.tags)
  const files = fullVersion?.files ?? []

  return (
    <div className="space-y-6 pb-16">
      <Link to="/" className="btn btn-ghost btn-sm -ml-2 inline-flex">
        ← Back
      </Link>

      {/* Header: title area + version sidebar */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6 items-start">
        {/* Left: skill info */}
        <div className="lg:col-span-3 space-y-3">
          <div>
            <h1 className="text-3xl font-bold">{skill.displayName}</h1>
            {skill.summary && (
              <p className="text-base-content/60 mt-1">{skill.summary}</p>
            )}
          </div>

          {/* Stats row */}
          <div className="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm text-base-content/60">
            <span className="flex items-center gap-1">
              <Star className="h-3.5 w-3.5" />
              {skill.stats.stars.toLocaleString()}
            </span>
            <span className="flex items-center gap-1">
              <Download className="h-3.5 w-3.5" />
              {skill.stats.downloads.toLocaleString()} downloads
            </span>
            <span className="flex items-center gap-1">
              <Package className="h-3.5 w-3.5" />
              {skill.stats.installsCurrent.toLocaleString()} current installs
              · {skill.stats.installsAllTime.toLocaleString()} all-time installs
            </span>
          </div>

          {/* Owner + time */}
          {owner && (
            <div className="flex items-center gap-2 text-sm text-base-content/50">
              <span>by</span>
              {owner.image ? (
                <img
                  src={owner.image}
                  alt={owner.displayName}
                  className="h-5 w-5 rounded-full object-cover"
                />
              ) : (
                <div className="h-5 w-5 rounded-full bg-primary/20 flex items-center justify-center text-xs font-bold text-primary">
                  {owner.displayName[0]}
                </div>
              )}
              <span className="font-medium text-base-content/70">
                {owner.displayName}
              </span>
              <span>·</span>
              <span>{timeAgo(skill.updatedAt)}</span>
            </div>
          )}

          <button className="btn btn-ghost btn-xs gap-1 text-base-content/40 hover:text-error -ml-1 mt-1">
            <Flag className="h-3 w-3" />
            Report
          </button>
        </div>

        {/* Right: version card */}
        {latestVersion && (
          <div className="lg:col-span-1">
            <div className="card bg-base-200 border sticky top-20">
              <div className="card-body p-4 gap-1">
                <p className="text-[10px] font-semibold tracking-widest uppercase text-base-content/40">
                  Current Version
                </p>
                <p className="text-2xl font-bold">v{latestVersion.version}</p>
                <p className="text-xs text-base-content/50 mb-2">
                  {formatDate(latestVersion.createdAt)}
                </p>
                <a
                  href={api.downloadUrl(slug)}
                  download
                  className="btn btn-primary btn-sm w-full"
                >
                  <Download className="h-4 w-4" />
                  Download zip
                </a>

                <div className="divider my-2 text-[10px] text-base-content/30">OR</div>

                <InstallCommand slug={slug} />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Version / tags badges */}
      {(latestVersion || tags.length > 0) && (
        <div className="flex flex-wrap items-center gap-2">
          {latestVersion && (
            <span className="badge badge-outline text-xs font-mono">
              latest v{latestVersion.version}
            </span>
          )}
          {tags.map(([tag, version]) => (
            <span key={tag} className="badge badge-ghost text-xs">
              {tag}: {version}
            </span>
          ))}
        </div>
      )}

      {/* Tabs + content */}
      <div>
        {/* Tab bar */}
        <div className="border-b border-base-300 flex">
          {(['files', 'compare', 'versions'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2.5 text-sm capitalize border-b-2 -mb-px transition-colors ${
                activeTab === tab
                  ? 'border-base-content text-base-content font-medium'
                  : 'border-transparent text-base-content/40 hover:text-base-content/70'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        <div className="mt-4">
          {/* Files tab */}
          {activeTab === 'files' && (
            files.length > 0 ? (
              <div className="space-y-6">
                {/* SKILL.md content */}
                {(loadingSkillMd || skillMdContent) && (
                  <div className="prose prose-sm max-w-none dark:prose-invert">
                    {loadingSkillMd ? (
                      <SkeletonLines lines={6} />
                    ) : (
                      <MarkdownContent content={skillMdContent!} stripHeading />
                    )}
                  </div>
                )}

                <FileExplorer
                  slug={slug}
                  version={latestVersion!.version}
                  files={files}
                />
              </div>
            ) : (
              <div className="text-center py-12 text-base-content/40 text-sm">
                No files in this version
              </div>
            )
          )}

          {/* Compare tab */}
          {activeTab === 'compare' && (
            <div className="text-center py-12 text-base-content/40 text-sm">
              Select two versions to compare
            </div>
          )}

          {/* Versions tab */}
          {activeTab === 'versions' && (
            <VersionList
              versions={versionsListData?.items ?? []}
              latestVersion={latestVersion?.version}
            />
          )}
        </div>
      </div>
    </div>
  )
}

function InstallCommand({ slug }: { slug: string }) {
  const [copied, setCopied] = useState(false)
  const command = `clawhub install --registry ${window.location.origin} ${slug}`

  const handleCopy = async () => {
    await navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-1.5 text-[10px] font-semibold tracking-widest uppercase text-base-content/40">
        <Terminal className="h-3 w-3" />
        Install via CLI
      </div>
      <div className="relative group">
        <pre className="bg-base-300 rounded-lg px-3 py-2.5 text-xs font-mono overflow-x-auto select-all">
          <code>{command}</code>
        </pre>
        <button
          onClick={handleCopy}
          className="absolute top-1.5 right-1.5 btn btn-ghost btn-xs opacity-0 group-hover:opacity-100 transition-opacity"
          aria-label="Copy command"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-success" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
        </button>
      </div>
    </div>
  )
}
