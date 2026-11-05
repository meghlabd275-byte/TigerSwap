'use client';

/**
 * Self-built NFT marketplace panel.
 *
 * A collection is admin-managed; a token belongs to exactly one account. Listing
 * a token escrows it, and an offer escrows the bid, so a sale or an accepted
 * offer can never fail for a missing asset or balance. Every figure here comes
 * from the API: when a value is unavailable the panel shows a dash rather than a
 * fabricated one.
 */

import { useMemo, useState } from 'react';
import {
  AlertTriangle,
  Check,
  Gem,
  HandCoins,
  Loader2,
  Tag,
  Trash2,
} from 'lucide-react';

import {
  useAcceptNFTOffer,
  useBuyNFT,
  useCancelNFTListing,
  useCancelNFTOffer,
  useListNFT,
  useMakeNFTOffer,
  useMyNFTListings,
  useMyNFTOffers,
  useMyNFTs,
  useNFTConfig,
  useNFTListings,
  useTokens,
  useUpdateNFTListingPrice,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';
import type { NFTListingView, NFTToken } from '@/lib/types';

/** The settlement assets the platform ledger can move. */
const SUPPORTED_PAYMENT_SYMBOLS = ['RUSD', 'USDT', 'USDC', 'TGR', 'RYUSD'] as const;

function shortAddress(value: string, size = 6): string {
  if (value.length <= size * 2 + 3) {
    return value;
  }
  return `${value.slice(0, size)}…${value.slice(-size)}`;
}

/** Formats an exact integer string for display without losing precision. */
function formatAmount(raw: string): string {
  if (raw === '' || raw === undefined || raw === null) {
    return '—';
  }
  return BigInt(raw).toLocaleString();
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

/** The default deadline: 7 days from now, as an RFC3339 string. */
function defaultDeadline(): string {
  return new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();
}

type View = 'market' | 'mine';

export function NFTMarketplace() {
  const authed = isAuthenticated();
  const [view, setView] = useState<View>('market');
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);

  const listingsQuery = useNFTListings();
  const configQuery = useNFTConfig();
  const tokensQuery = useTokens();
  const myTokensQuery = useMyNFTs();
  const myListingsQuery = useMyNFTListings();
  const myOffersQuery = useMyNFTOffers();

  const buy = useBuyNFT();
  const listNFT = useListNFT();
  const cancelListing = useCancelNFTListing();
  const updatePrice = useUpdateNFTListingPrice();
  const makeOffer = useMakeNFTOffer();
  const acceptOffer = useAcceptNFTOffer();
  const cancelOffer = useCancelNFTOffer();

  const [paymentSymbol, setPaymentSymbol] = useState<string>('RUSD');
  const [listTokenKey, setListTokenKey] = useState<string>('');
  const [listPrice, setListPrice] = useState('');
  const [offerAmount, setOfferAmount] = useState('');
  const [offerTokenKey, setOfferTokenKey] = useState<string>('');
  const [newPrice, setNewPrice] = useState('');

  /** Payment tokens the ledger can settle, taken from the real token list. */
  const paymentTokens = useMemo(() => {
    const tokens = tokensQuery.data ?? [];
    return tokens.filter((token) =>
      (SUPPORTED_PAYMENT_SYMBOLS as readonly string[]).includes(token.symbol.toUpperCase()),
    );
  }, [tokensQuery.data]);

  const paymentToken = useMemo(
    () => paymentTokens.find((token) => token.symbol.toUpperCase() === paymentSymbol) ?? null,
    [paymentTokens, paymentSymbol],
  );

  const myTokens = myTokensQuery.data ?? [];
  const listings = listingsQuery.data ?? [];

  /** The selected token in the list form. */
  const selectedListToken: NFTToken | null = useMemo(() => {
    if (listTokenKey === '') {
      return null;
    }
    return myTokens.find((token) => `${token.collection_id}:${token.token_id}` === listTokenKey) ?? null;
  }, [myTokens, listTokenKey]);

  const selectedOfferToken: NFTToken | null = useMemo(() => {
    if (offerTokenKey === '') {
      return null;
    }
    return myTokens.find((token) => `${token.collection_id}:${token.token_id}` === offerTokenKey) ?? null;
  }, [myTokens, offerTokenKey]);

  const run = <T,>(
    action: { mutate: (arg: T, options: { onSuccess: () => void; onError: (error: Error) => void }) => void },
    arg: T,
    successText: string,
  ) => {
    setMessage(null);
    action.mutate(arg, {
      onSuccess: () => setMessage({ kind: 'ok', text: successText }),
      onError: (error) => setMessage({ kind: 'error', text: error.message }),
    });
  };

  if (listingsQuery.isPending || configQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-10 text-center">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin text-tiger-orange" />
        <p className="text-gray-400">Loading the NFT marketplace…</p>
      </div>
    );
  }

  if (listingsQuery.error || configQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center gap-3 text-red-400" role="alert">
          <AlertTriangle className="w-5 h-5" />
          <p>{(listingsQuery.error ?? configQuery.error)?.message}</p>
        </div>
      </div>
    );
  }

  const config = configQuery.data;

  return (
    <div className="space-y-6">
      {message && (
        <div
          role="alert"
          className={`rounded-xl p-3 text-sm ${
            message.kind === 'ok'
              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
              : 'bg-red-500/10 text-red-400 border border-red-500/20'
          }`}
        >
          {message.text}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Gem className="w-5 h-5 text-tiger-orange" />
          <h2 className="text-lg font-display font-bold text-white">NFT Marketplace</h2>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-400">
            Fee {config ? (config.fee_bps / 100).toFixed(2) : '—'}%
          </span>
          <div className="flex rounded-lg overflow-hidden border border-white/10">
            {(['market', 'mine'] as View[]).map((id) => (
              <button
                key={id}
                onClick={() => setView(id)}
                className={`px-4 py-1.5 text-sm transition ${
                  view === id ? 'bg-tiger-orange/20 text-tiger-orange' : 'text-gray-400 hover:text-white'
                }`}
              >
                {id === 'market' ? 'Marketplace' : 'My NFTs'}
              </button>
            ))}
          </div>
        </div>
      </div>

      {view === 'market' ? (
        <MarketView listings={listings} authed={authed} onBuy={(id) => run(buy, id, 'Bought the NFT.')} busy={buy.isPending} />
      ) : !authed ? (
        <div className="glass-dark rounded-2xl p-6 text-gray-400 text-sm">
          Sign in to manage your NFTs.
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* List a token */}
          <div className="glass-dark rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-4">
              <Tag className="w-5 h-5 text-tiger-orange" />
              <h3 className="font-display font-bold text-white">List a token</h3>
            </div>
            <div className="space-y-4">
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-list-token">
                  Token
                </label>
                <select
                  id="nft-list-token"
                  value={listTokenKey}
                  onChange={(event) => setListTokenKey(event.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
                >
                  <option value="">Select a token</option>
                  {myTokens.map((token) => (
                    <option key={`${token.collection_id}:${token.token_id}`} value={`${token.collection_id}:${token.token_id}`}>
                      #{token.token_id} · {shortAddress(token.collection_id, 4)}
                      {token.escrow_listing_id ? ' (escrowed)' : ''}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-payment">
                  Price in
                </label>
                <select
                  id="nft-payment"
                  value={paymentSymbol}
                  onChange={(event) => setPaymentSymbol(event.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
                >
                  {SUPPORTED_PAYMENT_SYMBOLS.map((symbol) => (
                    <option key={symbol} value={symbol}>
                      {symbol}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-price">
                  Price ({paymentSymbol}, smallest unit)
                </label>
                <input
                  id="nft-price"
                  value={listPrice}
                  onChange={(event) => setListPrice(event.target.value)}
                  inputMode="numeric"
                  placeholder="e.g. 100000000000000000000"
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <button
                onClick={() => {
                  if (!selectedListToken || !paymentToken) {
                    return;
                  }
                  run(
                    listNFT,
                    {
                      collection_id: selectedListToken.collection_id,
                      token_id: selectedListToken.token_id,
                      payment_token_address: paymentToken.address,
                      payment_token_symbol: paymentToken.symbol,
                      price: listPrice,
                      deadline: defaultDeadline(),
                    },
                    'Listed the token.',
                  );
                }}
                disabled={
                  listNFT.isPending ||
                  !selectedListToken ||
                  !paymentToken ||
                  listPrice.trim() === '' ||
                  selectedListToken.escrow_listing_id != null
                }
                className="w-full py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
              >
                {listNFT.isPending ? 'Listing…' : 'List'}
              </button>
              {!paymentToken && (
                <p className="text-xs text-amber-400">
                  No settlement token is available in the token list. An operator adds RUSD, USDT, USDC, TGR or RYUSD first.
                </p>
              )}
            </div>
          </div>

          {/* Make an offer */}
          <div className="glass-dark rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-4">
              <HandCoins className="w-5 h-5 text-tiger-orange" />
              <h3 className="font-display font-bold text-white">Make an offer</h3>
            </div>
            <div className="space-y-4">
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-offer-token">
                  Token
                </label>
                <select
                  id="nft-offer-token"
                  value={offerTokenKey}
                  onChange={(event) => setOfferTokenKey(event.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
                >
                  <option value="">Select a token</option>
                  {myTokens.map((token) => (
                    <option key={`${token.collection_id}:${token.token_id}`} value={`${token.collection_id}:${token.token_id}`}>
                      #{token.token_id} · {shortAddress(token.collection_id, 4)}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-offer-amount">
                  Amount ({paymentSymbol}, smallest unit)
                </label>
                <input
                  id="nft-offer-amount"
                  value={offerAmount}
                  onChange={(event) => setOfferAmount(event.target.value)}
                  inputMode="numeric"
                  placeholder="integer in smallest unit"
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <p className="text-xs text-gray-500">
                A bid is escrowed from your balance and refunded when you cancel it. It settles only if the seller accepts.
              </p>
              <button
                onClick={() => {
                  if (!selectedOfferToken || !paymentToken) {
                    return;
                  }
                  run(
                    makeOffer,
                    {
                      collection_id: selectedOfferToken.collection_id,
                      token_id: selectedOfferToken.token_id,
                      payment_token_address: paymentToken.address,
                      payment_token_symbol: paymentToken.symbol,
                      amount: offerAmount,
                      deadline: defaultDeadline(),
                    },
                    'Offer escrowed.',
                  );
                }}
                disabled={
                  makeOffer.isPending || !selectedOfferToken || !paymentToken || offerAmount.trim() === ''
                }
                className="w-full py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
              >
                {makeOffer.isPending ? 'Offering…' : 'Make offer'}
              </button>
            </div>
          </div>

          {/* My listings */}
          <div className="glass-dark rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-4">
              <Tag className="w-5 h-5 text-tiger-orange" />
              <h3 className="font-display font-bold text-white">My listings</h3>
            </div>
            {(myListingsQuery.data ?? []).length === 0 ? (
              <p className="text-gray-400 text-sm">You have no listings.</p>
            ) : (
              <ul className="space-y-3">
                {(myListingsQuery.data ?? []).map((listing) => (
                  <li key={listing.id} className="flex items-center justify-between gap-3 border-b border-white/5 pb-2">
                    <div className="text-sm">
                      <div className="text-white font-mono">
                        #{listing.token_id} · {formatAmount(listing.price)} {listing.payment_token_symbol}
                      </div>
                      <div className="text-gray-500 text-xs">
                        {listing.status} · {listing.fee_bps / 100}% fee · expires {formatDate(listing.deadline)}
                      </div>
                    </div>
                    {listing.status === 'active' && (
                      <div className="flex items-center gap-2">
                        <input
                          value={newPrice}
                          onChange={(event) => setNewPrice(event.target.value)}
                          inputMode="numeric"
                          placeholder="new price"
                          className="w-28 bg-white/5 border border-white/10 rounded-lg px-2 py-1 text-white text-xs font-mono focus:outline-none focus:border-tiger-orange"
                        />
                        <button
                          onClick={() => run(updatePrice, { id: listing.id, price: newPrice }, 'Price updated.')}
                          disabled={updatePrice.isPending || newPrice.trim() === ''}
                          className="p-1.5 rounded-lg bg-white/5 text-gray-300 hover:text-white disabled:opacity-40"
                          title="Update price"
                        >
                          <Check className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => run(cancelListing, listing.id, 'Listing cancelled.')}
                          disabled={cancelListing.isPending}
                          className="p-1.5 rounded-lg bg-white/5 text-red-400 hover:text-red-300 disabled:opacity-40"
                          title="Cancel listing"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* My offers */}
          <div className="glass-dark rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-4">
              <HandCoins className="w-5 h-5 text-tiger-orange" />
              <h3 className="font-display font-bold text-white">My offers</h3>
            </div>
            {(myOffersQuery.data ?? []).length === 0 ? (
              <p className="text-gray-400 text-sm">You have no offers.</p>
            ) : (
              <ul className="space-y-3">
                {(myOffersQuery.data ?? []).map((offer) => (
                  <li key={offer.id} className="flex items-center justify-between gap-3 border-b border-white/5 pb-2">
                    <div className="text-sm">
                      <div className="text-white font-mono">
                        #{offer.token_id} · {formatAmount(offer.amount)} {offer.payment_token_symbol}
                      </div>
                      <div className="text-gray-500 text-xs">
                        {offer.status} · expires {formatDate(offer.deadline)}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() =>
                          run(
                            acceptOffer,
                            { id: offer.id, collectionId: offer.collection_id, tokenId: offer.token_id },
                            'Offer accepted.',
                          )
                        }
                        disabled={acceptOffer.isPending}
                        className="px-3 py-1.5 rounded-lg bg-tiger-orange/20 text-tiger-orange text-xs font-semibold hover:bg-tiger-orange/30 disabled:opacity-40"
                      >
                        Accept
                      </button>
                      <button
                        onClick={() => run(cancelOffer, offer.id, 'Offer cancelled.')}
                        disabled={cancelOffer.isPending}
                        className="p-1.5 rounded-lg bg-white/5 text-red-400 hover:text-red-300 disabled:opacity-40"
                        title="Cancel offer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/** The public marketplace: live listings with a buy action. */
function MarketView({
  listings,
  authed,
  onBuy,
  busy,
}: {
  listings: NFTListingView[];
  authed: boolean;
  onBuy: (id: string) => void;
  busy: boolean;
}) {
  if (listings.length === 0) {
    return (
      <div className="glass-dark rounded-2xl p-6 text-gray-400 text-sm">
        No NFTs are listed. An owner lists a token from the “My NFTs” tab.
      </div>
    );
  }
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {listings.map((view) => (
        <div key={view.listing.id} className="glass-dark rounded-2xl p-5 flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <span className="text-white font-display font-bold">{view.collection.name}</span>
            <span className="text-gray-500 text-xs font-mono">#{view.token.token_id}</span>
          </div>
          <div className="text-sm text-gray-400 font-mono truncate">{view.token.uri || '—'}</div>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs text-gray-500">Price</div>
              <div className="text-white font-mono">
                {formatAmount(view.listing.price)} {view.listing.payment_token_symbol}
              </div>
            </div>
            <div className="text-right">
              <div className="text-xs text-gray-500">Expires</div>
              <div className="text-gray-300 text-xs">{formatDate(view.listing.deadline)}</div>
            </div>
          </div>
          <button
            onClick={() => onBuy(view.listing.id)}
            disabled={!authed || busy}
            className="mt-1 w-full py-2 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
          >
            {authed ? 'Buy' : 'Sign in to buy'}
          </button>
        </div>
      ))}
    </div>
  );
}
