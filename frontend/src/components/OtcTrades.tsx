'use client';

/**
 * OTC private block trades panel.
 *
 * An OTC order publishes a private block trade that never touches the public
 * book, so a large order produces no price impact. The maker's input is
 * escrowed from their ledger balance and a taker fills against it; the server
 * is the source of truth for the settlement price and any fill quantity.
 */

import { useState } from 'react';
import { ArrowRightLeft, Loader2 } from 'lucide-react';

import {
  useCancelOtcOrder,
  useCreateOtcOrder,
  useOpenOtcOrders,
  useOtcOrders,
  useTakeOtcOrder,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';

const MAX_OTC_EXPIRY_HOURS = 24 * 30;

export function OtcTrades() {
  const [otcSymbol, setOtcSymbol] = useState('BTC/RUSD');
  const [otcSide, setOtcSide] = useState<'buy' | 'sell'>('buy');
  const [otcQuantity, setOtcQuantity] = useState('');
  const [otcPrice, setOtcPrice] = useState('');
  const [otcMinFill, setOtcMinFill] = useState('');
  const [otcExpiryHours, setOtcExpiryHours] = useState('24');
  const [takeQuantity, setTakeQuantity] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const authed = isAuthenticated();
  const myOtcQuery = useOtcOrders();
  const openOtcQuery = useOpenOtcOrders();
  const createOtc = useCreateOtcOrder();
  const takeOtc = useTakeOtcOrder();
  const cancelOtc = useCancelOtcOrder();

  const inputClass =
    'bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange';

  const handleCreateOtc = () => {
    setFormError(null);
    if (!authed) {
      setFormError('Sign in to publish a block trade.');
      return;
    }
    const hours = Number(otcExpiryHours);
    if (!Number.isFinite(hours) || hours <= 0 || hours > MAX_OTC_EXPIRY_HOURS) {
      setFormError(`Expiry must be between 1 and ${MAX_OTC_EXPIRY_HOURS} hours.`);
      return;
    }
    createOtc.mutate(
      {
        symbol: otcSymbol.trim(),
        side: otcSide,
        quantity: otcQuantity.trim(),
        price: otcPrice.trim(),
        min_fill: otcMinFill.trim() || undefined,
        expires_in_seconds: Math.round(hours * 3600),
      },
      {
        onSuccess: () => {
          setOtcQuantity('');
          setOtcPrice('');
          setOtcMinFill('');
        },
        onError: (error) => setFormError(error.message),
      },
    );
  };

  const handleTake = (id: string) => {
    setFormError(null);
    if (!takeQuantity.trim()) {
      setFormError('Enter a quantity to fill.');
      return;
    }
    takeOtc.mutate(
      { id, quantity: takeQuantity.trim() },
      {
        onSuccess: () => setTakeQuantity(''),
        onError: (error) => setFormError(error.message),
      },
    );
  };

  return (
    <div className="glass-dark rounded-2xl p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-display font-bold text-white">OTC block trades</h2>
        <ArrowRightLeft className="w-5 h-5 text-tiger-orange" />
      </div>

      {formError && (
        <p className="text-red-400 text-xs" role="alert">
          {formError}
        </p>
      )}

      {/* OTC create */}
      <section>
        <h3 className="text-sm font-semibold text-white mb-2">Publish a block trade</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <input
            aria-label="OTC market"
            type="text"
            value={otcSymbol}
            onChange={(event) => setOtcSymbol(event.target.value)}
            className={inputClass}
          />
          <select
            aria-label="OTC side"
            value={otcSide}
            onChange={(event) => setOtcSide(event.target.value as 'buy' | 'sell')}
            className={inputClass}
          >
            <option value="buy" className="bg-gray-900">
              buy
            </option>
            <option value="sell" className="bg-gray-900">
              sell
            </option>
          </select>
          <input
            aria-label="OTC quantity"
            type="text"
            inputMode="decimal"
            placeholder="Quantity"
            value={otcQuantity}
            onChange={(event) => setOtcQuantity(event.target.value)}
            className={inputClass}
          />
          <input
            aria-label="OTC price"
            type="text"
            inputMode="decimal"
            placeholder="Price"
            value={otcPrice}
            onChange={(event) => setOtcPrice(event.target.value)}
            className={inputClass}
          />
          <input
            aria-label="OTC minimum fill"
            type="text"
            inputMode="decimal"
            placeholder="Minimum fill (optional)"
            value={otcMinFill}
            onChange={(event) => setOtcMinFill(event.target.value)}
            className={inputClass}
          />
          <input
            aria-label="OTC expiry in hours"
            type="number"
            min={1}
            max={MAX_OTC_EXPIRY_HOURS}
            value={otcExpiryHours}
            onChange={(event) => setOtcExpiryHours(event.target.value)}
            className={inputClass}
          />
        </div>
        <button
          onClick={handleCreateOtc}
          disabled={createOtc.isPending}
          className="mt-3 px-4 py-2 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
        >
          {createOtc.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Publish'}
        </button>
      </section>

      {/* OTC book */}
      <section>
        <h3 className="text-sm font-semibold text-white mb-2">Open block trades</h3>
        {openOtcQuery.error ? (
          <p className="text-red-400 text-xs" role="alert">
            {openOtcQuery.error.message}
          </p>
        ) : (openOtcQuery.data ?? []).length === 0 ? (
          <p className="text-gray-500 text-xs">
            {openOtcQuery.isPending ? 'Loading…' : 'No open block trades.'}
          </p>
        ) : (
          <ul className="space-y-2">
            {(openOtcQuery.data ?? []).map((order) => (
              <li
                key={order.id}
                className="bg-white/5 rounded-xl p-3 flex items-center justify-between"
              >
                <span className="text-xs text-gray-300">
                  {order.symbol} {order.side} {order.quantity} @ {order.price} · min{' '}
                  {order.min_fill}
                </span>
                <div className="flex items-center space-x-2">
                  <input
                    aria-label={`Quantity to fill for order ${order.id}`}
                    type="text"
                    inputMode="decimal"
                    placeholder="Quantity"
                    value={takeQuantity}
                    onChange={(event) => setTakeQuantity(event.target.value)}
                    className="w-24 bg-white/10 border border-white/20 rounded-lg px-2 py-1 text-white text-xs focus:outline-none focus:border-tiger-orange"
                  />
                  <button
                    onClick={() => handleTake(order.id)}
                    disabled={takeOtc.isPending}
                    className="px-3 py-1 rounded-lg text-xs bg-tiger-orange text-white disabled:bg-gray-700"
                  >
                    Take
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* My OTC orders */}
      <section>
        <h3 className="text-sm font-semibold text-white mb-2">My block trades</h3>
        {myOtcQuery.error ? (
          <p className="text-red-400 text-xs" role="alert">
            {myOtcQuery.error.message}
          </p>
        ) : (myOtcQuery.data ?? []).length === 0 ? (
          <p className="text-gray-500 text-xs">
            {myOtcQuery.isPending ? 'Loading…' : 'None yet.'}
          </p>
        ) : (
          <ul className="space-y-2">
            {(myOtcQuery.data ?? []).map((order) => (
              <li
                key={order.id}
                className="bg-white/5 rounded-xl p-3 flex items-center justify-between"
              >
                <span className="text-xs text-gray-300">
                  {order.symbol} {order.side} {order.quantity} @ {order.price} · {order.status}
                </span>
                {order.status === 'open' && (
                  <button
                    onClick={() => cancelOtc.mutate(order.id)}
                    className="px-3 py-1 rounded-lg text-xs bg-white/10 text-gray-200 hover:bg-white/20"
                  >
                    Cancel
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
