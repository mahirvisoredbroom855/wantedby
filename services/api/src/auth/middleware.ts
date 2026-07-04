import { Request, Response, NextFunction } from 'express';
import jwt, { JwtHeader, SigningKeyCallback } from 'jsonwebtoken';
import jwksClient from 'jwks-rsa';
import { config } from '../config';

const client = jwksClient({
  jwksUri: `https://${config.auth0Domain}/.well-known/jwks.json`,
  cache: true,
  rateLimit: true,
});

function getKey(header: JwtHeader, callback: SigningKeyCallback): void {
  client.getSigningKey(header.kid, (err, key) => {
    if (err) return callback(err);
    callback(null, key?.getPublicKey());
  });
}

export interface AuthUser {
  sub: string;
  email?: string;
  name?: string;
}

export interface AuthRequest extends Request {
  user?: AuthUser;
}

export function verifyToken(token: string): Promise<AuthUser> {
  return new Promise((resolve, reject) => {
    jwt.verify(
      token,
      getKey,
      {
        audience: config.auth0Audience,
        issuer: `https://${config.auth0Domain}/`,
        algorithms: ['RS256'],
      },
      (err, decoded) => {
        if (err) return reject(err);
        resolve(decoded as AuthUser);
      },
    );
  });
}

// Express middleware — attaches req.user if a valid Bearer token is present.
// Does NOT reject missing tokens so public queries still work.
export async function optionalAuth(req: AuthRequest, _res: Response, next: NextFunction): Promise<void> {
  const authHeader = req.headers.authorization;
  if (!authHeader?.startsWith('Bearer ')) return next();

  try {
    req.user = await verifyToken(authHeader.slice(7));
  } catch {
    // Invalid token — treat as unauthenticated, let resolvers guard sensitive fields.
  }
  next();
}

// Stricter version — rejects with 401 if no valid token.
export async function requireAuth(req: AuthRequest, res: Response, next: NextFunction): Promise<void> {
  const authHeader = req.headers.authorization;
  if (!authHeader?.startsWith('Bearer ')) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  try {
    req.user = await verifyToken(authHeader.slice(7));
    next();
  } catch {
    res.status(401).json({ error: 'Invalid token' });
  }
}
