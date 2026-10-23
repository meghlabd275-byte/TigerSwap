/**
 * Session handling for the TigerSwap API.
 *
 * Design decisions that matter for security:
 *
 *  - The access token is held in memory only. It is never written to
 *    `localStorage` or `sessionStorage`, so a cross-site scripting bug cannot
 *    read a durable credential out of storage.
 *  - Where the backend issues an httpOnly refresh cookie, that cookie is the
 *    source of truth and `credentials: same-origin` carries it. The in-memory
 *    token is only a convenience for the current tab.
 *  - Sign-in is email and password. There is deliberately no "paste your seed
 *    phrase" path: a mnemonic must never be typed into, or transmitted by, a
 *    web page.
 */

import { ApiError, NetworkError, api, clearSessionToken, setSessionToken } from "./apiClient";
import { object, string } from "./validation";

export interface SessionUser {
  id: string;
  email: string;
  role: "user" | "admin" | "superadmin";
}

const loginResponseValidator = object<{ token?: string; user?: unknown; access_token?: string }>(
  {
    token: string,
    access_token: string,
    user: object(
      {
        id: string,
        email: string,
        role: string,
      },
      { optional: ["email"] },
    ),
  },
  { optional: ["token", "access_token", "user"] },
);

/** In-memory session. Cleared on reload, which is intentional. */
let currentUser: SessionUser | null = null;
const listeners = new Set<(user: SessionUser | null) => void>();

export function subscribe(listener: (user: SessionUser | null) => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function notify(): void {
  listeners.forEach((listener) => listener(currentUser));
}

export function getCurrentUser(): SessionUser | null {
  return currentUser;
}

export function isAuthenticated(): boolean {
  return currentUser !== null;
}

export function isAdmin(): boolean {
  return currentUser?.role === "admin" || currentUser?.role === "superadmin";
}

/** Password rules, mirrored from the server so the user gets fast feedback. */
export function validatePassword(password: string): string | null {
  if (password.length < 12) {
    return "Password must be at least 12 characters";
  }
  if (!/[a-z]/.test(password) || !/[A-Z]/.test(password)) {
    return "Password must contain both upper and lower case letters";
  }
  if (!/[0-9]/.test(password)) {
    return "Password must contain a digit";
  }
  return null;
}

export function validateEmail(email: string): string | null {
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(email)) {
    return "Enter a valid email address";
  }
  return null;
}

/**
 * Signs in and stores the resulting token in memory.
 *
 * Throws `ApiError` with the server's message on failure.
 */
export async function login(email: string, password: string): Promise<SessionUser> {
  const body = await api.post(
    "/auth/login",
    { email, password },
    { authenticated: false },
  );

  const parsed = loginResponseValidator(body);

  // Prefer an explicitly returned token; otherwise rely on the httpOnly cookie
  // the server sets, which the client never needs to read.
  const token = parsed.token ?? parsed.access_token;
  if (token) {
    setSessionToken(token);
  }

  const profile = await api.get("/user/profile");
  const user = parseProfile(profile);

  currentUser = user;
  notify();
  return user;
}

export async function register(email: string, password: string): Promise<SessionUser> {
  await api.post("/auth/register", { email, password }, { authenticated: false });
  return login(email, password);
}

export async function logout(): Promise<void> {
  try {
    await api.post("/auth/logout");
  } catch (error) {
    // A failed logout call must still clear local state.
    if (!(error instanceof ApiError) && !(error instanceof NetworkError)) {
      throw error;
    }
  } finally {
    clearSessionToken();
    currentUser = null;
    notify();
  }
}

/**
 * Restores a session from an httpOnly refresh cookie on page load.
 *
 * Returns null when there is no valid session; that is a normal outcome, not
 * an error.
 */
export async function restoreSession(): Promise<SessionUser | null> {
  try {
    const profile = await api.get("/user/profile");
    const user = parseProfile(profile);
    currentUser = user;
    notify();
    return user;
  } catch (error) {
    if (error instanceof ApiError && error.isUnauthorized) {
      currentUser = null;
      notify();
      return null;
    }
    throw error;
  }
}

function parseProfile(body: unknown): SessionUser {
  if (body === null || typeof body !== "object") {
    throw new Error("unexpected profile response");
  }

  const record = body as Record<string, unknown>;
  const id = String(record.id ?? "");
  const email = String(record.email ?? "");
  const role = String(record.role ?? "user");

  if (!id) {
    throw new Error("profile is missing an id");
  }

  const normalizedRole: SessionUser["role"] =
    role === "admin" || role === "superadmin" ? role : "user";

  return { id, email, role: normalizedRole };
}
