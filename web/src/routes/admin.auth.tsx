import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { AuthConfigSection } from '#/components/admin/AuthConfigSection'

export const Route = createFileRoute('/admin/auth')({
  component: AuthConfigPage,
})

function AuthConfigPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-2xl font-bold">Authentication Providers</h1>
      </div>
      <AuthConfigSection />
    </div>
  )
}
