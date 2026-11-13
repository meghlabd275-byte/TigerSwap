'use client';

/**
 * Admin panel for the compliance layer: identity review, the sanctions denylist
 * and the geographic policy.
 *
 * Every value shown comes from the API. A KYC decision and a denylist change are
 * persisted server-side and take effect on the next request, so an operator sees
 * the effect without a restart.
 */

import { useCallback, useEffect, useState } from 'react';
import { CheckCircle2, Loader2, RefreshCw, ShieldCheck, ShieldX, Trash2 } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { ComplianceSettings, KycSubmission, SanctionsEntry, SanctionsKind } from '@/lib/types';
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

const STATUS_STYLES: Record<KycSubmission['status'], string> = {
  unverified: 'bg-gray-500/20 text-gray-300',
  pending: 'bg-yellow-500/20 text-yellow-300',
  approved: 'bg-green-500/20 text-green-300',
  rejected: 'bg-red-500/20 text-red-300',
};

export function AdminCompliancePanel() {
  const [submissions, setSubmissions] = useState<KycSubmission[]>([]);
  const [entries, setEntries] = useState<SanctionsEntry[]>([]);
  const [settings, setSettings] = useState<ComplianceSettings | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const [restricted, setRestricted] = useState('');
  const [requireTrading, setRequireTrading] = useState(true);
  const [requireWithdrawal, setRequireWithdrawal] = useState(true);

  const [newKind, setNewKind] = useState<SanctionsKind>('address');
  const [newValue, setNewValue] = useState('');
  const [newNote, setNewNote] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextSubmissions, nextEntries, nextSettings] = await Promise.all([
        endpoints.getKycSubmissions(),
        endpoints.getSanctionsEntries(),
        endpoints.getComplianceSettings(),
      ]);
      setSubmissions(nextSubmissions);
      setEntries(nextEntries);
      setSettings(nextSettings);
      setRestricted(nextSettings.restricted_countries);
      setRequireTrading(nextSettings.require_kyc_for_trading);
      setRequireWithdrawal(nextSettings.require_kyc_for_withdrawal);
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = async (submission: KycSubmission, approve: boolean) => {
    setBusy(submission.user_id);
    setError(null);
    setOk(null);
    try {
      if (approve) {
        await endpoints.approveKyc(submission.user_id);
        setOk(`Approved ${submission.full_name || submission.user_id}`);
      } else {
        await endpoints.rejectKyc(submission.user_id, 'rejected by operator');
        setOk(`Rejected ${submission.full_name || submission.user_id}`);
      }
      await load();
    } catch (decisionError) {
      setError(errorMessage(decisionError));
    } finally {
      setBusy(null);
    }
  };

  const addEntry = async () => {
    if (!newValue.trim()) {
      return;
    }
    setBusy('add-entry');
    setError(null);
    setOk(null);
    try {
      await endpoints.addSanctionsEntry({
        kind: newKind,
        value: newValue.trim(),
        source: 'manual',
        note: newNote.trim(),
      });
      setNewValue('');
      setNewNote('');
      setOk('Denylist entry added');
      await load();
    } catch (addError) {
      setError(errorMessage(addError));
    } finally {
      setBusy(null);
    }
  };

  const removeEntry = async (entry: SanctionsEntry) => {
    setBusy(entry.id);
    setError(null);
    setOk(null);
    try {
      await endpoints.deleteSanctionsEntry(entry.id);
      await load();
    } catch (removeError) {
      setError(errorMessage(removeError));
    } finally {
      setBusy(null);
    }
  };

  const saveSettings = async () => {
    setBusy('settings');
    setError(null);
    setOk(null);
    try {
      const next = await endpoints.updateComplianceSettings({
        restricted_countries: restricted,
        require_kyc_for_trading: requireTrading,
        require_kyc_for_withdrawal: requireWithdrawal,
      });
      setSettings(next);
      setOk('Compliance policy saved');
    } catch (saveError) {
      setError(errorMessage(saveError));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <p className="text-gray-400 text-sm">
          Identity review, sanctions screening and the geographic policy.
        </p>
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          className="flex items-center gap-2 px-4 py-2 rounded-xl bg-white/5 border border-white/10 text-white hover:bg-white/10 disabled:opacity-50"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
          Refresh
        </button>
      </div>

      {error && <p role="alert" className="text-red-400 text-sm">{error}</p>}
      {ok && (
        <p role="status" className="flex items-center gap-2 text-green-400 text-sm">
          <CheckCircle2 className="w-4 h-4" /> {ok}
        </p>
      )}

      {/* Geographic and KYC policy */}
      <section className="bg-white/5 border border-white/10 rounded-2xl p-6">
        <h3 className="text-lg font-bold text-white mb-4">Policy</h3>
        <div className="space-y-4">
          <div>
            <label htmlFor="restricted-countries" className="text-gray-400 text-sm mb-2 block">
              Restricted countries (ISO 3166-1 alpha-2, comma-separated)
            </label>
            <input
              id="restricted-countries"
              type="text"
              value={restricted}
              onChange={(event) => setRestricted(event.target.value)}
              placeholder="KP,IR"
              className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500"
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-white font-medium">Require verification to trade</p>
              <p className="text-gray-400 text-sm">An unverified account is refused on trading routes</p>
            </div>
            <input
              type="checkbox"
              checked={requireTrading}
              onChange={(event) => setRequireTrading(event.target.checked)}
              className="w-5 h-5"
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-white font-medium">Require verification to withdraw</p>
              <p className="text-gray-400 text-sm">An unverified account is refused on the withdrawal route</p>
            </div>
            <input
              type="checkbox"
              checked={requireWithdrawal}
              onChange={(event) => setRequireWithdrawal(event.target.checked)}
              className="w-5 h-5"
            />
          </div>
          <button
            type="button"
            onClick={() => void saveSettings()}
            disabled={busy === 'settings'}
            className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 text-white py-3 rounded-xl font-bold"
          >
            {busy === 'settings' ? 'Saving...' : 'Save policy'}
          </button>
          {settings && (
            <p className="text-gray-500 text-xs">
              Last updated {new Date(settings.updated_at).toLocaleString()}
            </p>
          )}
        </div>
      </section>

      {/* Identity review queue */}
      <section className="bg-white/5 border border-white/10 rounded-2xl p-6">
        <h3 className="text-lg font-bold text-white mb-4">Identity review</h3>
        {submissions.length === 0 ? (
          <p className="text-gray-400 text-sm">No submissions.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="text-gray-400 text-sm">
                  <th className="py-2 pr-4">Name</th>
                  <th className="py-2 pr-4">Country</th>
                  <th className="py-2 pr-4">Document</th>
                  <th className="py-2 pr-4">Status</th>
                  <th className="py-2 pr-4">Submitted</th>
                  <th className="py-2 pr-4">Decision</th>
                </tr>
              </thead>
              <tbody>
                {submissions.map((submission) => (
                  <tr key={submission.id} className="border-t border-white/5">
                    <td className="py-3 pr-4 text-white">{submission.full_name || submission.user_id}</td>
                    <td className="py-3 pr-4 text-gray-300">{submission.country}</td>
                    <td className="py-3 pr-4 text-gray-300">
                      {submission.document_type} ····{submission.document_last4}
                    </td>
                    <td className="py-3 pr-4">
                      <span className={`px-2 py-1 rounded-lg text-xs ${STATUS_STYLES[submission.status]}`}>
                        {submission.status}
                      </span>
                    </td>
                    <td className="py-3 pr-4 text-gray-400 text-sm">
                      {new Date(submission.submitted_at).toLocaleString()}
                    </td>
                    <td className="py-3 pr-4">
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() => void decide(submission, true)}
                          disabled={busy === submission.user_id || submission.status === 'approved'}
                          className="flex items-center gap-1 px-3 py-1 rounded-lg bg-green-500/20 text-green-300 disabled:opacity-40"
                        >
                          <ShieldCheck className="w-4 h-4" /> Approve
                        </button>
                        <button
                          type="button"
                          onClick={() => void decide(submission, false)}
                          disabled={busy === submission.user_id || submission.status === 'rejected'}
                          className="flex items-center gap-1 px-3 py-1 rounded-lg bg-red-500/20 text-red-300 disabled:opacity-40"
                        >
                          <ShieldX className="w-4 h-4" /> Reject
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Sanctions denylist */}
      <section className="bg-white/5 border border-white/10 rounded-2xl p-6">
        <h3 className="text-lg font-bold text-white mb-4">Sanctions denylist</h3>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3 mb-6">
          <select
            value={newKind}
            onChange={(event) => setNewKind(event.target.value as SanctionsKind)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"
          >
            <option value="address">Address</option>
            <option value="country">Country</option>
            <option value="name">Name</option>
          </select>
          <input
            type="text"
            value={newValue}
            onChange={(event) => setNewValue(event.target.value)}
            placeholder="0x… / US / john doe"
            className="md:col-span-2 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500"
          />
          <button
            type="button"
            onClick={() => void addEntry()}
            disabled={busy === 'add-entry' || !newValue.trim()}
            className="bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 text-white rounded-xl font-bold"
          >
            Add
          </button>
        </div>
        {entries.length === 0 ? (
          <p className="text-gray-400 text-sm">The denylist is empty.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="text-gray-400 text-sm">
                  <th className="py-2 pr-4">Kind</th>
                  <th className="py-2 pr-4">Value</th>
                  <th className="py-2 pr-4">Source</th>
                  <th className="py-2 pr-4">Active</th>
                  <th className="py-2 pr-4"></th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id} className="border-t border-white/5">
                    <td className="py-3 pr-4 text-gray-300">{entry.kind}</td>
                    <td className="py-3 pr-4 text-white break-all">{entry.value}</td>
                    <td className="py-3 pr-4 text-gray-400">{entry.source}</td>
                    <td className="py-3 pr-4 text-gray-300">{entry.active ? 'yes' : 'no'}</td>
                    <td className="py-3 pr-4">
                      <button
                        type="button"
                        onClick={() => void removeEntry(entry)}
                        disabled={busy === entry.id}
                        className="flex items-center gap-1 px-3 py-1 rounded-lg bg-red-500/20 text-red-300 disabled:opacity-40"
                      >
                        <Trash2 className="w-4 h-4" /> Remove
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
