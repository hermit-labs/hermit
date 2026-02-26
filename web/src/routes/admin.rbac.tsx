import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { RBACSection } from '#/components/admin/RBACSection'

export const Route = createFileRoute('/admin/rbac')({
  component: RBACPage,
})

function RBACPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-2xl font-bold">Access Control</h1>
      </div>
      <RBACSection />
    </div>
  )
}
