'use client';

/**
 * Governance panel: veTGR locking, gauge voting and the bribe market.
 *
 * Everything shown here comes from the API and is validated before it is
 * rendered. When the backend is unreachable the panel says so rather than
 * showing invented numbers.
 */

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Coins, Loader2, Lock, Vote, Gift } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import { ValidationError, parseDecimal } from '@/lib/validation';

const MIN_LOCK_SECONDS = 7 * 24 * 60 * 60;
const MAX_LOCK_SECONDS = 4 * 365 * 24 * 60 * 60;
const DAY_SECONDS = 24 * 60 * 60;

/** Formats a base-10 wei string for display without losing precision. */
function formatAmount(raw: string, decimals = 4): string {
  const negative = raw.startsWith('-');
  const digits = (negative ? raw.slice(1) : raw).padStart(decimals + 1, '0');
  const whole = digits.slice(0, digits.length - decimals);
  const frac = digits.slice(digits.length - decimals).replace(/0+$/, '');
  const withSeparators = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${negative ? '-' : ''}${withSeparators}${frac ? `.${frac}` : ''}`;
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function Governance() {
  const queryClient = useQueryClient();
  const [lockAmount, setLockAmount] = useState('');
  const [lockDays, setLockDays] = useState('365');
  const [formError, setFormError] = useState<string | null>(null);

  const locksQuery = useQuery({
    queryKey: ['governance', 'locks'],
    queryFn: () => endpoints.getLocks(),
    retry: false,
  });

  const gaugesQuery = useQuery({
    queryKey: ['governance', 'gauges'],
    queryFn: () => endpoints.getGauges(true),
    retry: false,
  });

  const createLock = useMutation({
    mutationFn: (payload: { amount: string; duration_seconds: number }) => endpoints.createLock(payload),
    onSuccess: () => {
      setLockAmount('');
      setFormError(null);
      queryClient.invalidateQueries({ queryKey: ['governance'] });
    },
    onError: (error: Error) => setFormError(error.message),
  });

  const withdrawLock = useMutation({
    mutationFn: (id: string) => endpoints.withdrawLock(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['governance'] }),
    onError: (error: Error) => setFormError(error.message),
  });

  const vote = useMutation({
    mutationFn: (payload: { gaugeId: string; weight: string }) => endpoints.vote(payload.gaugeId, payload.weight),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['governance'] }),
    onError: (error: Error) => setFormError(error.message),
  });

  const locks = locksQuery.data ?? [];
  const gauges = gaugesQuery.data ?? [];

  const totalVotingPower = useMemo(
    () => locks.reduce((sum, lock) => sum + BigInt(lock.voting_power || '0'), BigInt(0)),
    [locks],
  );

  const activeLocks = locks.filter((lock) => !lock.withdrawn);

  function handleCreateLock() {
    setFormError(null);
    try {
      // parseDecimal rejects anything that is not a plain decimal string, so
      // no malformed amount can reach the API.
      const amount = parseDecimal(lockAmount, 'lock amount');
      if (BigInt(amount) <= BigInt(0)) {
        throw new ValidationError('Lock amount must be greater than zero', 'lock amount');
      }
      const days = Number(lockDays);
      if (!Number.isFinite(days) || days <= 0) {
        throw new ValidationError('Lock duration must be a positive number of days', 'duration');
      }
      const seconds = Math.round(days * DAY_SECONDS);
      if (seconds < MIN_LOCK_SECONDS || seconds > MAX_LOCK_SECONDS) {
        throw new ValidationError('Lock duration must be between 7 days and 4 years', 'duration');
      }
      createLock.mutate({ amount, duration_seconds: seconds });
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Invalid lock parameters');
    }
  }

  function handleVote(gaugeId: string, rawWeight: string) {
    setFormError(null);
    try {
      const weight = parseDecimal(rawWeight, 'vote weight');
      vote.mutate({ gaugeId, weight });
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Invalid vote weight');
    }
  }

  if (locksQuery.isPending || gaugesQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center text-gray-400">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
        Loading governance data…
      </div>
    );
  }

  if (locksQuery.error || gaugesQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <AlertTriangle className="w-10 h-10 text-yellow-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Governance unavailable</h3>
        <p className="text-gray-400 text-sm" role="alert">
          {(locksQuery.error ?? gaugesQuery.error)?.message}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Voting power summary */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">veTGR voting power</h2>
          <Vote className="w-5 h-5 text-tiger-orange" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-gray-400 text-xs">Total voting power</p>
            <p className="text-white font-mono text-lg">{formatAmount(totalVotingPower.toString())}</p>
          </div>
          <div>
            <p className="text-gray-400 text-xs">Active locks</p>
            <p className="text-white font-mono text-lg">{activeLocks.length}</p>
          </div>
        </div>
        <p className="text-gray-500 text-xs mt-3">
          Voting power decays linearly to zero at each lock&apos;s unlock date.
        </p>
      </div>

      {/* Create lock */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Lock TGR</h2>
          <Lock className="w-5 h-5 text-tiger-orange" />
        </div>

        <label className="block text-gray-400 text-xs mb-1" htmlFor="lock-amount">
          Amount (TGR)
        </label>
        <input
          id="lock-amount"
          value={lockAmount}
          onChange={(event) => setLockAmount(event.target.value)}
          inputMode="decimal"
          placeholder="0.0"
          className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white font-mono mb-4 focus:outline-none focus:border-tiger-orange"
        />

        <label className="block text-gray-400 text-xs mb-1" htmlFor="lock-days">
          Duration (days, 7 to 1460)
        </label>
        <input
          id="lock-days"
          value={lockDays}
          onChange={(event) => setLockDays(event.target.value)}
          inputMode="numeric"
          className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white font-mono mb-4 focus:outline-none focus:border-tiger-orange"
        />

        <button
          onClick={handleCreateLock}
          disabled={createLock.isPending || lockAmount === ''}
          className="w-full py-3 rounded-xl bg-tiger-orange text-black font-semibold hover:bg-tiger-orange/90 transition-colors disabled:opacity-50 flex items-center justify-center space-x-2"
        >
          {createLock.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Lock className="w-4 h-4" />}
          <span>Create lock</span>
        </button>

        {formError && (
          <p className="text-red-400 text-sm mt-3" role="alert">
            {formError}
          </p>
        )}
      </div>

      {/* Existing locks */}
      <div className="glass-dark rounded-2xl p-6">
        <h2 className="text-lg font-display font-bold text-white mb-4">Your locks</h2>
        {locks.length === 0 ? (
          <p className="text-gray-400 text-sm">You have no locks yet.</p>
        ) : (
          <div className="space-y-3">
            {locks.map((lock) => {
              const matured = new Date(lock.unlock_time).getTime() <= Date.now();
              return (
                <div key={lock.id} className="p-4 rounded-xl border border-white/10 bg-white/5">
                  <div className="flex items-center justify-between mb-2">
                    <div>
                      <p className="text-white font-mono">{formatAmount(lock.amount)} TGR</p>
                      <p className="text-gray-500 text-xs">
                        Unlocks {formatDate(lock.unlock_time)}
                        {lock.withdrawn ? ' · withdrawn' : ''}
                      </p>
                    </div>
                    <div className="text-right">
                      <p className="text-gray-400 text-xs">Voting power</p>
                      <p className="text-white font-mono">{formatAmount(lock.voting_power)}</p>
                    </div>
                  </div>
                  {!lock.withdrawn && (
                    <button
                      onClick={() => withdrawLock.mutate(lock.id)}
                      disabled={!matured || withdrawLock.isPending}
                      className="w-full py-2 rounded-lg bg-white/10 text-white text-sm hover:bg-white/20 transition-colors disabled:opacity-40"
                    >
                      {matured ? 'Withdraw' : 'Locked until unlock date'}
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Gauge voting */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Gauge voting</h2>
          <Coins className="w-5 h-5 text-tiger-orange" />
        </div>
        {gauges.length === 0 ? (
          <p className="text-gray-400 text-sm">No gauges have been created yet.</p>
        ) : (
          <div className="space-y-3">
            {gauges.map((gauge) => (
              <GaugeRow
                key={gauge.id}
                gaugeId={gauge.id}
                name={gauge.name}
                gaugeType={gauge.gauge_type}
                totalVoted={gauge.total_voted}
                votingPower={totalVotingPower.toString()}
                onVote={handleVote}
                pending={vote.isPending}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

interface GaugeRowProps {
  gaugeId: string;
  name: string;
  gaugeType: string;
  totalVoted: string;
  votingPower: string;
  onVote: (gaugeId: string, weight: string) => void;
  pending: boolean;
}

function GaugeRow({ gaugeId, name, gaugeType, totalVoted, votingPower, onVote, pending }: GaugeRowProps) {
  const [weight, setWeight] = useState('');
  const bribesQuery = useQuery({
    queryKey: ['governance', 'bribes', gaugeId],
    queryFn: () => endpoints.getBribes(gaugeId),
    retry: false,
  });

  const bribes = bribesQuery.data ?? [];

  return (
    <div className="p-4 rounded-xl border border-white/10 bg-white/5">
      <div className="flex items-center justify-between mb-2">
        <div>
          <p className="text-white font-medium">{name}</p>
          <p className="text-gray-500 text-xs capitalize">{gaugeType} gauge</p>
        </div>
        <div className="text-right">
          <p className="text-gray-400 text-xs">Total voted</p>
          <p className="text-white font-mono">{formatAmount(totalVoted)}</p>
        </div>
      </div>

      <div className="flex items-center space-x-2">
        <input
          value={weight}
          onChange={(event) => setWeight(event.target.value)}
          inputMode="decimal"
          placeholder={`up to ${formatAmount(votingPower)}`}
          aria-label={`Vote weight for ${name}`}
          className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-tiger-orange"
        />
        <button
          onClick={() => onVote(gaugeId, weight)}
          disabled={pending || weight === ''}
          className="px-4 py-2 rounded-lg bg-tiger-orange/20 text-tiger-orange text-sm hover:bg-tiger-orange/30 transition-colors disabled:opacity-40"
        >
          Vote
        </button>
      </div>

      {bribes.length > 0 && (
        <div className="mt-3 pt-3 border-t border-white/10">
          <p className="text-gray-400 text-xs flex items-center mb-2">
            <Gift className="w-3 h-3 mr-1" />
            Bribes for this gauge
          </p>
          <div className="space-y-1">
            {bribes.map((bribe) => (
              <div key={bribe.id} className="flex items-center justify-between text-xs">
                <span className="text-gray-400">
                  Epoch {bribe.epoch} · {bribe.token}
                </span>
                <span className="text-white font-mono">{formatAmount(bribe.amount)}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
