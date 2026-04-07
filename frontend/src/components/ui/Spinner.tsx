export function Spinner({ className = '' }: { className?: string }) {
  return (
    <div
      className={`inline-block animate-spin rounded-full border-2 border-surface-600 border-t-primary-500 ${
        className || 'h-6 w-6'
      }`}
      role="status"
      aria-label="Loading"
    />
  )
}

export function PageSpinner() {
  return (
    <div className="flex h-64 items-center justify-center">
      <Spinner className="h-8 w-8" />
    </div>
  )
}
