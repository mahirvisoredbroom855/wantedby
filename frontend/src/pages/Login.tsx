import { useAuth0 } from '@auth0/auth0-react';

export function Login() {
  const { loginWithRedirect } = useAuth0();

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-brand">
          <span className="brand-dot" />
          <h1>WantedBy</h1>
        </div>
        <p className="login-tagline">
          Real-time product pain intelligence.<br />
          Surface gaps before your competitors do.
        </p>
        <button
          className="btn-primary btn-lg"
          onClick={() => loginWithRedirect()}
        >
          Sign in to continue
        </button>
      </div>
    </div>
  );
}
