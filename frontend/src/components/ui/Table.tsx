import type { ReactNode } from 'react'

interface Column<T> {
  key: string
  header: string
  render?: (row: T) => ReactNode
  className?: string
}

interface TableProps<T> {
  columns: Column<T>[]
  data: T[]
  keyExtractor: (row: T) => string
  emptyMessage?: string
  loading?: boolean
  onRowClick?: (row: T) => void
}

export function Table<T>({
  columns,
  data,
  keyExtractor,
  emptyMessage = 'No data found',
  loading = false,
  onRowClick,
}: TableProps<T>) {
  const isEmpty = !loading && data.length === 0

  return (
    <>
      {/* Desktop table — hidden on small screens */}
      <div className="hidden overflow-x-auto rounded-2xl border border-gray-200 dark:border-surface-700 sm:block">
        <table className="min-w-full divide-y divide-gray-100 text-sm dark:divide-surface-700">
          <thead className="bg-gray-50 dark:bg-surface-800">
            <tr>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={`px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-500 ${col.className ?? ''}`}
                >
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 bg-white dark:divide-surface-700 dark:bg-surface-900">
            {loading ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-10 text-center text-sm text-gray-400 dark:text-slate-600">
                  Загрузка…
                </td>
              </tr>
            ) : isEmpty ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-10 text-center text-sm text-gray-400 dark:text-slate-600">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              data.map((row) => (
                <tr
                  key={keyExtractor(row)}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  className={`transition-colors ${
                    onRowClick
                      ? 'cursor-pointer hover:bg-gray-50 active:bg-gray-100 dark:hover:bg-surface-800 dark:active:bg-surface-700'
                      : ''
                  }`}
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={`px-4 py-3.5 text-gray-700 dark:text-slate-300 ${col.className ?? ''}`}
                    >
                      {col.render
                        ? col.render(row)
                        : String((row as Record<string, unknown>)[col.key] ?? '—')}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Mobile card list — shown only on xs screens */}
      <div className="flex flex-col gap-2 sm:hidden">
        {loading ? (
          <div className="rounded-2xl border border-gray-200 px-4 py-8 text-center text-sm text-gray-400 dark:border-surface-700 dark:text-slate-600">
            Загрузка…
          </div>
        ) : isEmpty ? (
          <div className="rounded-2xl border border-gray-200 px-4 py-8 text-center text-sm text-gray-400 dark:border-surface-700 dark:text-slate-600">
            {emptyMessage}
          </div>
        ) : (
          data.map((row) => (
            <div
              key={keyExtractor(row)}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
              className={[
                'rounded-2xl border border-gray-200 bg-white px-4 py-3',
                'dark:border-surface-700 dark:bg-surface-900',
                onRowClick ? 'cursor-pointer active:bg-gray-50 dark:active:bg-surface-800' : '',
              ].join(' ')}
            >
              {columns.map((col) => (
                <div key={col.key} className="flex items-start justify-between gap-2 py-1.5 text-sm first:pt-0 last:pb-0">
                  <span className="shrink-0 text-xs font-medium uppercase tracking-wider text-gray-400 dark:text-slate-600 pt-0.5">
                    {col.header}
                  </span>
                  <span className="text-right text-gray-700 dark:text-slate-300">
                    {col.render
                      ? col.render(row)
                      : String((row as Record<string, unknown>)[col.key] ?? '—')}
                  </span>
                </div>
              ))}
            </div>
          ))
        )}
      </div>
    </>
  )
}
