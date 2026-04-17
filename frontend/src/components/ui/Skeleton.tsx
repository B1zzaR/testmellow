interface SkeletonProps {
  className?: string
}

export function Skeleton({ className = 'h-4 w-full' }: SkeletonProps) {
  return (
    <div
      className={`rounded-xl bg-gray-200 dark:bg-surface-700 animate-shimmer ${className}`}
      aria-hidden="true"
    />
  )
}

export function StatCardSkeleton() {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 dark:border-surface-700 dark:bg-surface-900 sm:p-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-3">
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-9 w-24" />
          <Skeleton className="h-3.5 w-32" />
        </div>
        <Skeleton className="h-11 w-11 shrink-0 rounded-xl sm:h-14 sm:w-14" />
      </div>
    </div>
  )
}

export function CardSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white dark:border-surface-700 dark:bg-surface-900">
      <div className="border-b border-gray-100 px-4 py-3.5 dark:border-surface-700 sm:px-6 sm:py-4">
        <Skeleton className="h-5 w-36" />
      </div>
      <div className="space-y-3 px-4 py-4 sm:px-6 sm:py-5">
        {Array.from({ length: lines }).map((_, i) => (
          <Skeleton key={i} className={`h-4 ${i === lines - 1 ? 'w-2/3' : 'w-full'}`} />
        ))}
      </div>
    </div>
  )
}
