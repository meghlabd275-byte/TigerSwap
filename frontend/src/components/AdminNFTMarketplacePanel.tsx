'use client';

/**
 * Admin panel for the self-built NFT marketplace.
 *
 * An operator creates a collection, mints tokens into an account, watches every
 * listing and sets the marketplace fee. A new collection starts paused; minting
 * and listing are only possible once it is activated. Every action goes through
 * a real admin endpoint and a failure is shown rather than swallowed.
 */

import { useState } from 'react';
import { AlertTriangle, Gem, Loader2, Plus, Trash2 } from 'lucide-react';

import {
  useAllNFTCollections,
  useAllNFTListings,
  useCreateNFTCollection,
  useDeleteNFTCollection,
  useMintNFT,
  useNFTConfig,
  useUpdateNFTCollection,
  useUpdateNFTConfig,
} from '@/hooks/useApi';

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
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function AdminNFTMarketplacePanel() {
  const collectionsQuery = useAllNFTCollections();
  const listingsQuery = useAllNFTListings();
  const configQuery = useNFTConfig();

  const createCollection = useCreateNFTCollection();
  const updateCollection = useUpdateNFTCollection();
  const deleteCollection = useDeleteNFTCollection();
  const mint = useMintNFT();
  const updateConfig = useUpdateNFTConfig();

  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const [collectionForm, setCollectionForm] = useState({
    key: '',
    name: '',
    symbol: '',
    chain_id: '1',
    contract_address: '',
    base_uri: '',
    status: 'paused',
  });

  const [mintForm, setMintForm] = useState({
    collection_id: '',
    uri: '',
    owner: '',
    token_id: '',
  });

  const [feeBps, setFeeBps] = useState('');
  const [feeAccount, setFeeAccount] = useState('');

  const collections = collectionsQuery.data ?? [];
  const listings = listingsQuery.data ?? [];
  const config = configQuery.data;

  const run = (action: { mutate: (arg: never, options: { onSuccess: () => void; onError: (error: Error) => void }) => void }, arg: unknown, successText: string) => {
    setError(null);
    setOk(null);
    action.mutate(arg as never, {
      onSuccess: () => setOk(successText),
      onError: (err) => setError(err.message),
    });
  };

  if (collectionsQuery.isPending || listingsQuery.isPending || configQuery.isPending) {
    return (
      <div className="bg-white/5 rounded-2xl p-12 text-center">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin text-tiger-orange" />
        <p className="text-gray-400">Loading the NFT marketplace…</p>
      </div>
    );
  }

  if (collectionsQuery.error || listingsQuery.error || configQuery.error) {
    return (
      <div className="bg-white/5 rounded-2xl p-6">
        <div className="flex items-center gap-3 text-red-400" role="alert">
          <AlertTriangle className="w-5 h-5" />
          <p>{(collectionsQuery.error ?? listingsQuery.error ?? configQuery.error)?.message}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {error && (
        <div className="rounded-xl p-3 text-sm bg-red-500/10 text-red-400 border border-red-500/20" role="alert">
          {error}
        </div>
      )}
      {ok && (
        <div className="rounded-xl p-3 text-sm bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          {ok}
        </div>
      )}

      <div className="grid md:grid-cols-2 gap-6">
        {/* Create a collection */}
        <div className="bg-white/5 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <Plus className="w-5 h-5 text-tiger-orange" />
            <h3 className="font-bold text-white">Create a collection</h3>
          </div>
          <div className="space-y-3">
            {(
              [
                ['key', 'Key', 'text'],
                ['name', 'Name', 'text'],
                ['symbol', 'Symbol', 'text'],
                ['chain_id', 'Chain ID', 'number'],
                ['contract_address', 'Contract address (optional)', 'text'],
                ['base_uri', 'Base URI (optional)', 'text'],
              ] as const
            ).map(([field, label, type]) => (
              <div key={field}>
                <label className="text-gray-400 text-xs block mb-1" htmlFor={`nft-${field}`}>
                  {label}
                </label>
                <input
                  id={`nft-${field}`}
                  type={type}
                  value={collectionForm[field]}
                  onChange={(event) =>
                    setCollectionForm((current) => ({ ...current, [field]: event.target.value }))
                  }
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
                />
              </div>
            ))}
            <button
              onClick={() =>
                run(
                  createCollection,
                  {
                    key: collectionForm.key,
                    name: collectionForm.name,
                    symbol: collectionForm.symbol,
                    chain_id: Number(collectionForm.chain_id),
                    contract_address: collectionForm.contract_address || undefined,
                    base_uri: collectionForm.base_uri || undefined,
                    status: collectionForm.status,
                  },
                  'Collection created. Activate it to mint tokens.',
                )
              }
              disabled={
                createCollection.isPending ||
                collectionForm.key.trim() === '' ||
                collectionForm.name.trim() === '' ||
                collectionForm.symbol.trim() === ''
              }
              className="w-full py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
            >
              {createCollection.isPending ? 'Creating…' : 'Create collection'}
            </button>
          </div>
        </div>

        {/* Mint a token */}
        <div className="bg-white/5 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <Gem className="w-5 h-5 text-tiger-orange" />
            <h3 className="font-bold text-white">Mint a token</h3>
          </div>
          <div className="space-y-3">
            <div>
              <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-mint-collection">
                Collection
              </label>
              <select
                id="nft-mint-collection"
                value={mintForm.collection_id}
                onChange={(event) => setMintForm((current) => ({ ...current, collection_id: event.target.value }))}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
              >
                <option value="">Select a collection</option>
                {collections.map((collection) => (
                  <option key={collection.id} value={collection.id}>
                    {collection.name} ({collection.status})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-mint-uri">
                Token URI
              </label>
              <input
                id="nft-mint-uri"
                value={mintForm.uri}
                onChange={(event) => setMintForm((current) => ({ ...current, uri: event.target.value }))}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
              />
            </div>
            <div>
              <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-mint-owner">
                Owner account (optional)
              </label>
              <input
                id="nft-mint-owner"
                value={mintForm.owner}
                onChange={(event) => setMintForm((current) => ({ ...current, owner: event.target.value }))}
                placeholder="defaults to the operator"
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
              />
            </div>
            <div>
              <label className="text-gray-400 text-xs block mb-1" htmlFor="nft-mint-token-id">
                Token ID (optional)
              </label>
              <input
                id="nft-mint-token-id"
                value={mintForm.token_id}
                onChange={(event) => setMintForm((current) => ({ ...current, token_id: event.target.value }))}
                inputMode="numeric"
                placeholder="auto-incremented when blank"
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
              />
            </div>
            <button
              onClick={() =>
                run(
                  mint,
                  {
                    collectionId: mintForm.collection_id,
                    payload: {
                      uri: mintForm.uri,
                      owner: mintForm.owner || undefined,
                      token_id: mintForm.token_id === '' ? undefined : Number(mintForm.token_id),
                    },
                  },
                  'Token minted.',
                )
              }
              disabled={mint.isPending || mintForm.collection_id === '' || mintForm.uri.trim() === ''}
              className="w-full py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
            >
              {mint.isPending ? 'Minting…' : 'Mint token'}
            </button>
          </div>
        </div>
      </div>

      {/* Marketplace fee */}
      <div className="bg-white/5 rounded-2xl p-6">
        <h3 className="font-bold text-white mb-4">Marketplace fee</h3>
        <p className="text-gray-400 text-sm mb-4">
          Current fee {config ? (config.fee_bps / 100).toFixed(2) : '—'}% (max{' '}
          {config ? (config.max_fee_bps / 100).toFixed(2) : '—'}%). A listing or offer freezes the fee at creation, so a
          change applies only to new listings and offers.
        </p>
        <div className="grid md:grid-cols-3 gap-3">
          <input
            value={feeBps}
            onChange={(event) => setFeeBps(event.target.value)}
            inputMode="numeric"
            placeholder={`fee bps (e.g. ${config?.fee_bps ?? 250})`}
            className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
          />
          <input
            value={feeAccount}
            onChange={(event) => setFeeAccount(event.target.value)}
            placeholder="fee account (optional)"
            className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
          <button
            onClick={() =>
              run(
                updateConfig,
                {
                  fee_bps: feeBps === '' ? undefined : Number(feeBps),
                  fee_account: feeAccount || undefined,
                },
                'Marketplace fee updated.',
              )
            }
            disabled={updateConfig.isPending || (feeBps === '' && feeAccount === '')}
            className="py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
          >
            {updateConfig.isPending ? 'Saving…' : 'Update fee'}
          </button>
        </div>
      </div>

      {/* Collections */}
      <div className="bg-white/5 rounded-2xl p-6">
        <h3 className="font-bold text-white mb-4">Collections</h3>
        {collections.length === 0 ? (
          <p className="text-gray-400 text-sm">No collections yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-500 text-xs uppercase">
                  <th className="text-left py-2">Name</th>
                  <th className="text-left py-2">Symbol</th>
                  <th className="text-right py-2">Supply</th>
                  <th className="text-left py-2">Status</th>
                  <th className="text-right py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {collections.map((collection) => (
                  <tr key={collection.id} className="border-t border-white/5">
                    <td className="py-2 text-white">{collection.name}</td>
                    <td className="py-2 text-gray-300 font-mono">{collection.symbol}</td>
                    <td className="py-2 text-right text-gray-300 font-mono">{collection.total_supply}</td>
                    <td className="py-2">
                      <span
                        className={`px-2 py-0.5 rounded text-xs ${
                          collection.status === 'active'
                            ? 'bg-emerald-500/10 text-emerald-400'
                            : 'bg-amber-500/10 text-amber-400'
                        }`}
                      >
                        {collection.status}
                      </span>
                    </td>
                    <td className="py-2 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() =>
                            run(
                              updateCollection,
                              {
                                id: collection.id,
                                payload: { status: collection.status === 'active' ? 'paused' : 'active' },
                              },
                              'Collection status updated.',
                            )
                          }
                          disabled={updateCollection.isPending}
                          className="px-2 py-1 rounded bg-white/5 text-gray-300 text-xs hover:text-white disabled:opacity-40"
                        >
                          {collection.status === 'active' ? 'Pause' : 'Activate'}
                        </button>
                        <button
                          onClick={() => run(deleteCollection, collection.id, 'Collection deleted.')}
                          disabled={deleteCollection.isPending || collection.total_supply > 0}
                          title={collection.total_supply > 0 ? 'A collection with tokens cannot be deleted' : 'Delete'}
                          className="p-1.5 rounded bg-white/5 text-red-400 hover:text-red-300 disabled:opacity-40"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Listings */}
      <div className="bg-white/5 rounded-2xl p-6">
        <h3 className="font-bold text-white mb-4">Listings</h3>
        {listings.length === 0 ? (
          <p className="text-gray-400 text-sm">No listings yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-500 text-xs uppercase">
                  <th className="text-left py-2">Token</th>
                  <th className="text-left py-2">Seller</th>
                  <th className="text-right py-2">Price</th>
                  <th className="text-right py-2">Fee</th>
                  <th className="text-left py-2">Status</th>
                  <th className="text-left py-2">Deadline</th>
                </tr>
              </thead>
              <tbody>
                {listings.map((listing) => (
                  <tr key={listing.id} className="border-t border-white/5">
                    <td className="py-2 text-white font-mono">#{listing.token_id}</td>
                    <td className="py-2 text-gray-300 font-mono text-xs">{listing.seller_user_id}</td>
                    <td className="py-2 text-right text-gray-300 font-mono">
                      {formatAmount(listing.price)} {listing.payment_token_symbol}
                    </td>
                    <td className="py-2 text-right text-gray-400 font-mono">{(listing.fee_bps / 100).toFixed(2)}%</td>
                    <td className="py-2 text-gray-300">{listing.status}</td>
                    <td className="py-2 text-gray-400 text-xs">{formatDate(listing.deadline)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
