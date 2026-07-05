import { useAuth0 } from '@auth0/auth0-react';
import { LogOut, User } from 'lucide-react';

export function Navbar() {
  const { user, logout, isAuthenticated } = useAuth0();

  return (
    <nav className="navbar">
      <div className="navbar-brand">
        <span className="brand-dot" />
        WantedBy
      </div>
      {isAuthenticated && (
        <div className="navbar-right">
          <div className="navbar-user">
            <User size={14} />
            <span>{user?.name ?? user?.email}</span>
          </div>
          <button
            className="logout-btn"
            onClick={() => logout({ logoutParams: { returnTo: window.location.origin } })}
            aria-label="Log out"
          >
            <LogOut size={14} />
          </button>
        </div>
      )}
    </nav>
  );
}
