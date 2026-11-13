'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Loader2 } from 'lucide-react';
import { verifyEmail } from '@/lib/auth';

/**
 * Completes email verification with the token from the emailed link. The token
 * is redeemed once; a replay or an expired link reports an explicit error.
 */
export default function VerifyEmailPage() {
  const [state, setState] = useState<'working' | 'done' | 'error'>('working');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');
    if (!token) {
      setState('error');
      setError('This link is missing its verification token.');
      return;
    }
    let cancelled = false;
    void verifyEmail(token)
      .then(() => {
        if (!cancelled) {
          setState('done');
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState('error');
          setError(err instanceof Error ? err.message : 'Could not verify the email');
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="min-h-screen flex items-center justify-center bg-tiger-dark p-4">
      <div className="w-full max-w-md bg-tiger-dark border border-white/10 rounded-2xl p-6 shadow-2xl text-center">
        {state === 'working' && (
          <div className="flex items-center justify-center space-x-2 text-gray-300">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>Verifying your email…</span>
          </div>
        )}
        {state === 'done' && (
          <div className="space-y-4">
            <h1 className="text-xl font-display font-bold text-white">Email verified</h1>
            <p className="text-green-400 text-sm">Your email address is now confirmed.</p>
            <Link href="/" className="text-tiger-orange hover:underline text-sm">
              Back to TigerSwap
            </Link>
          </div>
        )}
        {state === 'error' && (
          <div className="space-y-4">
            <h1 className="text-xl font-display font-bold text-white">Verification failed</h1>
            <p className="text-red-400 text-sm">{error}</p>
            <Link href="/" className="text-tiger-orange hover:underline text-sm">
              Back to TigerSwap
            </Link>
          </div>
        )}
      </div>
    </main>
  );
}
