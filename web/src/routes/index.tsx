import { useState, useEffect, useRef, useCallback } from 'react'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useInfiniteQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { api, timeAgo } from '#/api'
import type { SkillSummary } from '#/api'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const [sort, setSort] = useState('updated')

  const {
    data,
    isLoading,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: ['skills', sort],
    queryFn: ({ pageParam }) =>
      api.listSkills({ sort, limit: 24, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
  })

  const sentinelRef = useRef<HTMLDivElement>(null)

  const handleIntersect = useCallback(
    (entries: IntersectionObserverEntry[]) => {
      if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
        fetchNextPage()
      }
    },
    [fetchNextPage, hasNextPage, isFetchingNextPage],
  )

  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const observer = new IntersectionObserver(handleIntersect, {
      rootMargin: '200px',
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [handleIntersect])

  const skills = data?.pages.flatMap((p) => p.items) ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Skills</h1>
        <select
          value={sort}
          onChange={(e) => setSort(e.target.value)}
          className="select select-bordered w-48"
        >
          <option value="updated">Updated</option>
          <option value="downloads">Downloads</option>
          <option value="stars">Stars</option>
          <option value="trending">Trending</option>
        </select>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {isLoading
          ? Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="card bg-base-200 h-40">
                <div className="card-body gap-2">
                  <div className="skeleton h-4 w-3/4" />
                  <div className="skeleton h-3 w-1/2" />
                  <div className="skeleton h-3 w-full" />
                </div>
              </div>
            ))
          : skills.map((skill) => (
              <SkillCard key={skill.slug} skill={skill} />
            ))}
      </div>

      {/* Sentinel + loading indicator */}
      <div ref={sentinelRef} className="py-4 text-center">
        {isFetchingNextPage && (
          <span className="loading loading-dots loading-sm text-base-content/40" />
        )}
        {!isLoading && !hasNextPage && skills.length > 0 && (
          <p className="text-xs text-base-content/30">No more skills</p>
        )}
      </div>
    </div>
  )
}

export function SkillCard({ skill }: { skill: SkillSummary }) {
  const navigate = useNavigate()
  return (
    <div
      onClick={() =>
        navigate({ to: '/skills/$slug', params: { slug: skill.slug } })
      }
      className="card bg-linear-to-br border border-base-300 hover:shadow-lg transition-all h-full cursor-pointer"
    >
      <div className="card-body gap-2">
        <h3 className="font-semibold">{skill.displayName}</h3>
        {skill.summary && (
          <p className="text-sm text-base-content/70 line-clamp-2">
            {skill.summary}
          </p>
        )}
        <div className="flex justify-between text-xs pt-2">
          <span className="flex gap-1 items-center">
            <Download className="h-3 w-3" /> {skill.stats.downloads}
          </span>
          <span>{timeAgo(skill.updatedAt)}</span>
        </div>
      </div>
    </div>
  )
}
