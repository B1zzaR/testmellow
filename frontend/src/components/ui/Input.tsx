import { useState } from 'react'
import type { InputHTMLAttributes, ReactNode } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  hint?: string
  leftAddon?: ReactNode
  rightAddon?: ReactNode
}

const inputBase = [
  'block w-full rounded-xl border px-3.5 py-3 text-base',
  'transition-[border-color,box-shadow] duration-150',
  'bg-white text-gray-900 placeholder:text-gray-400',
  'focus:outline-none focus:ring-2 focus:ring-primary-500/70 focus:border-primary-500',
  'disabled:cursor-not-allowed disabled:opacity-50',
  'dark:bg-surface-800 dark:text-slate-100 dark:placeholder:text-slate-500',
  'dark:focus:ring-primary-500/40 dark:focus:border-primary-500',
  'dark:disabled:bg-surface-900 dark:disabled:text-slate-600',
  // Fix autofill background bleed in WebKit
  '[&:-webkit-autofill]:!bg-surface-800 [&:-webkit-autofill]:!text-slate-100',
].join(' ')

export function Input({ label, error, hint, leftAddon, rightAddon, className = '', type, ...props }: InputProps) {
  const [showPassword, setShowPassword] = useState(false)
  const isPassword = type === 'password'
  const resolvedType = isPassword ? (showPassword ? 'text' : 'password') : type

  return (
    <div className="w-full">
      {label && (
        <label className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-slate-500">
          {label}
        </label>
      )}
      <div className="relative flex items-center">
        {leftAddon && (
          <div className="pointer-events-none absolute left-3 text-gray-400 dark:text-slate-500">
            {leftAddon}
          </div>
        )}
        <input
          type={resolvedType}
          className={[
            inputBase,
            error
              ? 'border-red-400 focus:ring-red-500/50 focus:border-red-500 dark:border-red-700'
              : 'border-gray-300 dark:border-surface-600',
            leftAddon ? 'pl-9' : '',
            isPassword || rightAddon ? 'pr-10' : '',
            className,
          ].join(' ')}
          {...props}
        />
        {/* Password toggle */}
        {isPassword && (
          <button
            type="button"
            tabIndex={-1}
            onClick={() => setShowPassword((v) => !v)}
            className="absolute right-3 flex h-6 w-6 items-center justify-center text-gray-400 transition-colors hover:text-gray-600 dark:text-slate-500 dark:hover:text-slate-300"
            aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
          >
            {showPassword ? (
              <svg viewBox="0 0 20 20" fill="none" className="h-4.5 w-4.5" stroke="currentColor" strokeWidth="1.5">
                <path d="M9.99 5C7.19 5 4.77 6.61 3 9c1.77 2.39 4.19 4 6.99 4s5.22-1.61 6.99-4C15.21 6.61 12.79 5 9.99 5z" />
                <circle cx="10" cy="9" r="2" />
              </svg>
            ) : (
              <svg viewBox="0 0 20 20" fill="none" className="h-4.5 w-4.5" stroke="currentColor" strokeWidth="1.5">
                <path d="M2 2l16 16M7.36 7.39A3.51 3.51 0 0010 13.5a3.51 3.51 0 002.64-1.12M3 9c1.77-2.39 4.19-4 6.99-4 1.05 0 2.05.22 2.96.62M17 9c-.78 1.05-1.77 1.92-2.91 2.52" strokeLinecap="round" />
              </svg>
            )}
          </button>
        )}
        {/* Custom right addon */}
        {!isPassword && rightAddon && (
          <div className="pointer-events-none absolute right-3 text-gray-400 dark:text-slate-500">
            {rightAddon}
          </div>
        )}
      </div>
      {error && (
        <p className="mt-1.5 flex items-center gap-1 text-sm text-red-500">
          <svg viewBox="0 0 16 16" className="h-3.5 w-3.5 shrink-0" fill="currentColor">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm-.75 4a.75.75 0 011.5 0v3.5a.75.75 0 01-1.5 0V5zm.75 6.5a.875.875 0 110-1.75.875.875 0 010 1.75z" />
          </svg>
          {error}
        </p>
      )}
      {hint && !error && <p className="mt-1.5 text-sm text-gray-400 dark:text-slate-600">{hint}</p>}
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
        <label className="mb-2 block text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-slate-500">
          {label}
        </label>
      )}
      <textarea
        className={[
          inputBase,
          error
            ? 'border-red-400 focus:ring-red-500/50 dark:border-red-700'
            : 'border-gray-300 dark:border-surface-600',
          className,
        ].join(' ')}
        {...props}
      />
      {error && <p className="mt-1.5 text-sm text-red-500">{error}</p>}
    </div>
  )
}
