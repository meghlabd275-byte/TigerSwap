'use client';

/**
 * Admin listings panel: futures markets, perpetual listings and margin
 * listings.
 *
 * Every table here is loaded from and written to the admin listing endpoints
 * through `endpoints.ts`. New listings are created paused by the server and
 * must be activated explicitly, so nothing is tradable until an operator says
 * so. Failures are surfaced to the operator rather than hidden behind an
 * optimistic update.
 */

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Plus, RefreshCw, Trash2, ToggleLeft, ToggleRight, AlertCircle } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type {
  FuturesMarket,
  FuturesMarketInput,
  MarginListing,
  MarginListingInput,
  PerpsListing,
  PerpsListingInput,
} from '@/lib/types';
import {
  parseDecimal,
  parseInteger,
  parseRatio,
  parseSymbol,
  ValidationError,
} from '@/lib/validation';

type ListingKind = 'futures' | 'perps' | 'margin';

const CONTRACT_TYPES = ['RUSD_M', 'USDT_M', 'USDC_M', 'RYUSD_M', 'COIN_M', 'TGR_M'];
const SETTLEMENTS = ['RUSD', 'USDT', 'USDC', 'RYUSD', 'BTC', 'ETH', 'TGR'];
const LISTING_TYPES = ['perpetual', 'quarterly', 'bi_quarterly', 'delivery'];
const MARKET_TYPES = ['oracle', 'orderbook'];
const MARGIN_MODES = ['cross', 'isolated'];
const RISK_TIERS = ['low', 'medium', 'high'];

