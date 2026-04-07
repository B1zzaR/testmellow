import type { ButtonHTMLAttributes } from 'react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
}

const variants = {
  primary: [
    'bg-primary-500 text-white',
    'hover:bg-primary-400',
    'active:bg-primary-600 active:scale-[0.97]',
    'dark:shadow-glow-sm dark:hover:shadow-glow-md',
    'disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none disabled:scale-100',
    'focus-visible:ring-primary-500 focus-visible:ring-offset-surface-950',
  ].join(' '),
  secondary: [
    'bg-transparent text-slate-300 border border-surface-600',
    'hover:bg-surface-700 hover:border-surface-500 hover:text-slate-100',
    'active:bg-surface-600 active:scale-[0.97]',
    'dark:bg-surface-800 dark:border-surface-600',
    'disabled:opacity-40 disabled:cursor-not-allowed disabled:scale-100',
    'focus-visible:ring-surface-500',
  ].join(' '),
  danger: [
    'bg-red-600 text-white',
    'hover:bg-red-500',
    'active:bg-red-700 active:scale-[0.97]',
    'disabled:opacity-40 disabled:cursor-not-allowed disabled:scale-100',
    'focus-visible:ring-red-500',
  ].join(' '),
  ghost: [
    'bg-transparent text-slate-400',
    'hover:bg-surface-700 hover:text-slate-200',
    'active:bg-surface-600 active:scale-[0.97]',
    'disabled:opacity-40 disabled:cursor-not-allowed disabled:scale-100',
    'focus-visible:ring-surface-500',
  ].join(' '),
}

const sizes = {
  sm: 'min-h-[36px] px-4 py-2 text-sm rounded-lg',
  md: 'min-h-[44px] px-5 py-2.5 text-sm rounded-lg',
  lg: 'min-h-[48px] px-7 py-3 text-base rounded-xl',
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  className = '',
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 font-medium select-none whitespace-nowrap
        focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2
        transition-all duration-150
        ${variants[variant]} ${sizes[size]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && (
        <svg className="h-4 w-4 animate-spin shrink-0" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      )}
      {children}
    </button>
  )
}
