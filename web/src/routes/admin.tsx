import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Shield } from 'lucide-react'
import { api } from '#/api'

export const Route = createFileRoute('/admin')({
  component: AdminLayout,
})

function AdminLayout() {
  const { data: user } = useQuery({
    queryKey: ['user'],
    queryFn: () => api.whoami(),
  })

  if (!user) {
    return (
      <div className="text-center py-16">
        <div className="alert alert-error max-w-md mx-auto">
          <Shield className="h-4 w-4" />
          <div>
            <h3 className="font-semibold">Admin access required</h3>
            <p className="text-sm">
              You don't have permission to access this page
            </p>
          </div>
        </div>
      </div>
    )
  }

  return <Outlet />
}