function errorMessage(error: unknown): string {
  if (error instanceof ValidationError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unexpected error';
}

export function AdminListingsPanel() {
  const [kind, setKind] = useState<ListingKind>('futures');
  const [futures, setFutures] = useState<FuturesMarket[]>([]);
  const [perps, setPerps] = useState<PerpsListing[]>([]);
  const [margin, setMargin] = useState<MarginListing[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [f, p, m] = await Promise.all([
        endpoints.getFuturesMarkets(),
        endpoints.getPerpsListings(),
        endpoints.getMarginListings(),
      ]);
      setFutures(f);
      setPerps(p);
      setMargin(m);
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const toggleStatus = useCallback(
    async (id: string, current: string) => {
      setBusyId(id);
      setError(null);
      const next = current === 'active' ? 'paused' : 'active';
      try {
        if (kind === 'futures') {
          await endpoints.updateFuturesMarket(id, { status: next });
        } else if (kind === 'perps') {
          await endpoints.updatePerpsListing(id, { status: next });
        } else {
          await endpoints.updateMarginListing(id, { status: next });
        }
        await load();
      } catch (toggleError) {
        setError(errorMessage(toggleError));
      } finally {
        setBusyId(null);
      }
    },
    [kind, load],
  );

  const remove = useCallback(
    async (id: string) => {
      setBusyId(id);
      setError(null);
      try {
        if (kind === 'futures') {
          await endpoints.deleteFuturesMarket(id);
        } else if (kind === 'perps') {
          await endpoints.deletePerpsListing(id);
        } else {
          await endpoints.deleteMarginListing(id);
        }
        await load();
      } catch (removeError) {
        setError(errorMessage(removeError));
      } finally {
        setBusyId(null);
      }
    },
    [kind, load],
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-white">Market Listings</h2>
        <div className="flex items-center space-x-3">
          <div className="flex rounded-lg overflow-hidden border border-white/10">
            {(['futures', 'perps', 'margin'] as ListingKind[]).map((k) => (
              <button
                key={k}
                onClick={() => setKind(k)}
                className={`px-4 py-2 text-sm font-medium capitalize transition-colors ${
                  kind === k ? 'bg-tiger-orange text-white' : 'bg-white/5 text-gray-400 hover:text-white'
                }`}
              >
                {k}
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
          <button
            onClick={() => setShowForm((open) => !open)}
            className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
          >
            <Plus className="w-4 h-4" />
            <span>New Listing</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-center space-x-2 mb-4 px-4 py-3 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400 text-sm">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {showForm && (
        <CreateListingForm
          kind={kind}
          onCreated={async () => {
            setShowForm(false);
            await load();
          }}
          onError={(message) => setError(message)}
        />
      )}

      <div className="bg-white/5 rounded-2xl overflow-hidden">
        {kind === 'futures' && (
          <table className="w-full">
            <thead className="bg-white/5">
              <tr>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Symbol</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Contract</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Settlement</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Max Leverage</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Funding / 8h</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {futures.map((market) => (
                <tr key={market.id} className="border-t border-white/5">
                  <td className="px-6 py-4 text-white font-medium">{market.symbol}</td>
                  <td className="px-6 py-4 text-gray-400">{market.contract_type}</td>
                  <td className="px-6 py-4 text-gray-400">{market.settlement}</td>
                  <td className="px-6 py-4 text-white">{market.max_leverage}x</td>
                  <td className="px-6 py-4 text-gray-400">{market.funding_rate}</td>
                  <td className="px-6 py-4">
                    <StatusCell status={market.status} busy={busyId === market.id} onToggle={() => void toggleStatus(market.id, market.status)} />
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => void remove(market.id)}
                      disabled={busyId === market.id}
                      className="p-2 text-gray-400 hover:text-red-400 disabled:opacity-50"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {kind === 'perps' && (
          <table className="w-full">
            <thead className="bg-white/5">
              <tr>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Symbol</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Type</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Collateral</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Max Leverage</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Taker Fee</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {perps.map((listing) => (
                <tr key={listing.id} className="border-t border-white/5">
                  <td className="px-6 py-4 text-white font-medium">{listing.symbol}</td>
                  <td className="px-6 py-4 text-gray-400">{listing.market_type}</td>
                  <td className="px-6 py-4 text-gray-400">{listing.collateral_assets}</td>
                  <td className="px-6 py-4 text-white">{listing.max_leverage}x</td>
                  <td className="px-6 py-4 text-gray-400">{(listing.taker_fee_rate * 100).toFixed(3)}%</td>
                  <td className="px-6 py-4">
                    <StatusCell status={listing.status} busy={busyId === listing.id} onToggle={() => void toggleStatus(listing.id, listing.status)} />
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => void remove(listing.id)}
                      disabled={busyId === listing.id}
                      className="p-2 text-gray-400 hover:text-red-400 disabled:opacity-50"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {kind === 'margin' && (
          <table className="w-full">
            <thead className="bg-white/5">
              <tr>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Pair</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Mode</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">LTV</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Liq. Threshold</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Max Leverage</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {margin.map((listing) => (
                <tr key={listing.id} className="border-t border-white/5">
                  <td className="px-6 py-4 text-white font-medium">{listing.pair}</td>
                  <td className="px-6 py-4 text-gray-400">{listing.margin_mode}</td>
                  <td className="px-6 py-4 text-white">{(listing.ltv * 100).toFixed(0)}%</td>
                  <td className="px-6 py-4 text-gray-400">{(listing.liquidation_threshold * 100).toFixed(0)}%</td>
                  <td className="px-6 py-4 text-white">{listing.max_leverage}x</td>
                  <td className="px-6 py-4">
                    <StatusCell status={listing.status} busy={busyId === listing.id} onToggle={() => void toggleStatus(listing.id, listing.status)} />
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => void remove(listing.id)}
                      disabled={busyId === listing.id}
                      className="p-2 text-gray-400 hover:text-red-400 disabled:opacity-50"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {!loading && futures.length === 0 && perps.length === 0 && margin.length === 0 && (
          <p className="px-6 py-8 text-gray-400 text-center">No listings yet.</p>
        )}
      </div>
    </div>
  );
}

function StatusCell({ status, busy, onToggle }: { status: string; busy: boolean; onToggle: () => void }) {
  const active = status === 'active';
  return (
    <button onClick={onToggle} disabled={busy} className={`${active ? 'text-green-400' : 'text-red-400'} disabled:opacity-50`}>
      {active ? <ToggleRight className="w-6 h-6" /> : <ToggleLeft className="w-6 h-6" />}
    </button>
  );
}

interface CreateListingFormProps {
  kind: ListingKind;
  onCreated: () => void | Promise<void>;
  onError: (message: string) => void;
}

function CreateListingForm({ kind, onCreated, onError }: CreateListingFormProps) {
  const [symbolValue, setSymbolValue] = useState('');
  const [contractType, setContractType] = useState(CONTRACT_TYPES[0]);
  const [settlement, setSettlement] = useState(SETTLEMENTS[0]);
  const [maxLeverage, setMaxLeverage] = useState('100');
  const [fundingRate, setFundingRate] = useState('0.0001');
  const [fundingInterval, setFundingInterval] = useState('8');
  const [maintenanceMargin, setMaintenanceMargin] = useState('0.005');
  const [initialMargin, setInitialMargin] = useState('0.01');
  const [liquidationFee, setLiquidationFee] = useState('0.005');
  const [insuranceShare, setInsuranceShare] = useState('0.1');
  const [takerFee, setTakerFee] = useState('0.0002');
  const [marketType, setMarketType] = useState(MARKET_TYPES[0]);
  const [collateral, setCollateral] = useState('RUSD,USDT,USDC');
  const [maxOpenInterest, setMaxOpenInterest] = useState('1000000');
  const [marginMode, setMarginMode] = useState(MARGIN_MODES[0]);
  const [ltv, setLtv] = useState('0.7');
  const [liquidationThreshold, setLiquidationThreshold] = useState('0.75');
  const [liquidationBonus, setLiquidationBonus] = useState('0.05');
  const [closeFactor, setCloseFactor] = useState('0.5');
  const [riskTier, setRiskTier] = useState(RISK_TIERS[1]);
  const [submitting, setSubmitting] = useState(false);

  const submit = async () => {
    setSubmitting(true);
    try {
      const parsedSymbol = parseSymbol(symbolValue, 'symbol');
      const leverage = parseInteger(maxLeverage, 'max_leverage', { min: 1, max: 1001 });

      if (kind === 'futures') {
        const payload: FuturesMarketInput = {
          symbol: parsedSymbol,
          contract_type: contractType,
          settlement,
          max_leverage: leverage,
          funding_rate: parseDecimal(fundingRate, 'funding_rate'),
          funding_interval_hours: parseInteger(fundingInterval, 'funding_interval_hours', { min: 1, max: 24 }),
          maintenance_margin_rate: parseRatio(maintenanceMargin, 'maintenance_margin_rate'),
          initial_margin_rate: parseRatio(initialMargin, 'initial_margin_rate'),
          liquidation_fee_rate: parseRatio(liquidationFee, 'liquidation_fee_rate'),
          insurance_fund_share: parseRatio(insuranceShare, 'insurance_fund_share'),
          taker_fee_rate: parseRatio(takerFee, 'taker_fee_rate'),
        };
        await endpoints.createFuturesMarket(payload);
      } else if (kind === 'perps') {
        const payload: PerpsListingInput = {
          symbol: parsedSymbol,
          market_type: marketType,
          collateral_assets: collateral,
          max_open_interest: maxOpenInterest,
          max_leverage: leverage,
          taker_fee_rate: parseRatio(takerFee, 'taker_fee_rate'),
          maintenance_margin_rate: parseRatio(maintenanceMargin, 'maintenance_margin_rate'),
          liquidation_fee_rate: parseRatio(liquidationFee, 'liquidation_fee_rate'),
        };
        await endpoints.createPerpsListing(payload);
      } else {
        const payload: MarginListingInput = {
          pair: parsedSymbol,
          margin_mode: marginMode,
          max_leverage: leverage,
          collateral_assets: collateral,
          ltv: parseRatio(ltv, 'ltv'),
          liquidation_threshold: parseRatio(liquidationThreshold, 'liquidation_threshold'),
          liquidation_bonus: parseRatio(liquidationBonus, 'liquidation_bonus'),
          close_factor: parseRatio(closeFactor, 'close_factor'),
          risk_tier: riskTier,
        };
        await endpoints.createMarginListing(payload);
      }

      await onCreated();
    } catch (submitError) {
      onError(errorMessage(submitError));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="bg-white/5 rounded-2xl p-6 mb-6">
      <h3 className="text-lg font-bold text-white mb-4 capitalize">New {kind} Listing</h3>
      <div className="grid md:grid-cols-3 gap-4">
        <Field label="Symbol (BASE/QUOTE)">
          <input value={symbolValue} onChange={(e) => setSymbolValue(e.target.value)} placeholder="BTC/RUSD" className={inputClass} />
        </Field>

        {kind === 'futures' && (
          <>
            <Field label="Contract Type">
              <select value={contractType} onChange={(e) => setContractType(e.target.value)} className={inputClass}>
                {CONTRACT_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </Field>
            <Field label="Settlement">
              <select value={settlement} onChange={(e) => setSettlement(e.target.value)} className={inputClass}>
                {SETTLEMENTS.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </Field>
            <Field label="Funding Rate">
              <input value={fundingRate} onChange={(e) => setFundingRate(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Funding Interval (h)">
              <input value={fundingInterval} onChange={(e) => setFundingInterval(e.target.value)} className={inputClass} />
            </Field>
          </>
        )}

        {kind === 'perps' && (
          <>
            <Field label="Market Type">
              <select value={marketType} onChange={(e) => setMarketType(e.target.value)} className={inputClass}>
                {MARKET_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </Field>
            <Field label="Max Open Interest">
              <input value={maxOpenInterest} onChange={(e) => setMaxOpenInterest(e.target.value)} className={inputClass} />
            </Field>
          </>
        )}

        {kind === 'margin' && (
          <>
            <Field label="Margin Mode">
              <select value={marginMode} onChange={(e) => setMarginMode(e.target.value)} className={inputClass}>
                {MARGIN_MODES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </Field>
            <Field label="LTV">
              <input value={ltv} onChange={(e) => setLtv(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Liquidation Threshold">
              <input value={liquidationThreshold} onChange={(e) => setLiquidationThreshold(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Liquidation Bonus">
              <input value={liquidationBonus} onChange={(e) => setLiquidationBonus(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Close Factor">
              <input value={closeFactor} onChange={(e) => setCloseFactor(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Risk Tier">
              <select value={riskTier} onChange={(e) => setRiskTier(e.target.value)} className={inputClass}>
                {RISK_TIERS.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </Field>
          </>
        )}

        <Field label="Max Leverage">
          <input value={maxLeverage} onChange={(e) => setMaxLeverage(e.target.value)} className={inputClass} />
        </Field>
        <Field label="Taker Fee (ratio)">
          <input value={takerFee} onChange={(e) => setTakerFee(e.target.value)} className={inputClass} />
        </Field>

        {kind !== 'margin' && (
          <>
            <Field label="Maintenance Margin">
              <input value={maintenanceMargin} onChange={(e) => setMaintenanceMargin(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Liquidation Fee">
              <input value={liquidationFee} onChange={(e) => setLiquidationFee(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Collateral Assets">
              <input value={collateral} onChange={(e) => setCollateral(e.target.value)} className={inputClass} />
            </Field>
          </>
        )}

        {kind === 'futures' && (
          <>
            <Field label="Initial Margin">
              <input value={initialMargin} onChange={(e) => setInitialMargin(e.target.value)} className={inputClass} />
            </Field>
            <Field label="Insurance Fund Share">
              <input value={insuranceShare} onChange={(e) => setInsuranceShare(e.target.value)} className={inputClass} />
            </Field>
          </>
        )}
      </div>

      <div className="flex items-center space-x-3 mt-6">
        <button
          onClick={() => void submit()}
          disabled={submitting}
          className="px-5 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors disabled:opacity-50"
        >
          {submitting ? 'Creating…' : 'Create Listing'}
        </button>
        <p className="text-gray-400 text-sm">New listings are created paused; activate them when ready.</p>
      </div>
    </div>
  );
}

const inputClass =
  'w-full px-3 py-2 bg-white/10 border border-white/20 rounded-lg text-white focus:outline-none focus:border-tiger-orange';

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-gray-400 text-sm mb-1">{label}</span>
      {children}
    </label>
  );
}
