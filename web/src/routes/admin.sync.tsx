import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { SyncSection } from '#/components/admin/SyncSection'

export const Route = createFileRoute('/admin/sync')({
  component: SyncPage,
})

function SyncPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/admin" className="btn btn-ghost btn-sm btn-circle">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-2xl font-bold">Sync Sources</h1>
      </div>
      <SyncSection />
    </div>
  )
}
