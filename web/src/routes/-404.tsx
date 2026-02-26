import { Link } from '@tanstack/react-router'

export function NotFoundComponent() {
  return (
    <div className="text-center py-16 space-y-4">
      <h1 className="text-4xl font-bold">404</h1>
      <p className="text-lg text-base-content/60">Page not found</p>
      <Link to="/" className="btn btn-primary">
        Back to home
      </Link>
    </div>
  )
}
