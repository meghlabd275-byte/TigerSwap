'use client';

/**
 * OTC and TWAMM panel.
 *
 * OTC publishes a private block trade that never touches the public book, so a
 * large order produces no price impact. TWAMM slices an order across its
 * lifetime, so there is no single transaction to sandwich. Both post to the
 * API; the settlement price and any fill quantity come back from the server.
 */

import { useState } from 'react';
import { ArrowRightLeft, Clock, Loader2 } from 'lucide-react';

import {
  useCancelOtcOrder,
  useCancelTwammOrder,
  useCreateOtcOrder,
  useCreateTwammOrder,
  useOpenOtcOrders,
  useOtcOrders,
  useTakeOtcOrder,
  useTwammOrders,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';

const MAX_OTC_EXPIRY_HOURS = 24 * 30;

/** Bounds the server enforces on a TWAMM schedule, mirrored for early feedback. */
const MAX_TWAMM_DURATION_HOURS = 180 * 24;

export function OtcTwamm() {
  const [otcSymbol, setOtcSymbol] = useState('BTC/RUSD');
  const [otcSide, setOtcSide] = useState<'buy' | 'sell'>('buy');
  const [otcQuantity, setOtcQuantity] = useState('');
  const [otcPrice, setOtcPrice] = useState('');
  const [otcMinFill, setOtcMinFill] = useState('');
  const [otcExpiryHours, setOtcExpiryHours] = useState('24');
  const [takeQuantity, setTakeQuantity] = useState('');

  const [twammSymbol, setTwammSymbol] = useState('BTC/RUSD');
  const [twammSide, setTwammSide] = useState<'buy' | 'sell'>('buy');
  const [twammQuantity, setTwammQuantity] = useState('');
  const [twammLimit, setTwammLimit] = useState('0');
  const [twammDuration, setTwammDuration] = useState('24');
  const [twammDelay, setTwammDelay] = useState('0');

  const [formError, setFormError] = useState<string | null>(null);

  const authed = isAuthenticated();
  const myOtcQuery = useOtcOrders();
  const openOtcQuery = useOpenOtcOrders();
  const twammQuery = useTwammOrders();
  const createOtc = useCreateOtcOrder();
  const takeOtc = useTakeOtcOrder();
  const cancelOtc = useCancelOtcOrder();
  const createTwamm = useCreateTwammOrder();
  const cancelTwamm = useCancelTwammOrder();

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

  const handleCreateTwamm = () => {
    setFormError(null);
    if (!authed) {
      setFormError('Sign in to schedule a TWAMM order.');
      return;
    }
    const hours = Number(twammDuration);
    if (!Number.isFinite(hours) || hours < 1 || hours > MAX_TWAMM_DURATION_HOURS) {
      setFormError(`Duration must be between 1 and ${MAX_TWAMM_DURATION_HOURS} hours.`);
      return;
    }
    createTwamm.mutate(
      {
        symbol: twammSymbol.trim(),
        side: twammSide,
        total_quantity: twammQuantity.trim(),
        limit_price: twammLimit.trim() || '0',
        duration_hours: Math.round(hours),
        start_delay_seconds: Number(twammDelay) || 0,
      },
      {
        onSuccess: () => setTwammQuantity(''),
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
        <h2 className="text-lg font-display font-bold text-white">OTC &amp; TWAMM</h2>
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
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <select
            aria-label="OTC side"
            value={otcSide}
            onChange={(event) => setOtcSide(event.target.value as 'buy' | 'sell')}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
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
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="OTC price"
            type="text"
            inputMode="decimal"
            placeholder="Price"
            value={otcPrice}
            onChange={(event) => setOtcPrice(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="OTC minimum fill"
            type="text"
            inputMode="decimal"
            placeholder="Minimum fill (optional)"
            value={otcMinFill}
            onChange={(event) => setOtcMinFill(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="OTC expiry in hours"
            type="number"
            min={1}
            max={MAX_OTC_EXPIRY_HOURS}
            value={otcExpiryHours}
            onChange={(event) => setOtcExpiryHours(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
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

      {/* TWAMM */}
      <section className="border-t border-white/10 pt-4">
        <h3 className="text-sm font-semibold text-white mb-2 flex items-center space-x-2">
          <Clock className="w-4 h-4 text-tiger-orange" />
          <span>TWAMM schedule</span>
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <input
            aria-label="TWAMM market"
            type="text"
            value={twammSymbol}
            onChange={(event) => setTwammSymbol(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <select
            aria-label="TWAMM side"
            value={twammSide}
            onChange={(event) => setTwammSide(event.target.value as 'buy' | 'sell')}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          >
            <option value="buy" className="bg-gray-900">
              buy
            </option>
            <option value="sell" className="bg-gray-900">
              sell
            </option>
          </select>
          <input
            aria-label="TWAMM total quantity"
            type="text"
            inputMode="decimal"
            placeholder="Total quantity"
            value={twammQuantity}
            onChange={(event) => setTwammQuantity(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="TWAMM limit price"
            type="text"
            inputMode="decimal"
            placeholder="Limit price (0 = none)"
            value={twammLimit}
            onChange={(event) => setTwammLimit(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="TWAMM duration in hours"
            type="number"
            min={1}
            value={twammDuration}
            onChange={(event) => setTwammDuration(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <input
            aria-label="TWAMM start delay in seconds"
            type="number"
            min={0}
            value={twammDelay}
            onChange={(event) => setTwammDelay(event.target.value)}
            className="bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <button
          onClick={handleCreateTwamm}
          disabled={createTwamm.isPending}
          className="mt-3 px-4 py-2 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
        >
          {createTwamm.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Schedule'}
        </button>

        {twammQuery.error ? (
          <p className="text-red-400 text-xs mt-3" role="alert">
            {twammQuery.error.message}
          </p>
        ) : (twammQuery.data ?? []).length === 0 ? (
          <p className="text-gray-500 text-xs mt-3">
            {twammQuery.isPending ? 'Loading…' : 'No schedules yet.'}
          </p>
        ) : (
          <ul className="space-y-2 mt-3">
            {(twammQuery.data ?? []).map((order) => (
              <li
                key={order.id}
                className="bg-white/5 rounded-xl p-3 flex items-center justify-between"
              >
                <span className="text-xs text-gray-300">
                  {order.symbol} {order.side} {order.filled_quantity}/{order.total_quantity} ·{' '}
                  {order.status}
                </span>
                {order.status === 'active' && (
                  <button
                    onClick={() => cancelTwamm.mutate(order.id)}
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
