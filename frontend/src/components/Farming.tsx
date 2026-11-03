'use client';

/**
 * Farming and airdrops panel.
 *
 * A farm stake escrows a real ledger balance and its reward comes from a funded
 * emission budget, so this panel only ever displays what the API reports:
 * stakes, pending reward and a nullable APR. When the API is unreachable the
 * panel says so rather than showing invented numbers.
 */

import { useState } from 'react';
import { AlertTriangle, Coins, Gift, Loader2, Sprout, Wallet } from 'lucide-react';

import {
  useAirdrops,
  useClaimAirdrop,
  useClaimFarmRewards,
  useFarms,
  useFarmStakes,
  useMyAirdrops,
  useStakeFarm,
  useUnstakeFarm,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';
import { ValidationError, parseDecimal } from '@/lib/validation';

/** Formats a base-10 integer string with a fixed number of decimal places. */
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

/** Formats an APR fraction as a percentage, or a dash when it is unknown. */
function formatApr(apr: number | null): string {
  if (apr === null || !Number.isFinite(apr)) {
    return '—';
  }
  return `${(apr * 100).toFixed(2)}%`;
}

export function Farming() {
  const authed = isAuthenticated();
  const [formError, setFormError] = useState<string | null>(null);

  const farmsQuery = useFarms();
  const stakesQuery = useFarmStakes();
  const airdropsQuery = useAirdrops();
  const allocationsQuery = useMyAirdrops();

  const stake = useStakeFarm();
  const unstake = useUnstakeFarm();
  const claimRewards = useClaimFarmRewards();
  const claimAirdrop = useClaimAirdrop();

  const farms = farmsQuery.data ?? [];
  const stakes = stakesQuery.data ?? [];
  const airdrops = airdropsQuery.data ?? [];
  const allocations = allocationsQuery.data ?? [];

  function handleAmount(raw: string, label: string): string | null {
    try {
      const amount = parseDecimal(raw, label);
      if (BigInt(amount) <= BigInt(0)) {
        throw new ValidationError(`${label} must be greater than zero`, label);
      }
      return amount;
    } catch (error) {
      setFormError(error instanceof Error ? error.message : `Invalid ${label}`);
      return null;
    }
  }

  if (farmsQuery.isPending || airdropsQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center text-gray-400">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
        Loading farms and airdrops…
      </div>
    );
  }

  if (farmsQuery.error || airdropsQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <AlertTriangle className="w-10 h-10 text-yellow-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Farming unavailable</h3>
        <p className="text-gray-400 text-sm" role="alert">
          {(farmsQuery.error ?? airdropsQuery.error)?.message}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {formError && (
        <p className="text-red-400 text-sm" role="alert">
          {formError}
        </p>
      )}

      {/* Live farms */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Liquidity farms</h2>
          <Sprout className="w-5 h-5 text-tiger-orange" />
        </div>
        {farms.length === 0 ? (
          <p className="text-gray-400 text-sm">No farms are active right now.</p>
        ) : (
          <div className="space-y-3">
            {farms.map((view) => (
              <FarmRow
                key={view.farm.id}
                farmId={view.farm.id}
                name={view.farm.name}
                symbol={view.farm.staking_token_symbol}
                totalStaked={view.farm.total_staked}
                status={view.farm.status}
                endTime={view.farm.end_time}
                apr={view.apr}
                rewards={view.rewards.map((reward) => `${reward.token_symbol} ${formatAmount(reward.rate_per_second)}/s`)}
                disabled={!authed || stake.isPending || unstake.isPending || claimRewards.isPending}
                onSubmit={(amount) => {
                  setFormError(null);
                  const parsed = handleAmount(amount, 'stake amount');
                  if (parsed) {
                    stake.mutate({ id: view.farm.id, amount: parsed });
                  }
                }}
                onUnstake={(amount) => {
                  setFormError(null);
                  const parsed = handleAmount(amount, 'unstake amount');
                  if (parsed) {
                    unstake.mutate({ id: view.farm.id, amount: parsed });
                  }
                }}
                onClaim={() => {
                  setFormError(null);
                  claimRewards.mutate({ id: view.farm.id });
                }}
              />
            ))}
          </div>
        )}
      </div>

      {/* My stakes */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Your stakes</h2>
          <Wallet className="w-5 h-5 text-tiger-orange" />
        </div>
        {!authed ? (
          <p className="text-gray-400 text-sm">Sign in to view and manage your stakes.</p>
        ) : stakesQuery.isPending ? (
          <p className="text-gray-400 text-sm">Loading stakes…</p>
        ) : stakesQuery.error ? (
          <p className="text-red-400 text-sm" role="alert">
            {stakesQuery.error.message}
          </p>
        ) : stakes.length === 0 ? (
          <p className="text-gray-400 text-sm">You have no active stakes.</p>
        ) : (
          <div className="space-y-3">
            {stakes.map((position) => (
              <div key={position.stake_id} className="p-4 rounded-xl border border-white/10 bg-white/5">
                <div className="flex items-center justify-between mb-2">
                  <div>
                    <p className="text-white font-medium">{position.farm_name}</p>
                    <p className="text-gray-500 text-xs">
                      {position.staking_token_symbol} · ends {formatDate(position.end_time)}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-gray-400 text-xs">Staked</p>
                    <p className="text-white font-mono">{formatAmount(position.staked)}</p>
                  </div>
                </div>
                <div className="pt-2 border-t border-white/10">
                  <p className="text-gray-400 text-xs mb-1 flex items-center">
                    <Gift className="w-3 h-3 mr-1" />
                    Pending rewards
                  </p>
                  {position.pending_rewards.length === 0 ? (
                    <p className="text-gray-500 text-xs">No reward streams.</p>
                  ) : (
                    <div className="space-y-1">
                      {position.pending_rewards.map((pending) => (
                        <div key={pending.token_address} className="flex items-center justify-between text-xs">
                          <span className="text-gray-400">{pending.symbol}</span>
                          <span className="text-white font-mono">{formatAmount(pending.pending)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Airdrops */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Airdrops</h2>
          <Coins className="w-5 h-5 text-tiger-orange" />
        </div>

        {!authed ? (
          <p className="text-gray-400 text-sm">Sign in to check your airdrop allocations.</p>
        ) : allocationsQuery.isPending ? (
          <p className="text-gray-400 text-sm">Loading allocations…</p>
        ) : allocationsQuery.error ? (
          <p className="text-red-400 text-sm" role="alert">
            {allocationsQuery.error.message}
          </p>
        ) : allocations.length === 0 ? (
          <p className="text-gray-400 text-sm">You have no airdrop allocations.</p>
        ) : (
          <div className="space-y-3">
            {allocations.map((allocation) => (
              <div key={allocation.allocation_id} className="p-4 rounded-xl border border-white/10 bg-white/5">
                <div className="flex items-center justify-between mb-2">
                  <div>
                    <p className="text-white font-medium">{allocation.campaign_name}</p>
                    <p className="text-gray-500 text-xs">
                      {allocation.symbol} · {allocation.status} · ends {formatDate(allocation.end_time)}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-gray-400 text-xs">Allocation</p>
                    <p className="text-white font-mono">{formatAmount(allocation.amount)}</p>
                  </div>
                </div>
                <button
                  onClick={() => {
                    setFormError(null);
                    claimAirdrop.mutate(allocation.campaign_id);
                  }}
                  disabled={allocation.claimed || claimAirdrop.isPending}
                  className="w-full py-2 rounded-lg bg-tiger-orange/20 text-tiger-orange text-sm hover:bg-tiger-orange/30 transition-colors disabled:opacity-40"
                >
                  {allocation.claimed ? 'Claimed' : 'Claim'}
                </button>
              </div>
            ))}
          </div>
        )}

        {airdrops.length > 0 && (
          <div className="mt-4 pt-4 border-t border-white/10">
            <p className="text-gray-400 text-xs mb-2">Open campaigns</p>
            <div className="space-y-1">
              {airdrops.map((campaign) => (
                <div key={campaign.id} className="flex items-center justify-between text-xs">
                  <span className="text-gray-400">{campaign.name}</span>
                  <span className="text-white font-mono">
                    {formatAmount(campaign.funded)} {campaign.token_symbol} funded
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

interface FarmRowProps {
  farmId: string;
  name: string;
  symbol: string;
  totalStaked: string;
  status: string;
  endTime: string;
  apr: number | null;
  rewards: string[];
  disabled: boolean;
  onSubmit: (amount: string) => void;
  onUnstake: (amount: string) => void;
  onClaim: () => void;
}

function FarmRow({
  farmId,
  name,
  symbol,
  totalStaked,
  status,
  endTime,
  apr,
  rewards,
  disabled,
  onSubmit,
  onUnstake,
  onClaim,
}: FarmRowProps) {
  const [amount, setAmount] = useState('');
  const inputId = `farm-amount-${farmId}`;

  return (
    <div className="p-4 rounded-xl border border-white/10 bg-white/5">
      <div className="flex items-center justify-between mb-2">
        <div>
          <p className="text-white font-medium">{name}</p>
          <p className="text-gray-500 text-xs capitalize">
            {status} · {symbol} · ends {formatDate(endTime)}
          </p>
        </div>
        <div className="text-right">
          <p className="text-gray-400 text-xs">APR</p>
          <p className="text-tiger-orange font-mono">{formatApr(apr)}</p>
        </div>
      </div>

      <div className="flex items-center justify-between text-xs mb-3">
        <span className="text-gray-400">Total staked</span>
        <span className="text-white font-mono">{formatAmount(totalStaked)}</span>
      </div>
      {rewards.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-3">
          {rewards.map((reward) => (
            <span key={reward} className="px-2 py-1 rounded bg-tiger-orange/10 text-tiger-orange text-xs font-mono">
              {reward}
            </span>
          ))}
        </div>
      )}

      <label className="block text-gray-400 text-xs mb-1" htmlFor={inputId}>
        Amount ({symbol})
      </label>
      <input
        id={inputId}
        value={amount}
        onChange={(event) => setAmount(event.target.value)}
        inputMode="decimal"
        placeholder="0"
        className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm mb-2 focus:outline-none focus:border-tiger-orange"
      />
      <div className="flex items-center space-x-2">
        <button
          onClick={() => onSubmit(amount)}
          disabled={disabled || amount === ''}
          className="flex-1 py-2 rounded-lg bg-tiger-orange text-black font-semibold text-sm hover:bg-tiger-orange/90 transition-colors disabled:opacity-40"
        >
          Stake
        </button>
        <button
          onClick={() => onUnstake(amount)}
          disabled={disabled || amount === ''}
          className="flex-1 py-2 rounded-lg bg-white/10 text-white text-sm hover:bg-white/20 transition-colors disabled:opacity-40"
        >
          Unstake
        </button>
        <button
          onClick={onClaim}
          disabled={disabled}
          className="flex-1 py-2 rounded-lg bg-tiger-orange/20 text-tiger-orange text-sm hover:bg-tiger-orange/30 transition-colors disabled:opacity-40"
        >
          Claim
        </button>
      </div>
    </div>
  );
}
