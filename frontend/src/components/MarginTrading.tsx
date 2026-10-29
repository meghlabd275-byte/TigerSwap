'use client';

/**
 * Spot margin panel.
 *
 * A margin account is opened for a listed pair, collateral is deposited, and
 * the account's health factor is read back from the API. Collateral, borrow and
 * repay all post to the API, which values the account from TigerSwap's own
 * oracle; no price or health factor is computed in the browser.
 */

import { useState } from 'react';
import { AlertTriangle, Coins, Loader2, ShieldCheck } from 'lucide-react';

import {
  useBorrowMargin,
  useDepositMarginCollateral,
  useMarginAccount,
  useMarginMarkets,
  useOpenMarginAccount,
  useRepayMargin,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';

export function MarginTrading() {
  const [pair, setPair] = useState('');
  const [accountId, setAccountId] = useState<string | null>(null);
  const [collateralAsset, setCollateralAsset] = useState('BTC');
  const [collateralAmount, setCollateralAmount] = useState('');
  const [borrowAsset, setBorrowAsset] = useState('RUSD');
  const [borrowAmount, setBorrowAmount] = useState('');
  const [repayAmount, setRepayAmount] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const marketsQuery = useMarginMarkets();
  const markets = marketsQuery.data ?? [];
  const selectedPair = pair || markets[0]?.pair || '';

  const accountQuery = useMarginAccount(accountId);
  const openAccount = useOpenMarginAccount();
  const depositCollateral = useDepositMarginCollateral();
  const borrow = useBorrowMargin();
  const repay = useRepayMargin();

  const authed = isAuthenticated();

  const handleOpen = () => {
    setFormError(null);
    if (!authed) {
      setFormError('Sign in to open a margin account.');
      return;
    }
    if (!selectedPair) {
      setFormError('No margin market is available.');
      return;
    }
    openAccount.mutate(
      { pair: selectedPair },
      {
        onSuccess: (account) => setAccountId(account.id),
        onError: (error) => setFormError(error.message),
      },
    );
  };

  const runAction = (
    mutate: () => void,
    clear?: () => void,
  ) => {
    setFormError(null);
    if (!accountId) {
      setFormError('Open a margin account first.');
      return;
    }
    mutate();
    if (clear) clear();
  };

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-display font-bold text-white">Margin</h2>
        <Coins className="w-5 h-5 text-tiger-orange" />
      </div>

      {/* Open account */}
      <div className="mb-4">
        <label htmlFor="margin-pair" className="text-gray-400 text-sm mb-2 block">
          Market
        </label>
        <div className="flex space-x-2">
          <select
            id="margin-pair"
            value={selectedPair}
            onChange={(event) => setPair(event.target.value)}
            disabled={markets.length === 0}
            className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange disabled:text-gray-500"
          >
            {markets.length === 0 ? (
              <option value="" className="bg-gray-900">
                {marketsQuery.isPending ? 'Loading markets…' : 'No margin markets available'}
              </option>
            ) : (
              markets.map((market) => (
                <option key={market.pair} value={market.pair} className="bg-gray-900">
                  {market.pair} · {market.max_leverage}x
                </option>
              ))
            )}
          </select>
          <button
            onClick={handleOpen}
            disabled={openAccount.isPending || accountId !== null || !selectedPair}
            className="px-4 py-3 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
          >
            {openAccount.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Open account'}
          </button>
        </div>
      </div>

      {marketsQuery.error && (
        <p className="text-red-400 text-xs mb-3" role="alert">
          {marketsQuery.error.message}
        </p>
      )}

      {formError && (
        <p className="text-red-400 text-xs mb-3" role="alert">
          {formError}
        </p>
      )}

      {/* Health */}
      {accountId && (
        <div className="bg-white/5 rounded-xl p-3 mb-4">
          {accountQuery.isPending ? (
            <p className="text-gray-400 text-sm flex items-center space-x-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Loading account…</span>
            </p>
          ) : accountQuery.error ? (
            <p className="text-red-400 text-xs" role="alert">
              {accountQuery.error.message}
            </p>
          ) : accountQuery.data ? (
            <>
              <div className="flex items-center justify-between mb-2">
                <span className="text-gray-400 text-xs">Health factor</span>
                <span
                  className={`font-semibold ${
                    accountQuery.data.health_factor >= 1.5
                      ? 'text-green-400'
                      : accountQuery.data.health_factor >= 1
                        ? 'text-yellow-400'
                        : 'text-red-400'
                  }`}
                >
                  {Number.isFinite(accountQuery.data.health_factor)
                    ? accountQuery.data.health_factor.toFixed(3)
                    : '∞'}
                </span>
              </div>
              <dl className="grid grid-cols-3 gap-2 text-xs">
                <div>
                  <dt className="text-gray-500">Collateral</dt>
                  <dd className="text-white">
                    ${accountQuery.data.collateral_value_usd.toLocaleString()}
                  </dd>
                </div>
                <div>
                  <dt className="text-gray-500">Debt</dt>
                  <dd className="text-white">${accountQuery.data.debt_value_usd.toLocaleString()}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">Equity</dt>
                  <dd className="text-white">${accountQuery.data.equity_usd.toLocaleString()}</dd>
                </div>
              </dl>
            </>
          ) : null}
        </div>
      )}

      {/* Deposit collateral */}
      <div className="mb-4">
        <label htmlFor="margin-collateral" className="text-gray-400 text-sm mb-2 block">
          Deposit collateral
        </label>
        <div className="flex space-x-2">
          <input
            id="margin-collateral"
            type="text"
            inputMode="decimal"
            value={collateralAmount}
            onChange={(event) => setCollateralAmount(event.target.value)}
            placeholder="0.00"
            className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="Collateral asset"
            type="text"
            value={collateralAsset}
            onChange={(event) => setCollateralAsset(event.target.value.toUpperCase())}
            className="w-24 bg-white/10 border border-white/20 rounded-xl px-3 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
          <button
            onClick={() =>
              runAction(
                () =>
                  depositCollateral.mutate(
                    { account_id: accountId!, asset: collateralAsset, amount: collateralAmount },
                    { onError: (error) => setFormError(error.message) },
                  ),
                () => setCollateralAmount(''),
              )
            }
            disabled={!accountId || depositCollateral.isPending || !collateralAmount}
            className="px-4 py-3 rounded-xl font-medium bg-white/10 text-white disabled:bg-gray-700 disabled:text-gray-500"
          >
            Deposit
          </button>
        </div>
      </div>

      {/* Borrow */}
      <div className="mb-4">
        <label htmlFor="margin-borrow" className="text-gray-400 text-sm mb-2 block">
          Borrow
        </label>
        <div className="flex space-x-2">
          <input
            id="margin-borrow"
            type="text"
            inputMode="decimal"
            value={borrowAmount}
            onChange={(event) => setBorrowAmount(event.target.value)}
            placeholder="0.00"
            className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="Borrow asset"
            type="text"
            value={borrowAsset}
            onChange={(event) => setBorrowAsset(event.target.value.toUpperCase())}
            className="w-24 bg-white/10 border border-white/20 rounded-xl px-3 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
          <button
            onClick={() =>
              runAction(
                () =>
                  borrow.mutate(
                    { account_id: accountId!, asset: borrowAsset, amount: borrowAmount },
                    { onError: (error) => setFormError(error.message) },
                  ),
                () => setBorrowAmount(''),
              )
            }
            disabled={!accountId || borrow.isPending || !borrowAmount}
            className="px-4 py-3 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
          >
            Borrow
          </button>
        </div>
      </div>

      {/* Repay */}
      <div className="mb-4">
        <label htmlFor="margin-repay" className="text-gray-400 text-sm mb-2 block">
          Repay {borrowAsset}
        </label>
        <div className="flex space-x-2">
          <input
            id="margin-repay"
            type="text"
            inputMode="decimal"
            value={repayAmount}
            onChange={(event) => setRepayAmount(event.target.value)}
            placeholder="0.00"
            className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
          <button
            onClick={() =>
              runAction(
                () =>
                  repay.mutate(
                    { account_id: accountId!, asset: borrowAsset, amount: repayAmount },
                    { onError: (error) => setFormError(error.message) },
                  ),
                () => setRepayAmount(''),
              )
            }
            disabled={!accountId || repay.isPending || !repayAmount}
            className="px-4 py-3 rounded-xl font-medium bg-white/10 text-white disabled:bg-gray-700 disabled:text-gray-500"
          >
            Repay
          </button>
        </div>
      </div>

      <div className="flex items-start space-x-2 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-xl">
        <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-yellow-400 text-xs">
          Borrowing increases liquidation risk. Keep the health factor above 1 to avoid liquidation.
        </p>
      </div>

      {accountId && (
        <p className="flex items-center space-x-1 text-gray-500 text-xs mt-3">
          <ShieldCheck className="w-3 h-3" />
          <span>Account {accountId.slice(0, 8)}…</span>
        </p>
      )}
    </div>
  );
}
