/**
 * Auth middleware unit tests — jsonwebtoken.verify is mocked so tests
 * don't need a live Auth0 tenant. Covers header-parsing and rejection paths.
 */
import { Request, Response, NextFunction } from 'express';

// Mock jwks-rsa so getSigningKey never makes a network call.
jest.mock('jwks-rsa', () =>
  jest.fn().mockReturnValue({
    getSigningKey: (_kid: string, cb: (e: Error | null, k?: { getPublicKey: () => string }) => void) =>
      cb(null, { getPublicKey: () => 'pubkey' }),
  }),
);

// Control jwt.verify return value per test.
const mockJwtVerify = jest.fn();
jest.mock('jsonwebtoken', () => ({
  ...jest.requireActual('jsonwebtoken'),
  verify: (token: string, getKey: unknown, opts: unknown, cb: (e: Error | null, d?: unknown) => void) =>
    mockJwtVerify(token, cb),
}));

import { optionalAuth, requireAuth, AuthRequest } from '../auth/middleware';

function makeReq(authHeader?: string): AuthRequest {
  return { headers: { authorization: authHeader } } as unknown as AuthRequest;
}

function makeRes(): Response {
  const res: Partial<Response> = {};
  res.status = jest.fn().mockReturnValue(res);
  res.json = jest.fn().mockReturnValue(res);
  return res as Response;
}

describe('optionalAuth', () => {
  it('calls next() with no Authorization header', async () => {
    const next = jest.fn() as NextFunction;
    await optionalAuth(makeReq(), makeRes(), next);
    expect(next).toHaveBeenCalled();
  });

  it('attaches user when token is valid', async () => {
    const user = { sub: 'auth0|abc', email: 'test@example.com' };
    mockJwtVerify.mockImplementationOnce((_t: string, cb: (e: null, d: typeof user) => void) => cb(null, user));
    const req = makeReq('Bearer valid.token');
    const next = jest.fn() as NextFunction;
    await optionalAuth(req, makeRes(), next);
    expect(req.user).toEqual(user);
    expect(next).toHaveBeenCalled();
  });

  it('calls next() even when token is invalid (optional)', async () => {
    mockJwtVerify.mockImplementationOnce((_t: string, cb: (e: Error) => void) => cb(new Error('expired')));
    const req = makeReq('Bearer bad.token');
    const next = jest.fn() as NextFunction;
    await optionalAuth(req, makeRes(), next);
    expect(req.user).toBeUndefined();
    expect(next).toHaveBeenCalled();
  });
});

describe('requireAuth', () => {
  it('returns 401 when no Authorization header', async () => {
    const res = makeRes();
    await requireAuth(makeReq(), res, jest.fn() as NextFunction);
    expect(res.status).toHaveBeenCalledWith(401);
  });

  it('returns 401 when token verification fails', async () => {
    mockJwtVerify.mockImplementationOnce((_t: string, cb: (e: Error) => void) => cb(new Error('invalid')));
    const res = makeRes();
    await requireAuth(makeReq('Bearer bad'), res, jest.fn() as NextFunction);
    expect(res.status).toHaveBeenCalledWith(401);
  });

  it('calls next() with valid token', async () => {
    const user = { sub: 'auth0|xyz' };
    mockJwtVerify.mockImplementationOnce((_t: string, cb: (e: null, d: typeof user) => void) => cb(null, user));
    const req = makeReq('Bearer good');
    const next = jest.fn() as NextFunction;
    const res = makeRes();
    await requireAuth(req, res, next);
    expect(req.user).toEqual(user);
    expect(next).toHaveBeenCalled();
  });
});
