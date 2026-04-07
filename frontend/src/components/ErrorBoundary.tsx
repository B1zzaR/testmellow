import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: { componentStack: string }) {
    console.error('[ErrorBoundary]', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex min-h-svh items-center justify-center bg-gray-50 dark:bg-surface-950 p-6">
          <div className="max-w-md w-full rounded-2xl border border-red-200 dark:border-red-800/40 bg-white dark:bg-surface-900 p-8 text-center shadow-xl">
            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-red-100 dark:bg-red-500/10 text-3xl">
              ⚠️
            </div>
            <h2 className="text-lg font-bold text-gray-900 dark:text-slate-100">Что-то пошло не так</h2>
            <p className="mt-2 text-sm text-gray-500 dark:text-slate-500">
              Произошла непредвиденная ошибка. Попробуйте перезагрузить страницу.
            </p>
            {this.state.error && (
              <details className="mt-4 text-left">
                <summary className="cursor-pointer text-xs text-gray-400 dark:text-slate-600">Подробности</summary>
                <pre className="mt-2 overflow-auto rounded bg-gray-100 dark:bg-surface-800 p-3 text-xs text-gray-700 dark:text-slate-300 whitespace-pre-wrap">
                  {this.state.error.message}
                </pre>
              </details>
            )}
            <button
              onClick={() => window.location.reload()}
              className="mt-6 rounded-lg bg-primary-500 px-5 py-2 text-sm font-medium text-white hover:bg-primary-600 active:scale-95 transition-all"
            >
              Перезагрузить
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
