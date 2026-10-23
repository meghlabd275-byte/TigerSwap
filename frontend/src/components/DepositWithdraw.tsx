'use client';

/**
 * Deposit and withdraw panel.
 *
 * Deposits are shown as the user's own address with a real QR code rendered
 * locally. Withdrawals are sent as a real on-chain transaction signed by the
 * user's wallet, and the transaction hash is reported back.
 *
 * There is no fiat rail: this component does not invent bank details, and it
 * does not simulate a withdrawal.
 */

import { useEffect, useRef, useState } from 'react';
import { motion } from 'framer-motion';
import {
  Wallet,
  ArrowDownLeft,
  ArrowUpRight,
  Copy,
  Check,
  QrCode,
  ExternalLink,
  Loader2,
  AlertCircle,
} from 'lucide-react';
import QRCode from 'qrcode';

import { ValidationError, address as addressValidator } from '@/lib/validation';
import { formatAddress, parseUnits } from '@/lib/wallet';

export interface DepositWithdrawProps {
  currentWallet: {
    address: string;
    chainId: number;
    chainName: string;
  } | null;
  tokens: Array<{
    symbol: string;
    name: string;
    address: string;
    decimals: number;
    logoURI: string | null;
  }>;
  chainId: number;
  /** Explorer base URL for the active chain, used for transaction links. */
  explorerURL?: string | null;
  /** Sends a native-token transfer and resolves to the transaction hash. */
  onWithdraw: (to: string, amount: string) => Promise<string>;
  /** Optional: the user's available native balance, for validation. */
  availableBalance?: string;
}

