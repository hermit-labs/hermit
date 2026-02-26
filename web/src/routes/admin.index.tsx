import { createFileRoute, Link } from '@tanstack/react-router'
import {
  ShieldCheck,
  RefreshCw,
  Key,
  Package,
  Lock,
  Users,
} from 'lucide-react'
import { StatsSection } from '#/components/admin/StatsSection'

export const Route = createFileRoute('/admin/')({
  component: AdminIndexPage,
})

const NAV_ITEMS = [
  {
    to: '/admin/users',
    icon: <Users className="h-6 w-6" />,
    title: 'User Management',
    desc: 'Create, edit & manage user accounts',
  },
  {
    to: '/admin/auth',
    icon: <Lock className="h-6 w-6" />,
    title: 'Authentication',
    desc: 'Configure LDAP authentication',
  },
  {
    to: '/admin/rbac',
    icon: <ShieldCheck className="h-6 w-6" />,
    title: 'Access Control',
    desc: 'Manage repository roles & members',
  },
  {
    to: '/admin/sync',
    icon: <RefreshCw className="h-6 w-6" />,
    title: 'Sync Sources',
    desc: 'Proxy upstreams & sync settings',
  },
  {
    to: '/admin/tokens',
    icon: <Key className="h-6 w-6" />,
    title: 'Access Tokens',
    desc: 'Manage personal access tokens',
  },
  {
    to: '/admin/skills',
    icon: <Package className="h-6 w-6" />,
    title: 'Skill Management',
    desc: 'View, delete, or restore skills',
  },
] as const

function AdminIndexPage() {
  return (
    <div className="space-y-8">
      <div className="card bg-base-200 border">
        <div className="card-body">
          <h1 className="card-title text-2xl">Admin Dashboard</h1>
          <p className="text-base-content/60">
            System overview and management
          </p>
        </div>
      </div>

      <StatsSection />

      <div className="space-y-4">
        <h2 className="text-xl font-semibold">Management</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {NAV_ITEMS.map((item) => (
            <Link
              key={item.to}
              to={item.to}
              className="card bg-base-200 border hover:border-primary hover:shadow-lg transition-all cursor-pointer"
            >
              <div className="card-body items-center text-center py-6">
                <div className="text-primary">{item.icon}</div>
                <h3 className="card-title text-base">{item.title}</h3>
                <p className="text-sm text-base-content/60">{item.desc}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  )
}
