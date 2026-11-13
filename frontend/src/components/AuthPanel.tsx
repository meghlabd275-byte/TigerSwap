'use client';

import { useState } from 'react';
import { X, Loader2 } from 'lucide-react';
import { login, logout, register, validateEmail, validatePassword, validateUsername } from '@/lib/auth';

type Mode = 'signin' | 'register';

/**
 * Email/password sign-in and registration.
 *
 * The access token is held in memory by the auth layer and never written to
 * storage, so a cross-site scripting bug cannot read a durable credential. The
 * panel is the only place a normal user can establish a session; every
 * authenticated component gates on the same in-memory session.
 */
export function AuthPanel({ onClose }: { onClose: () => void }) {
  const [mode, setMode] = useState<Mode>('signin');
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setError(null);

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }
    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }
    if (mode === 'register') {
      const usernameError = validateUsername(username);
      if (usernameError) {
        setError(usernameError);
        return;
      }
    }

    setBusy(true);
    try {
      if (mode === 'register') {
        await register(email, password, username);
      } else {
        await login(email, password);
      }
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="w-full max-w-md bg-tiger-dark border border-white/10 rounded-2xl p-6 shadow-2xl">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-xl font-display font-bold text-white">
              {mode === 'signin' ? 'Sign in' : 'Create account'}
            </h2>
            <p className="text-sm text-gray-500 mt-1">
              {mode === 'signin'
                ? 'Use the email and password on your TigerSwap account.'
                : 'Your password is never stored in the browser.'}
            </p>
          </div>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-white" aria-label="Close">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div>
            <label className="text-gray-400 text-sm mb-1 block">Email</label>
            <input
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
            />
          </div>

          {mode === 'register' && (
            <div>
              <label className="text-gray-400 text-sm mb-1 block">Username</label>
              <input
                type="text"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              />
            </div>
          )}

          <div>
            <label className="text-gray-400 text-sm mb-1 block">Password</label>
            <input
              type="password"
              autoComplete={mode === 'signin' ? 'current-password' : 'new-password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
            />
            {mode === 'register' && (
              <p className="text-xs text-gray-500 mt-1">
                At least 12 characters, with upper and lower case and a digit.
              </p>
            )}
          </div>

          {error && <p className="text-red-400 text-sm">{error}</p>}

          <button
            type="submit"
            disabled={busy}
            className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {busy && <Loader2 className="w-4 h-4 animate-spin" />}
            <span>{mode === 'signin' ? 'Sign in' : 'Create account'}</span>
          </button>
        </form>

        <button
          onClick={() => {
            setMode(mode === 'signin' ? 'register' : 'signin');
            setError(null);
          }}
          className="w-full text-center text-sm text-gray-400 hover:text-white mt-4"
        >
          {mode === 'signin'
            ? 'No account? Create one'
            : 'Already have an account? Sign in'}
        </button>
      </div>
    </div>
  );
}

export async function signOut(): Promise<void> {
  await logout();
}
