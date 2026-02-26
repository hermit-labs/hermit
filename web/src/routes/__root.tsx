import {
  createRootRouteWithContext,
  Outlet,
} from '@tanstack/react-router'
import { useState, useEffect, useCallback } from 'react'

import '../styles.css'
import TanStackQueryProvider from '../integrations/tanstack-query/root-provider'
import { ThemeProvider } from '#/hooks/useTheme'
import {
  getToken,
  setToken as saveToken,
  clearToken as removeToken,
} from '#/api'
import { Navbar } from '#/components/Navbar'
import { SignInModal } from '#/components/SignInModal'
import { NotFoundComponent } from './-404'

import type { QueryClient } from '@tanstack/react-query'

interface MyRouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
  component: RootComponent,
  notFoundComponent: NotFoundComponent,
})

function RootComponent() {
  const [token, setTokenState] = useState<string | null>(null)
  const [loginOpen, setLoginOpen] = useState(false)

  const handleLogin = useCallback((t: string) => {
    saveToken(t)
    setTokenState(t)
    setLoginOpen(false)
  }, [])

  const handleLogout = useCallback(() => {
    removeToken()
    setTokenState(null)
    setLoginOpen(false)
  }, [])

  useEffect(() => {
    setTokenState(getToken())
    const params = new URLSearchParams(window.location.search)
    const authToken = params.get('auth_token')
    if (authToken) {
      handleLogin(authToken)
      const url = new URL(window.location.href)
      url.searchParams.delete('auth_token')
      url.searchParams.delete('auth_subject')
      url.searchParams.delete('auth_name')
      window.history.replaceState({}, '', url.pathname)
    }
    const authError = params.get('auth_error')
    if (authError) {
      const url = new URL(window.location.href)
      url.searchParams.delete('auth_error')
      window.history.replaceState({}, '', url.pathname)
    }
  }, [handleLogin])

  return (
    <ThemeProvider>
      <TanStackQueryProvider>
        <div className="min-h-screen bg-base-100">
          <Navbar
            loggedIn={!!token}
            onSignIn={() => setLoginOpen(true)}
            onSignOut={handleLogout}
          />
          <main className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
            <Outlet />
          </main>
          {loginOpen && (
            <SignInModal
              onLogin={handleLogin}
              onClose={() => setLoginOpen(false)}
            />
          )}
        </div>
      </TanStackQueryProvider>
    </ThemeProvider>
  )
}
