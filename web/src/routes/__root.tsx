import {
  createRootRouteWithContext,
  Outlet,
  useNavigate,
} from '@tanstack/react-router'
import { useState, useEffect, useCallback } from 'react'

import '../styles.css'
import TanStackQueryProvider from '../integrations/tanstack-query/root-provider'
import { ThemeProvider } from '#/hooks/useTheme'
import {
  getToken,
  setToken as saveToken,
  clearToken as removeToken,
  onUnauthorized,
  onTokenChange,
} from '#/api'
import { Navbar } from '#/components/Navbar'
import { NotFoundComponent } from './-404'

import type { QueryClient } from '@tanstack/react-query'

interface MyRouterContext {
  queryClient: QueryClient
}

function getCurrentPath(): string {
  if (typeof window === 'undefined') return '/'
  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
  component: RootComponent,
  notFoundComponent: NotFoundComponent,
})

function RootComponent() {
  const navigate = useNavigate()
  const [token, setTokenState] = useState<string | null>(() => getToken())

  const handleLogout = useCallback(() => {
    removeToken()
    navigate({ to: '/login', replace: true })
  }, [navigate])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const authToken = params.get('auth_token')
    if (authToken) {
      saveToken(authToken)
      const url = new URL(window.location.href)
      url.searchParams.delete('auth_token')
      url.searchParams.delete('auth_subject')
      url.searchParams.delete('auth_name')
      window.history.replaceState({}, '', url.pathname)
      navigate({ to: '/' })
    }
    const authError = params.get('auth_error')
    if (authError) {
      const url = new URL(window.location.href)
      url.searchParams.delete('auth_error')
      window.history.replaceState({}, '', url.pathname)
    }
  }, [navigate])

  useEffect(() => onTokenChange(setTokenState), [])
  useEffect(
    () =>
      onUnauthorized(() => {
        navigate({
          to: '/login',
          search: { redirect: getCurrentPath() },
          replace: true,
        })
      }),
    [navigate],
  )

  return (
    <ThemeProvider>
      <TanStackQueryProvider>
        <div className="min-h-screen bg-base-100">
          <Navbar loggedIn={!!token} onSignOut={handleLogout} />
          <main className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
            <Outlet />
          </main>
        </div>
      </TanStackQueryProvider>
    </ThemeProvider>
  )
}
