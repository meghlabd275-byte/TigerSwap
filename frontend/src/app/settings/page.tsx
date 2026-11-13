'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Loader2, ShieldCheck, ShieldOff } from 'lucide-react';
import { useSession } from '@/hooks/useSession';
import {
  beginTOTPEnrollment,
  confirmTOTPEnrollment,
  disableTOTP,
  fetchTOTPStatus,
  requestEmailVerification,
} from '@/lib/auth';

/**
 * Account settings: two-factor enrolment and email verification.
 *
 * Enrolment is two steps. The panel shows the secret and the otpauth URI, then
 * requires a code from the authenticator before the factor is active, so a user
 * cannot lock themselves out with a mis-scanned secret. Disabling requires a
 * current code for the same reason.
 */
export default function SettingsPage() {
  const { user } = useSession();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [otpauthUri, setOtpauthUri] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [disableCode, setDisableCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!user) {
      return;
    }
    let cancelled = false;
    void fetchTOTPStatus()
      .then((value) => {
        if (!cancelled) {
          setEnabled(value);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Could not load the two-factor status');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [user]);

  if (!user) {
    return (
      <main className="min-h-screen flex items-center justify-center bg-tiger-dark p-4">
        <div className="text-center space-y-4">
          <p className="text-gray-300">Sign in to manage your account security.</p>
          <Link href="/" className="text-tiger-orange hover:underline text-sm">
            Back to TigerSwap
          </Link>
        </div>
      </main>
    );
  }

  const startEnrollment = async () => {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const enrollment = await beginTOTPEnrollment();
      setSecret(enrollment.secret);
      setOtpauthUri(enrollment.otpauthUri);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start enrolment');
    } finally {
      setBusy(false);
    }
  };

  const confirmEnrollment = async () => {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await confirmTOTPEnrollment(code);
      setEnabled(true);
      setSecret(null);
      setOtpauthUri(null);
      setCode('');
      setNotice('Two-factor authentication is now active.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The code was rejected');
    } finally {
      setBusy(false);
    }
  };

  const turnOff = async () => {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await disableTOTP(disableCode);
      setEnabled(false);
      setDisableCode('');
      setNotice('Two-factor authentication is now off.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The code was rejected');
    } finally {
      setBusy(false);
    }
  };

  const resendVerification = async () => {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await requestEmailVerification();
      setNotice('A verification link has been sent to your email address.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not send the verification link');
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="min-h-screen bg-tiger-dark p-6">
      <div className="max-w-2xl mx-auto space-y-6">
        <h1 className="text-2xl font-display font-bold text-white">Account settings</h1>

        <section className="bg-white/5 border border-white/10 rounded-2xl p-6 space-y-4">
          <div className="flex items-center space-x-2">
            {enabled ? (
              <ShieldCheck className="w-5 h-5 text-green-400" />
            ) : (
              <ShieldOff className="w-5 h-5 text-gray-400" />
            )}
            <h2 className="text-lg font-semibold text-white">Two-factor authentication</h2>
          </div>

          {enabled === null && <p className="text-gray-400 text-sm">Loading status…</p>}

          {enabled === false && !secret && (
            <>
              <p className="text-gray-400 text-sm">
                Add a second factor with any authenticator app. Once active, a code is required
                to sign in and to withdraw funds.
              </p>
              <button
                type="button"
                disabled={busy}
                onClick={() => void startEnrollment()}
                className="px-4 py-2 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold disabled:opacity-50"
              >
                Enable two-factor
              </button>
            </>
          )}

          {enabled === false && secret && (
            <div className="space-y-4">
              <p className="text-gray-400 text-sm">
                Add this secret to your authenticator, then enter the code it shows.
              </p>
              <code className="block break-all bg-black/40 rounded-xl p-3 text-tiger-orange text-sm">
                {secret}
              </code>
              {otpauthUri && (
                <p className="text-xs text-gray-500 break-all">otpauth URI: {otpauthUri}</p>
              )}
              <div>
                <label className="text-gray-400 text-sm mb-1 block">Authenticator code</label>
                <input
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
                  className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white tracking-[0.4em] focus:outline-none focus:border-tiger-orange"
                  placeholder="000000"
                />
              </div>
              <button
                type="button"
                disabled={busy}
                onClick={() => void confirmEnrollment()}
                className="px-4 py-2 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold disabled:opacity-50"
              >
                Confirm and activate
              </button>
            </div>
          )}

          {enabled === true && (
            <div className="space-y-4">
              <p className="text-green-400 text-sm">Two-factor authentication is active.</p>
              <div>
                <label className="text-gray-400 text-sm mb-1 block">
                  Enter a code to turn it off
                </label>
                <input
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  value={disableCode}
                  onChange={(e) => setDisableCode(e.target.value.replace(/\D/g, ''))}
                  className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white tracking-[0.4em] focus:outline-none focus:border-tiger-orange"
                  placeholder="000000"
                />
              </div>
              <button
                type="button"
                disabled={busy}
                onClick={() => void turnOff()}
                className="px-4 py-2 bg-red-500/20 border border-red-500/30 rounded-xl text-red-300 font-semibold disabled:opacity-50"
              >
                Disable two-factor
              </button>
            </div>
          )}
        </section>

        <section className="bg-white/5 border border-white/10 rounded-2xl p-6 space-y-4">
          <h2 className="text-lg font-semibold text-white">Email verification</h2>
          <p className="text-gray-400 text-sm">
            Confirm your email address so account notices reach you.
          </p>
          <button
            type="button"
            disabled={busy}
            onClick={() => void resendVerification()}
            className="px-4 py-2 bg-white/5 border border-white/10 rounded-xl text-white font-semibold hover:bg-white/10 disabled:opacity-50"
          >
            Send verification link
          </button>
        </section>

        {notice && <p className="text-green-400 text-sm">{notice}</p>}
        {error && <p className="text-red-400 text-sm">{error}</p>}
        {busy && (
          <p className="flex items-center space-x-2 text-gray-400 text-sm">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>Working…</span>
          </p>
        )}
      </div>
    </main>
  );
}
