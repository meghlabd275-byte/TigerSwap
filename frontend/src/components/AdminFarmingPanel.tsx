'use client';

/**
 * Admin panel for farms and airdrops.
 *
 * Farms and airdrop campaigns are created paused and must be funded and
 * activated explicitly, so nothing pays out until an operator says so. Every
 * write goes to the admin endpoints; the panel never computes a reward or an
 * allocation in the browser.
 */

import { useCallback, useEffect, useState } from 'react';
import { AlertCircle, Coins, Plus, RefreshCw, Sprout, Trash2 } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { AirdropCampaign, FarmView } from '@/lib/types';
import { ValidationError, parseDecimal, parseInteger } from '@/lib/validation';

function errorMessage(error: unknown): string {
  if (error instanceof ValidationError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unexpected error';
}

/** Formats a base-10 integer string for display. */
function formatAmount(raw: string, decimals = 4): string {
  const negative = raw.startsWith('-');
  const digits = (negative ? raw.slice(1) : raw).padStart(decimals + 1, '0');
  const whole = digits.slice(0, digits.length - decimals);
  const frac = digits.slice(digits.length - decimals).replace(/0+$/, '');
  const withSeparators = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${negative ? '-' : ''}${withSeparators}${frac ? `.${frac}` : ''}`;
}

const FARM_TYPES = ['lp', 'single', 'stable'];
const STATUSES = ['active', 'paused', 'closed'];

export function AdminFarmingPanel() {
  const [kind, setKind] = useState<'farms' | 'airdrops'>('farms');
  const [farms, setFarms] = useState<FarmView[]>([]);
  const [airdrops, setAirdrops] = useState<AirdropCampaign[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      if (kind === 'farms') {
        setFarms(await endpoints.getAllFarms());
      } else {
        setAirdrops(await endpoints.getAllAirdrops());
      }
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setLoading(false);
    }
  }, [kind]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-2">
          {(['farms', 'airdrops'] as const).map((option) => (
            <button
              key={option}
              onClick={() => setKind(option)}
              className={`px-4 py-2 rounded-lg text-sm font-medium capitalize transition-colors ${
                kind === option ? 'bg-tiger-orange/20 text-tiger-orange' : 'text-gray-400 hover:text-white hover:bg-white/5'
              }`}
            >
              {option}
            </button>
          ))}
        </div>
        <button
          onClick={() => void load()}
          disabled={loading}
          className="flex items-center space-x-2 px-4 py-2 bg-white/10 text-white rounded-lg hover:bg-white/20 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {error && (
        <p role="alert" className="flex items-center space-x-2 text-red-400 text-sm">
          <AlertCircle className="w-4 h-4" />
          <span>{error}</span>
        </p>
      )}

      {kind === 'farms' ? (
        <FarmAdminTable rows={farms} onReload={load} onError={setError} />
      ) : (
        <AirdropAdminTable rows={airdrops} onReload={load} onError={setError} />
      )}
    </div>
  );
}

interface AdminTableProps<T> {
  rows: T[];
  onReload: () => Promise<void>;
  onError: (message: string) => void;
}

function FarmAdminTable({ rows, onReload, onError }: AdminTableProps<FarmView>) {
  const [showForm, setShowForm] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [farmType, setFarmType] = useState('lp');
  const [chainId, setChainId] = useState('1');
  const [stakeToken, setStakeToken] = useState('');
  const [stakeSymbol, setStakeSymbol] = useState('');
  const [stakeDecimals, setStakeDecimals] = useState('18');
  const [rewardToken, setRewardToken] = useState('');
  const [rewardSymbol, setRewardSymbol] = useState('');
  const [rewardDecimals, setRewardDecimals] = useState('18');
  const [ratePerSecond, setRatePerSecond] = useState('');
  const [fundAmount, setFundAmount] = useState('');

  async function createFarm() {
    onError('');
    try {
      const parsedChain = parseInteger(chainId, 'chain id', { min: 1 });
      const parsedStakeDecimals = parseInteger(stakeDecimals, 'staking decimals', { min: 0, max: 36 });
      const parsedRewardDecimals = parseInteger(rewardDecimals, 'reward decimals', { min: 0, max: 36 });
      const rate = parseDecimal(ratePerSecond, 'rate per second');
      if (BigInt(rate) <= BigInt(0)) {
        throw new ValidationError('rate per second must be greater than zero', 'rate per second');
      }
      const now = new Date();
      const end = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000);
      await endpoints.createFarm({
        key: key.trim(),
        name: name.trim(),
        farm_type: farmType,
        chain_id: parsedChain,
        staking_token_address: stakeToken.trim(),
        staking_token_symbol: stakeSymbol.trim(),
        staking_token_decimals: parsedStakeDecimals,
        start_time: now.toISOString(),
        end_time: end.toISOString(),
        rewards: [
          {
            token_address: rewardToken.trim(),
            token_symbol: rewardSymbol.trim(),
            token_decimals: parsedRewardDecimals,
            rate_per_second: rate,
          },
        ],
      });
      setShowForm(false);
      setKey('');
      setName('');
      setStakeToken('');
      setStakeSymbol('');
      setRewardToken('');
      setRewardSymbol('');
      setRatePerSecond('');
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  async function setStatus(id: string, status: string) {
    setBusy(id);
    onError('');
    try {
      await endpoints.updateFarm(id, { status });
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  async function fund(id: string, tokenAddress: string) {
    setBusy(id);
    onError('');
    try {
      const amount = parseDecimal(fundAmount, 'fund amount');
      if (BigInt(amount) <= BigInt(0)) {
        throw new ValidationError('fund amount must be greater than zero', 'fund amount');
      }
      await endpoints.fundFarm(id, tokenAddress, amount);
      setFundAmount('');
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  async function remove(id: string) {
    setBusy(id);
    onError('');
    try {
      await endpoints.deleteFarm(id);
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center space-x-3">
        <button
          onClick={() => setShowForm((open) => !open)}
          className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-black font-semibold rounded-lg hover:bg-tiger-orange/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          <span>Create Farm</span>
        </button>
        <input
          value={fundAmount}
          onChange={(event) => setFundAmount(event.target.value)}
          inputMode="decimal"
          placeholder="Fund amount (smallest unit)"
          aria-label="Farm fund amount"
          className="flex-1 max-w-xs bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm"
        />
      </div>

      {showForm && (
        <div className="bg-white/5 rounded-2xl p-6 space-y-4">
          <div className="grid md:grid-cols-3 gap-4">
            <Field label="Key" value={key} onChange={setKey} placeholder="tgr-rusd-lp" />
            <Field label="Name" value={name} onChange={setName} placeholder="TGR/RUSD LP Farm" />
            <div>
              <label className="block text-gray-400 text-xs mb-1" htmlFor="farm-type">Farm type</label>
              <select
                id="farm-type"
                value={farmType}
                onChange={(event) => setFarmType(event.target.value)}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white"
              >
                {FARM_TYPES.map((option) => <option key={option} value={option}>{option}</option>)}
              </select>
            </div>
            <Field label="Chain id" value={chainId} onChange={setChainId} placeholder="1" />
            <Field label="Staking token address" value={stakeToken} onChange={setStakeToken} placeholder="0x…" />
            <Field label="Staking symbol" value={stakeSymbol} onChange={setStakeSymbol} placeholder="TGR-RUSD-LP" />
            <Field label="Staking decimals" value={stakeDecimals} onChange={setStakeDecimals} placeholder="18" />
            <Field label="Reward token address" value={rewardToken} onChange={setRewardToken} placeholder="0x…" />
            <Field label="Reward symbol" value={rewardSymbol} onChange={setRewardSymbol} placeholder="TGR" />
            <Field label="Reward decimals" value={rewardDecimals} onChange={setRewardDecimals} placeholder="18" />
            <Field label="Rate per second" value={ratePerSecond} onChange={setRatePerSecond} placeholder="1000000000000" />
          </div>
          <button
            onClick={() => void createFarm()}
            className="px-4 py-2 bg-tiger-orange text-black font-semibold rounded-lg hover:bg-tiger-orange/90 transition-colors"
          >
            Create farm
          </button>
          <p className="text-gray-500 text-xs">
            The farm is created paused. Fund its reward stream, then activate it.
          </p>
        </div>
      )}

      <div className="bg-white/5 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/5 text-gray-400">
            <tr>
              <th className="text-left px-4 py-3">Farm</th>
              <th className="text-left px-4 py-3">Type</th>
              <th className="text-left px-4 py-3">Total staked</th>
              <th className="text-left px-4 py-3">Rewards</th>
              <th className="text-left px-4 py-3">Status</th>
              <th className="text-right px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">
                  <Sprout className="w-8 h-8 mx-auto mb-2 text-gray-500" />
                  No farms yet.
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr key={row.farm.id} className="border-t border-white/5">
                  <td className="px-4 py-3">
                    <p className="text-white font-medium">{row.farm.name}</p>
                    <p className="text-gray-500 text-xs font-mono">{row.farm.key}</p>
                  </td>
                  <td className="px-4 py-3 text-gray-300 capitalize">{row.farm.farm_type}</td>
                  <td className="px-4 py-3 text-white font-mono">{formatAmount(row.farm.total_staked)}</td>
                  <td className="px-4 py-3 text-gray-300">
                    {row.rewards.length === 0 ? (
                      '—'
                    ) : (
                      <div className="space-y-1">
                        {row.rewards.map((reward) => (
                          <div key={reward.id} className="flex items-center justify-between space-x-2">
                            <span className="font-mono text-xs">
                              {reward.token_symbol} · funded {formatAmount(reward.total_reward)}
                            </span>
                            <button
                              onClick={() => void fund(row.farm.id, reward.token_address)}
                              disabled={busy === row.farm.id || fundAmount === ''}
                              aria-label={`Fund ${reward.token_symbol} rewards for ${row.farm.key}`}
                              className="px-2 py-0.5 rounded bg-blue-500/20 text-blue-400 text-xs hover:bg-blue-500/30 disabled:opacity-40"
                            >
                              Fund
                            </button>
                          </div>
                        ))}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded-full text-xs ${
                      row.farm.status === 'active' ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'
                    }`}>
                      {row.farm.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end space-x-2">
                      {row.farm.status !== 'active' && (
                        <button
                          onClick={() => void setStatus(row.farm.id, 'active')}
                          disabled={busy === row.farm.id}
                          className="px-3 py-1 rounded bg-green-500/20 text-green-400 text-xs hover:bg-green-500/30 disabled:opacity-40"
                        >
                          Activate
                        </button>
                      )}
                      {row.farm.status === 'active' && (
                        <button
                          onClick={() => void setStatus(row.farm.id, 'paused')}
                          disabled={busy === row.farm.id}
                          className="px-3 py-1 rounded bg-yellow-500/20 text-yellow-400 text-xs hover:bg-yellow-500/30 disabled:opacity-40"
                        >
                          Pause
                        </button>
                      )}
                      <button
                        onClick={() => void remove(row.farm.id)}
                        disabled={busy === row.farm.id}
                        aria-label={`Close farm ${row.farm.key}`}
                        className="p-1 text-red-400 hover:text-red-300 disabled:opacity-40"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function AirdropAdminTable({ rows, onReload, onError }: AdminTableProps<AirdropCampaign>) {
  const [showForm, setShowForm] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [chainId, setChainId] = useState('1');
  const [tokenAddress, setTokenAddress] = useState('');
  const [tokenSymbol, setTokenSymbol] = useState('');
  const [tokenDecimals, setTokenDecimals] = useState('18');
  const [allocations, setAllocations] = useState('');
  const [fundAmount, setFundAmount] = useState('');

  async function createCampaign() {
    onError('');
    try {
      const parsedChain = parseInteger(chainId, 'chain id', { min: 1 });
      const parsedDecimals = parseInteger(tokenDecimals, 'token decimals', { min: 0, max: 36 });
      const now = new Date();
      const end = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000);
      const campaign = await endpoints.createAirdrop({
        key: key.trim(),
        name: name.trim(),
        chain_id: parsedChain,
        token_address: tokenAddress.trim(),
        token_symbol: tokenSymbol.trim(),
        token_decimals: parsedDecimals,
        start_time: now.toISOString(),
        end_time: end.toISOString(),
      });

      if (allocations.trim() !== '') {
        const parsed = allocations
          .split('\n')
          .map((line) => line.trim())
          .filter((line) => line !== '')
          .map((line) => {
            const [user, amount] = line.split(/[,\s]+/);
            if (!user || !amount) {
              throw new ValidationError('each allocation line must be "user_id amount"', 'allocations');
            }
            const parsedAmount = parseDecimal(amount, 'allocation amount');
            if (BigInt(parsedAmount) <= BigInt(0)) {
              throw new ValidationError('allocation amounts must be greater than zero', 'allocations');
            }
            return { user_id: user, amount: parsedAmount };
          });
        await endpoints.setAirdropAllocations(campaign.id, parsed);
      }
      setShowForm(false);
      setKey('');
      setName('');
      setTokenAddress('');
      setTokenSymbol('');
      setAllocations('');
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    }
  }

  async function fund(id: string) {
    setBusy(id);
    onError('');
    try {
      const amount = parseDecimal(fundAmount, 'fund amount');
      if (BigInt(amount) <= BigInt(0)) {
        throw new ValidationError('fund amount must be greater than zero', 'fund amount');
      }
      await endpoints.fundAirdrop(id, amount);
      setFundAmount('');
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  async function setStatus(id: string, status: string) {
    setBusy(id);
    onError('');
    try {
      await endpoints.updateAirdrop(id, { status });
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  async function remove(id: string) {
    setBusy(id);
    onError('');
    try {
      await endpoints.deleteAirdrop(id);
      await onReload();
    } catch (caught) {
      onError(errorMessage(caught));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center space-x-3">
        <button
          onClick={() => setShowForm((open) => !open)}
          className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-black font-semibold rounded-lg hover:bg-tiger-orange/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          <span>Create Campaign</span>
        </button>
        <input
          value={fundAmount}
          onChange={(event) => setFundAmount(event.target.value)}
          inputMode="decimal"
          placeholder="Fund amount (smallest unit)"
          aria-label="Fund amount"
          className="flex-1 max-w-xs bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm"
        />
      </div>

      {showForm && (
        <div className="bg-white/5 rounded-2xl p-6 space-y-4">
          <div className="grid md:grid-cols-3 gap-4">
            <Field label="Key" value={key} onChange={setKey} placeholder="tgr-genesis" />
            <Field label="Name" value={name} onChange={setName} placeholder="TGR Genesis" />
            <Field label="Chain id" value={chainId} onChange={setChainId} placeholder="1" />
            <Field label="Token address" value={tokenAddress} onChange={setTokenAddress} placeholder="0x…" />
            <Field label="Token symbol" value={tokenSymbol} onChange={setTokenSymbol} placeholder="TGR" />
            <Field label="Token decimals" value={tokenDecimals} onChange={setTokenDecimals} placeholder="18" />
          </div>
          <div>
            <label className="block text-gray-400 text-xs mb-1" htmlFor="allocations">
              Allocations (one per line: user_id amount)
            </label>
            <textarea
              id="allocations"
              value={allocations}
              onChange={(event) => setAllocations(event.target.value)}
              rows={4}
              placeholder={'user-1 500000000000000000000\nuser-2 250000000000000000000'}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm"
            />
          </div>
          <button
            onClick={() => void createCampaign()}
            className="px-4 py-2 bg-tiger-orange text-black font-semibold rounded-lg hover:bg-tiger-orange/90 transition-colors"
          >
            Create campaign
          </button>
          <p className="text-gray-500 text-xs">
            The campaign is created paused. Fund it so its escrow covers the allocations, then activate it.
          </p>
        </div>
      )}

      <div className="bg-white/5 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/5 text-gray-400">
            <tr>
              <th className="text-left px-4 py-3">Campaign</th>
              <th className="text-left px-4 py-3">Token</th>
              <th className="text-left px-4 py-3">Allocated</th>
              <th className="text-left px-4 py-3">Funded</th>
              <th className="text-left px-4 py-3">Claimed</th>
              <th className="text-left px-4 py-3">Status</th>
              <th className="text-right px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">
                  <Coins className="w-8 h-8 mx-auto mb-2 text-gray-500" />
                  No airdrop campaigns yet.
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr key={row.id} className="border-t border-white/5">
                  <td className="px-4 py-3">
                    <p className="text-white font-medium">{row.name}</p>
                    <p className="text-gray-500 text-xs font-mono">{row.key}</p>
                  </td>
                  <td className="px-4 py-3 text-gray-300">{row.token_symbol}</td>
                  <td className="px-4 py-3 text-white font-mono">{formatAmount(row.total_allocated)}</td>
                  <td className="px-4 py-3 text-white font-mono">{formatAmount(row.funded)}</td>
                  <td className="px-4 py-3 text-white font-mono">{formatAmount(row.total_claimed)}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded-full text-xs ${
                      row.status === 'active' ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'
                    }`}>
                      {row.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end space-x-2">
                      <button
                        onClick={() => void fund(row.id)}
                        disabled={busy === row.id || fundAmount === ''}
                        className="px-3 py-1 rounded bg-blue-500/20 text-blue-400 text-xs hover:bg-blue-500/30 disabled:opacity-40"
                      >
                        Fund
                      </button>
                      {row.status !== 'active' ? (
                        <button
                          onClick={() => void setStatus(row.id, 'active')}
                          disabled={busy === row.id}
                          className="px-3 py-1 rounded bg-green-500/20 text-green-400 text-xs hover:bg-green-500/30 disabled:opacity-40"
                        >
                          Activate
                        </button>
                      ) : (
                        <button
                          onClick={() => void setStatus(row.id, 'paused')}
                          disabled={busy === row.id}
                          className="px-3 py-1 rounded bg-yellow-500/20 text-yellow-400 text-xs hover:bg-yellow-500/30 disabled:opacity-40"
                        >
                          Pause
                        </button>
                      )}
                      <button
                        onClick={() => void remove(row.id)}
                        disabled={busy === row.id}
                        aria-label={`Delete campaign ${row.key}`}
                        className="p-1 text-red-400 hover:text-red-300 disabled:opacity-40"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

interface FieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

function Field({ label, value, onChange, placeholder }: FieldProps) {
  const id = `field-${label.replace(/\s+/g, '-').toLowerCase()}`;
  return (
    <div>
      <label className="block text-gray-400 text-xs mb-1" htmlFor={id}>{label}</label>
      <input
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-tiger-orange"
      />
    </div>
  );
}
