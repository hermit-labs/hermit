import { useQuery } from '@tanstack/react-query'
import {
  Package,
  Download,
  Star,
  Layers,
  Activity,
  Database,
  CheckCircle,
} from 'lucide-react'
import { api } from '#/api'

export function StatsSection() {
  const { data: stats, isLoading } = useQuery({
    queryKey: ['admin-stats'],
    queryFn: () => api.getDashboardStats(),
    refetchInterval: 30_000,
  })

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">Overview</h2>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="card bg-base-200 border">
              <div className="card-body py-4">
                <div className="skeleton h-4 w-16" />
                <div className="skeleton h-8 w-20" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (!stats) return null

  const statCards = [
    {
      label: 'Skills',
      value: stats.totalSkills,
      icon: <Package className="h-5 w-5" />,
      color: 'text-primary',
    },
    {
      label: 'Versions',
      value: stats.totalVersions,
      icon: <Layers className="h-5 w-5" />,
      color: 'text-secondary',
    },
    {
      label: 'Downloads',
      value: stats.totalDownloads,
      icon: <Download className="h-5 w-5" />,
      color: 'text-accent',
    },
    {
      label: 'Stars',
      value: stats.totalStars,
      icon: <Star className="h-5 w-5" />,
      color: 'text-warning',
    },
    {
      label: 'Installs',
      value: stats.totalInstalls,
      icon: <Activity className="h-5 w-5" />,
      color: 'text-success',
    },
  ]

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold">Overview</h2>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {statCards.map((card) => (
          <div key={card.label} className="card bg-base-200 border">
            <div className="card-body py-4 gap-1">
              <div className={`flex items-center gap-2 ${card.color}`}>
                {card.icon}
                <span className="text-sm font-medium text-base-content/60">
                  {card.label}
                </span>
              </div>
              <p className="text-2xl font-bold tabular-nums">
                {card.value.toLocaleString()}
              </p>
            </div>
          </div>
        ))}
      </div>

      {stats.repositories.length > 0 && (
        <div className="card bg-base-200 border">
          <div className="card-body">
            <h3 className="font-semibold flex items-center gap-2">
              <Database className="h-4 w-4" />
              Repositories
            </h3>
            <div className="overflow-x-auto">
              <table className="table table-sm">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Upstream</th>
                    <th>Skills</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {stats.repositories.map((repo) => (
                    <tr key={repo.id}>
                      <td className="font-medium">{repo.name}</td>
                      <td>
                        <span
                          className={`badge badge-sm ${
                            repo.type === 'hosted'
                              ? 'badge-primary'
                              : repo.type === 'proxy'
                                ? 'badge-secondary'
                                : 'badge-accent'
                          }`}
                        >
                          {repo.type}
                        </span>
                      </td>
                      <td className="max-w-48 truncate text-xs text-base-content/60">
                        {repo.upstreamUrl || '—'}
                      </td>
                      <td className="tabular-nums">
                        {repo.skillCount.toLocaleString()}
                      </td>
                      <td>
                        {repo.enabled ? (
                          <span className="badge badge-success badge-sm gap-1">
                            <CheckCircle className="h-3 w-3" /> Active
                          </span>
                        ) : (
                          <span className="badge badge-error badge-sm gap-1">
                            Disabled
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