export function DepositWithdraw({
  currentWallet,
  tokens,
  chainId,
  explorerURL,
  onWithdraw,
  availableBalance,
}: DepositWithdrawProps) {
  const [activeTab, setActiveTab] = useState<'deposit' | 'withdraw'>('deposit');
  const [selectedToken, setSelectedToken] = useState<DepositWithdrawProps['tokens'][number] | null>(
    tokens[0] ?? null,
  );
  const [withdrawAddress, setWithdrawAddress] = useState('');
  const [withdrawAmount, setWithdrawAmount] = useState('');
  const [showQR, setShowQR] = useState(false);
  const [copied, setCopied] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<string | null>(null);

  const qrContainerRef = useRef<HTMLDivElement | null>(null);

  const depositAddress = currentWallet?.address ?? '';

  // Render the QR code into the DOM when the panel is open. The value is the
  // user's own address, encoded locally, so nothing leaves the browser.
  useEffect(() => {
    if (!showQR || !depositAddress || !qrContainerRef.current) {
      return;
    }

    let cancelled = false;

    QRCode.toCanvas(depositAddress, { width: 192, margin: 1 }, (qrError, canvas) => {
      if (cancelled || qrError) {
        if (qrError) {
          setError('Could not render the QR code');
        }
        return;
      }
      const container = qrContainerRef.current;
      if (container) {
        container.replaceChildren(canvas);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [showQR, depositAddress]);

  if (!currentWallet) {
    return (
      <div className="bg-white/5 rounded-xl p-8 text-center">
        <Wallet className="w-16 h-16 text-gray-400 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Connect wallet</h3>
        <p className="text-gray-400">Connect your wallet to deposit or withdraw</p>
      </div>
    );
  }

  const copyAddress = async () => {
    try {
      await navigator.clipboard.writeText(depositAddress);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setError('Could not copy to the clipboard');
    }
  };

  const handleWithdraw = async () => {
    setError(null);
    setTxHash(null);

    let recipient: string;
    try {
      recipient = addressValidator(withdrawAddress, 'recipient');
    } catch (validationError) {
      setError(
        validationError instanceof ValidationError
          ? validationError.message
          : 'Enter a valid recipient address',
      );
      return;
    }

    const decimals = selectedToken?.decimals ?? 18;
    let amount: bigint;
    try {
      amount = parseUnits(withdrawAmount, decimals);
    } catch (validationError) {
      setError(
        validationError instanceof ValidationError ? validationError.message : 'Enter a valid amount',
      );
      return;
    }

    if (availableBalance !== undefined) {
      const available = Number(availableBalance);
      if (Number.isFinite(available) && Number(withdrawAmount) > available) {
        setError('Amount exceeds your available balance');
        return;
      }
    }

    setIsProcessing(true);
    try {
      const hash = await onWithdraw(recipient, withdrawAmount);
      setTxHash(hash);
      setWithdrawAddress('');
      setWithdrawAmount('');
    } catch (withdrawError) {
      setError(withdrawError instanceof Error ? withdrawError.message : 'Withdrawal failed');
    } finally {
      setIsProcessing(false);
    }
  };

  const explorerTxURL = explorerURL && txHash ? `${explorerURL}/tx/${txHash}` : null;

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-2 bg-white/5 rounded-xl p-1">
        <button
          onClick={() => setActiveTab('deposit')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-lg transition-colors ${
            activeTab === 'deposit' ? 'bg-tiger-orange text-white' : 'text-gray-400 hover:text-white'
          }`}
        >
          <ArrowDownLeft className="w-5 h-5" />
          <span>Deposit</span>
        </button>
        <button
          onClick={() => setActiveTab('withdraw')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-lg transition-colors ${
            activeTab === 'withdraw' ? 'bg-tiger-orange text-white' : 'text-gray-400 hover:text-white'
          }`}
        >
          <ArrowUpRight className="w-5 h-5" />
          <span>Withdraw</span>
        </button>
      </div>

      {error && (
        <div role="alert" className="flex items-start space-x-2 p-4 bg-red-500/10 border border-red-500/20 rounded-xl text-red-400 text-sm">
          <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {activeTab === 'deposit' && (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-6">
          <div className="bg-white/5 rounded-xl p-6">
            <h3 className="text-white font-semibold mb-4">Deposit on {currentWallet.chainName}</h3>

            <div className="mb-4">
              <label htmlFor="deposit-token" className="text-gray-400 text-sm mb-2 block">
                Token
              </label>
              <select
                id="deposit-token"
                value={selectedToken?.address ?? ''}
                onChange={(event) =>
                  setSelectedToken(tokens.find((token) => token.address === event.target.value) ?? null)
                }
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              >
                {tokens.map((token) => (
                  <option key={`${token.address}-${token.symbol}`} value={token.address} className="bg-gray-900">
                    {token.symbol} — {token.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="mb-4">
              <label htmlFor="deposit-address" className="text-gray-400 text-sm mb-2 block">
                Your deposit address
              </label>
              <div className="flex items-center space-x-2">
                <input
                  id="deposit-address"
                  readOnly
                  value={depositAddress}
                  className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white font-mono text-sm"
                />
                <button
                  onClick={() => void copyAddress()}
                  className="p-3 bg-white/10 border border-white/20 rounded-xl text-gray-400 hover:text-white transition-colors"
                  title="Copy address"
                >
                  {copied ? <Check className="w-5 h-5 text-green-400" /> : <Copy className="w-5 h-5" />}
                </button>
                <button
                  onClick={() => setShowQR((value) => !value)}
                  className="p-3 bg-white/10 border border-white/20 rounded-xl text-gray-400 hover:text-white transition-colors"
                  title="Show QR code"
                >
                  <QrCode className="w-5 h-5" />
                </button>
              </div>
            </div>

            {showQR && (
              <div className="mb-4 p-4 bg-white rounded-xl flex justify-center">
                <div ref={qrContainerRef} aria-label="Deposit address QR code" />
              </div>
            )}

            <div className="flex items-start space-x-3 p-4 bg-yellow-500/10 border border-yellow-500/20 rounded-xl">
              <AlertCircle className="w-5 h-5 text-yellow-400 shrink-0 mt-0.5" />
              <div className="text-sm text-yellow-400">
                <p className="font-medium mb-1">Send only on {currentWallet.chainName}</p>
                <p>
                  Assets sent on another network, or tokens other than the one selected, may be
                  permanently lost.
                </p>
              </div>
            </div>
          </div>
        </motion.div>
      )}

      {activeTab === 'withdraw' && (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-6">
          <div className="bg-white/5 rounded-xl p-6">
            <h3 className="text-white font-semibold mb-4">Withdraw from {currentWallet.chainName}</h3>

            <div className="mb-4">
              <label htmlFor="withdraw-token" className="text-gray-400 text-sm mb-2 block">
                Token
              </label>
              <select
                id="withdraw-token"
                value={selectedToken?.address ?? ''}
                onChange={(event) =>
                  setSelectedToken(tokens.find((token) => token.address === event.target.value) ?? null)
                }
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              >
                {tokens.map((token) => (
                  <option key={`${token.address}-${token.symbol}`} value={token.address} className="bg-gray-900">
                    {token.symbol} — {token.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="mb-4">
              <label htmlFor="withdraw-address" className="text-gray-400 text-sm mb-2 block">
                Recipient address
              </label>
              <input
                id="withdraw-address"
                type="text"
                value={withdrawAddress}
                onChange={(event) => setWithdrawAddress(event.target.value)}
                placeholder="0x…"
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white font-mono focus:outline-none focus:border-tiger-orange"
              />
            </div>

            <div className="mb-4">
              <label htmlFor="withdraw-amount" className="text-gray-400 text-sm mb-2 block">
                Amount
              </label>
              <input
                id="withdraw-amount"
                type="text"
                inputMode="decimal"
                value={withdrawAmount}
                onChange={(event) => setWithdrawAmount(event.target.value)}
                placeholder="0.00"
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              />
              {availableBalance !== undefined && (
                <p className="text-gray-500 text-xs mt-2">Available: {availableBalance}</p>
              )}
            </div>

            <p className="text-gray-500 text-xs mb-4">
              Network: {currentWallet.chainName} (chain ID {chainId}). Your wallet will ask you to
              confirm and will pay the network fee.
            </p>

            {txHash && (
              <div className="mb-4 bg-green-500/10 border border-green-500/20 rounded-xl p-4">
                <p className="text-green-400 text-sm mb-1">Withdrawal submitted</p>
                <p className="text-white font-mono text-xs break-all">{txHash}</p>
                {explorerTxURL && (
                  <a
                    href={explorerTxURL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-tiger-orange text-sm flex items-center space-x-1 mt-2"
                  >
                    <span>View on explorer</span>
                    <ExternalLink className="w-3 h-3" />
                  </a>
                )}
              </div>
            )}

            <button
              onClick={() => void handleWithdraw()}
              disabled={isProcessing || !withdrawAddress || !withdrawAmount}
              className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors flex items-center justify-center space-x-2"
            >
              {isProcessing ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  <span>Confirm in your wallet…</span>
                </>
              ) : (
                <>
                  <ArrowUpRight className="w-5 h-5" />
                  <span>Withdraw</span>
                </>
              )}
            </button>
          </div>
        </motion.div>
      )}
    </div>
  );
}
