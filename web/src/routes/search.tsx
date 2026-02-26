import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { SearchX } from 'lucide-react'
import { api } from '#/api'
import { SkillCard } from './index'

export const Route = createFileRoute('/search')({
  validateSearch: (s: Record<string, unknown>) => ({
    q: (s.q as string) || '',
  }),
  component: SearchPage,
})

function SearchPage() {
  const { q } = Route.useSearch()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['search', q],
    queryFn: () => api.searchSkills(q),
    enabled: q.length > 0,
  })

  if (!q)
    return (
      <div className="text-center py-24">
        <p className="text-base-content/60">
          Use the search bar to find skills
        </p>
        <Link to="/" className="link mt-2 block">
          Browse all
        </Link>
      </div>
    )

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">
        Search: <span className="text-primary">{q}</span>
      </h1>

      {isLoading && <p>Loading…</p>}
      {isError && <div className="alert alert-error">Search failed</div>}
      {!isLoading && data?.results.length === 0 && (
        <div className="flex flex-col items-center py-16 gap-4">
          <SearchX className="h-12 w-12 text-base-content/30" />
          <p className="text-base-content/60">No skills found</p>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {data?.results.map((skill) => (
          <SkillCard key={skill.slug} skill={skill} />
        ))}
      </div>
    </div>
  )
}
