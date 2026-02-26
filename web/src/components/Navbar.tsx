import { Link } from '@tanstack/react-router'
import { Shield, LogIn, LogOut, User } from 'lucide-react'

export function Navbar({
  loggedIn,
  onSignIn,
  onSignOut,
}: {
  loggedIn: boolean
  onSignIn: () => void
  onSignOut: () => void
}) {
  return (
    <header className="navbar bg-base-200 border-b border-base-300 sticky top-0 z-40 px-4">
      <div className="navbar-start">
        <Link to="/" className="flex items-center gap-2 font-bold text-lg">
          <img src="/hermit-crab.svg" alt="hermit" className="h-7 w-7" />
          <span>hermit</span>
        </Link>
      </div>
      <div className="navbar-end gap-2">
        <div className="flex gap-2">
          <Link to="/publish" className="btn btn-ghost btn-sm">
            Publish
          </Link>
          {loggedIn && (
            <Link to="/admin" className="btn btn-ghost btn-sm">
              <Shield className="h-4 w-4" />
              Admin
            </Link>
          )}
          {loggedIn ? (
            <div className="dropdown dropdown-end">
              <div tabIndex={0} role="button" className="btn btn-ghost btn-sm btn-circle">
                <User className="h-5 w-5" />
              </div>
              <ul tabIndex={0} className="dropdown-content menu bg-base-200 rounded-box z-50 w-40 p-2 shadow-lg mt-2">
                <li>
                  <button onClick={onSignOut} className="text-error">
                    <LogOut className="h-4 w-4" />
                    Sign out
                  </button>
                </li>
              </ul>
            </div>
          ) : (
            <button onClick={onSignIn} className="btn btn-primary btn-sm">
              <LogIn className="h-4 w-4" />
              Sign in
            </button>
          )}
        </div>
      </div>
    </header>
  )
}
