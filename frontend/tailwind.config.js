/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Primary accent — cyber green
        primary: {
          50:  '#f0fdf4',
          100: '#dcfce7',
          200: '#bbf7d0',
          300: '#86efac',
          400: '#4ade80',
          500: '#22c55e',
          600: '#16a34a',
          700: '#15803d',
          800: '#166534',
          900: '#14532d',
          950: '#052e16',
        },
        // Dark surface palette
        surface: {
          950: '#07070d',
          900: '#0d0d1a',
          800: '#131320',
          700: '#1a1a2c',
          600: '#242438',
          500: '#32324a',
        },
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'Noto Sans', 'Ubuntu', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      boxShadow: {
        'glow-sm': '0 0 10px rgba(34, 197, 94, 0.15)',
        'glow-md': '0 0 18px rgba(34, 197, 94, 0.22)',
        'glow-lg': '0 0 30px rgba(34, 197, 94, 0.28)',
        'card':    '0 1px 3px rgba(0,0,0,0.5), 0 1px 2px rgba(0,0,0,0.4)',
        'card-lg': '0 4px 12px rgba(0,0,0,0.6)',
      },
    },
  },
  plugins: [],
}
