import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from '@/router'
import { useEffect } from 'react'
import { useThemeStore } from '@/store/themeStore'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
})

export default function App() {
  const mode = useThemeStore((s) => s.mode)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', mode === 'dark')
  }, [mode])

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
