'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Loader2 } from 'lucide-react';
import { resetPassword, validatePassword } from '@/lib/auth';

/**
 * Completes a password reset. The token comes from the email link's query
 * string. A successful reset invalidates every existing session on the server.
 */
export default function ResetPasswordPage() {
  const [token, setToken] = useState<string | null>(null);
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    setToken(params.get('token'));
  }, []);

  const submit = async () => {
    setError(null);
    if (!token) {
      setError('This link is missing its reset token.');
      return;
    }
    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }
    if (password !== confirm) {
      setError('The two passwords do not match.');
      return;
    }
    setBusy(true);
    try {
      await resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not reset the password');
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="min-h-screen flex items-center justify-center bg-tiger-dark p-4">
      <div className="w-full max-w-md bg-tiger-dark border border-white/10 rounded-2xl p-6 shadow-2xl">
        <h1 className="text-xl font-display font-bold text-white">Set a new password</h1>

        {done ? (
          <div className="mt-4 space-y-4">
            <p className="text-green-400 text-sm">
              Your password has been reset. All other sessions have been signed out.
            </p>
            <Link href="/" className="text-tiger-orange hover:underline text-sm">
              Back to TigerSwap
            </Link>
          </div>
        ) : (
          <form
            className="mt-4 space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void submit();
            }}
          >
            <div>
              <label className="text-gray-400 text-sm mb-1 block">New password</label>
              <input
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              />
              <p className="text-xs text-gray-500 mt-1">
                At least 12 characters, with upper and lower case and a digit.
              </p>
            </div>
            <div>
              <label className="text-gray-400 text-sm mb-1 block">Confirm password</label>
              <input
                type="password"
                autoComplete="new-password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              />
            </div>
            {error && <p className="text-red-400 text-sm">{error}</p>}
            <button
              type="submit"
              disabled={busy}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {busy && <Loader2 className="w-4 h-4 animate-spin" />}
              <span>Reset password</span>
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
