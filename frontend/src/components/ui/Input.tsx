import type { InputHTMLAttributes, ReactNode } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  hint?: string
  leftAddon?: ReactNode
}

export function Input({ label, error, hint, leftAddon, className = '', ...props }: InputProps) {
  return (
    <div className="w-full">
      {label && (
        <label className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-slate-300">{label}</label>
      )}
      <div className="relative flex items-center">
        {leftAddon && (
          <div className="pointer-events-none absolute left-3 text-gray-400 dark:text-slate-500">{leftAddon}</div>
        )}
        <input
          className={`block w-full rounded-lg border px-3 py-2.5 text-sm
            bg-white text-gray-900 placeholder:text-gray-400
            focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500
            disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400
            dark:bg-slate-900 dark:text-slate-100 dark:placeholder:text-slate-500 dark:disabled:bg-slate-800 dark:disabled:text-slate-500
            ${error ? 'border-red-400 focus:ring-red-400 dark:border-red-500' : 'border-gray-300 dark:border-slate-700'}
            ${leftAddon ? 'pl-9' : ''}
            ${className}`}
          {...props}
        />
      </div>
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
      {hint && !error && <p className="mt-1 text-xs text-gray-400 dark:text-slate-500">{hint}</p>}
    </div>
  )
}

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string
  error?: string
}

export function Textarea({ label, error, className = '', ...props }: TextareaProps) {
  return (
    <div className="w-full">
      {label && (
        <label className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-slate-300">{label}</label>
      )}
      <textarea
        className={`block w-full rounded-lg border px-3 py-2.5 text-sm
          bg-white text-gray-900 placeholder:text-gray-400
          focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500
          disabled:cursor-not-allowed disabled:bg-gray-50
          dark:bg-slate-900 dark:text-slate-100 dark:placeholder:text-slate-500 dark:disabled:bg-slate-800
          ${error ? 'border-red-400 focus:ring-red-400 dark:border-red-500' : 'border-gray-300 dark:border-slate-700'}
          ${className}`}
        {...props}
      />
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
    </div>
  )
}
