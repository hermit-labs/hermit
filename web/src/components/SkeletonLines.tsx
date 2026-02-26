const WIDTHS = ['w-3/4', 'w-full', 'w-5/6', 'w-2/3', 'w-full', 'w-4/5', 'w-full', 'w-1/2']

export function SkeletonLines({
  lines = 6,
  height = 'h-4',
}: {
  lines?: number
  height?: string
}) {
  return (
    <div className="space-y-3 animate-pulse">
      {Array.from({ length: lines }, (_, i) => (
        <div
          key={i}
          className={`${height} bg-base-300 rounded ${WIDTHS[i % WIDTHS.length]}`}
        />
      ))}
    </div>
  )
}
