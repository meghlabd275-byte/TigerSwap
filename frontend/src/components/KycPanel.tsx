'use client';

/**
 * Identity-verification panel for the signed-in account.
 *
 * The panel reads the caller's own record and, when unverified or rejected,
 * offers the submission form. The document number is sent once and never
 * returned by the API; the server stores only its digest and last four digits,
 * so the panel can never echo it back.
 */

import { useCallback, useEffect, useState } from 'react';
import { CheckCircle2, Clock, Loader2, ShieldAlert, ShieldCheck } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { KycProfile } from '@/lib/types';
import { ValidationError } from '@/lib/validation';

function errorMessage(error: unknown): string {
  if (error instanceof ValidationError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unexpected error';
}

const STATUS_LABEL: Record<KycProfile['status'], string> = {
  unverified: 'Not verified',
  pending: 'Under review',
  approved: 'Verified',
  rejected: 'Rejected',
};

export function KycPanel() {
  const [profile, setProfile] = useState<KycProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [fullName, setFullName] = useState('');
  const [country, setCountry] = useState('');
  const [documentType, setDocumentType] = useState('passport');
  const [documentNumber, setDocumentNumber] = useState('');
  const [dateOfBirth, setDateOfBirth] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setProfile(await endpoints.getKyc());
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const submit = async () => {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await endpoints.submitKyc({
        full_name: fullName.trim(),
        country: country.trim().toUpperCase(),
        document_type: documentType,
        document_number: documentNumber.trim(),
        date_of_birth: dateOfBirth || undefined,
      });
      // The document number is not retained in component state after submission.
      setDocumentNumber('');
      setNotice('Submitted for review');
      await load();
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      setBusy(false);
    }
  };

  const status = profile?.status ?? 'unverified';
  const canSubmit = status === 'unverified' || status === 'rejected';

  return (
    <section className="bg-white/5 border border-white/10 rounded-2xl p-6 space-y-4">
      <div className="flex items-center space-x-2">
        {status === 'approved' ? (
          <ShieldCheck className="w-5 h-5 text-green-400" />
        ) : status === 'pending' ? (
          <Clock className="w-5 h-5 text-yellow-400" />
        ) : (
          <ShieldAlert className="w-5 h-5 text-gray-400" />
        )}
        <h2 className="text-lg font-semibold text-white">Identity verification</h2>
      </div>

      {loading && <p className="text-gray-400 text-sm">Loading status…</p>}

      {!loading && profile && (
        <p className="text-gray-300 text-sm">
          Status: <span className="text-white font-medium">{STATUS_LABEL[status]}</span>
          {profile.country ? ` · ${profile.country}` : ''}
        </p>
      )}

      {status === 'rejected' && profile?.rejection_reason && (
        <p className="text-red-400 text-sm">Reason: {profile.rejection_reason}</p>
      )}

      {canSubmit && !loading && (
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label htmlFor="kyc-name" className="text-gray-400 text-sm mb-2 block">
                Full name
              </label>
              <input
                id="kyc-name"
                type="text"
                value={fullName}
                onChange={(event) => setFullName(event.target.value)}
                required
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"
              />
            </div>
            <div>
              <label htmlFor="kyc-country" className="text-gray-400 text-sm mb-2 block">
                Country (ISO 3166-1 alpha-2)
              </label>
              <input
                id="kyc-country"
                type="text"
                maxLength={2}
                value={country}
                onChange={(event) => setCountry(event.target.value.toUpperCase())}
                required
                placeholder="US"
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500"
              />
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label htmlFor="kyc-doctype" className="text-gray-400 text-sm mb-2 block">
                Document type
              </label>
              <select
                id="kyc-doctype"
                value={documentType}
                onChange={(event) => setDocumentType(event.target.value)}
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"
              >
                <option value="passport">Passport</option>
                <option value="national_id">National ID</option>
                <option value="drivers_license">Driver&apos;s licence</option>
              </select>
            </div>
            <div>
              <label htmlFor="kyc-docno" className="text-gray-400 text-sm mb-2 block">
                Document number
              </label>
              <input
                id="kyc-docno"
                type="text"
                value={documentNumber}
                onChange={(event) => setDocumentNumber(event.target.value)}
                required
                autoComplete="off"
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"
              />
            </div>
          </div>
          <div>
            <label htmlFor="kyc-dob" className="text-gray-400 text-sm mb-2 block">
              Date of birth (optional)
            </label>
            <input
              id="kyc-dob"
              type="date"
              value={dateOfBirth}
              onChange={(event) => setDateOfBirth(event.target.value)}
              className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"
            />
          </div>
          <button
            type="submit"
            disabled={busy || !fullName.trim() || !country.trim() || !documentNumber.trim()}
            className="px-4 py-2 bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 rounded-xl text-white font-semibold"
          >
            {busy ? 'Submitting…' : 'Submit for review'}
          </button>
        </form>
      )}

      {status === 'approved' && (
        <p className="flex items-center space-x-2 text-green-400 text-sm">
          <CheckCircle2 className="w-4 h-4" />
          <span>Your identity is verified. Trading and withdrawals are enabled.</span>
        </p>
      )}

      {notice && <p className="text-green-400 text-sm">{notice}</p>}
      {error && <p className="text-red-400 text-sm">{error}</p>}
      {busy && (
        <p className="flex items-center space-x-2 text-gray-400 text-sm">
          <Loader2 className="w-4 h-4 animate-spin" />
          <span>Working…</span>
        </p>
      )}
    </section>
  );
}
