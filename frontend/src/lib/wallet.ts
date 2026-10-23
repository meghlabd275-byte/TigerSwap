/**
 * Wallet interaction for TigerSwap.
 *
 * The browser never holds a private key or a seed phrase. Every signing
 * operation is delegated to the user's own wallet (an injected EVM provider or
 * an EIP-1193 compatible wallet), so custody stays with the user and a
 * compromised page cannot drain funds.
 *
 * Reading balances is done through the wallet's own provider, which keeps the
 * app free of a third-party RPC dependency.
 */

import { ValidationError, address as addressValidator, hex, nonNegative, number, object, string } from "./validation";

/** The subset of EIP-1193 this module relies on. */
export interface Eip1193Provider {
  request(args: { method: string; params?: unknown[] | Record<string, unknown> }): Promise<unknown>;
  on?(event: string, handler: (...args: unknown[]) => void): void;
  removeListener?(event: string, handler: (...args: unknown[]) => void): void;
}

export interface ConnectedAccount {
  address: string;
  chainId: number;
}

export class WalletError extends Error {
  readonly code: string;

  constructor(message: string, code = "wallet_error") {
    super(message);
    this.name = "WalletError";
    this.code = code;
  }
}

declare global {
  interface Window {
    ethereum?: Eip1193Provider;
  }
}

/** Returns the injected provider, or null when no wallet is installed. */
export function getInjectedProvider(): Eip1193Provider | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.ethereum ?? null;
}

const accountsValidator = (value: unknown): string[] => {
  if (!Array.isArray(value) || value.length === 0) {
    throw new ValidationError("wallet returned no accounts");
  }
  return value.map((entry, index) => addressValidator(entry, `accounts[${index}]`));
};

/**
 * Requests account access and returns the first account.
 *
 * `eth_requestAccounts` is what prompts the user; the wallet decides whether
 * to approve. Nothing is stored beyond the returned address.
 */
export async function connectWallet(provider?: Eip1193Provider): Promise<ConnectedAccount> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected. Install an EVM wallet to continue.", "no_provider");
  }

  let accounts: string[];
  try {
    const raw = await active.request({ method: "eth_requestAccounts" });
    accounts = accountsValidator(raw);
  } catch (error) {
    if (error instanceof WalletError || error instanceof ValidationError) {
      throw error;
    }
    const message = error instanceof Error ? error.message : "wallet connection rejected";
    // The user declining is a normal outcome, not a bug.
    throw new WalletError(message, /denied|reject/i.test(message) ? "user_rejected" : "connect_failed");
  }

  const chainId = await getChainId(active);
  return { address: accounts[0], chainId };
}

/** Reads the current chain id from the provider. */
export async function getChainId(provider?: Eip1193Provider): Promise<number> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const raw = await active.request({ method: "eth_chainId" });
  const parsed = typeof raw === "string" ? Number.parseInt(raw, 16) : Number(raw);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new WalletError("wallet returned an invalid chain id", "invalid_chain");
  }
  return parsed;
}

/**
 * Asks the wallet to switch chains.
 *
 * If the chain is unknown to the wallet, `wallet_addEthereumChain` is called
 * with the supplied parameters, which the user must approve.
 */
export async function switchChain(
  chainId: number,
  addParameters?: {
    chainName: string;
    nativeCurrency: { name: string; symbol: string; decimals: number };
    rpcUrls: string[];
    blockExplorerUrls?: string[];
  },
  provider?: Eip1193Provider,
): Promise<void> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const hexChainId = `0x${chainId.toString(16)}`;

  try {
    await active.request({
      method: "wallet_switchEthereumChain",
      params: [{ chainId: hexChainId }],
    });
  } catch (error) {
    const code = (error as { code?: number })?.code;
    if (code === 4902 && addParameters) {
      await active.request({
        method: "wallet_addEthereumChain",
        params: [{ chainId: hexChainId, ...addParameters }],
      });
      return;
    }
    throw new WalletError(
      error instanceof Error ? error.message : "could not switch chain",
      "switch_failed",
    );
  }
}

const balanceValidator = (value: unknown): bigint => {
  const text = string(value, "balance");
  if (!/^0x[0-9a-fA-F]+$/.test(text)) {
    throw new ValidationError("balance is not a hex quantity", "balance");
  }
  return BigInt(text);
};

/** Reads the native balance of an address, in wei. */
export async function getNativeBalance(
  address: string,
  provider?: Eip1193Provider,
): Promise<bigint> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const validAddress = addressValidator(address);
  const raw = await active.request({
    method: "eth_getBalance",
    params: [validAddress, "latest"],
  });
  return balanceValidator(raw);
}

/**
 * Reads an ERC-20 balance.
 *
 * The `balanceOf(address)` selector is `0x70a08231`, so the calldata is that
 * selector followed by the 32-byte left-padded address.
 */
export async function getErc20Balance(
  tokenAddress: string,
  holder: string,
  provider?: Eip1193Provider,
): Promise<bigint> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const token = addressValidator(tokenAddress, "tokenAddress");
  const owner = addressValidator(holder, "holder");
  const data = `0x70a08231${owner.slice(2).toLowerCase().padStart(64, "0")}`;

  const raw = await active.request({
    method: "eth_call",
    params: [{ to: token, data }, "latest"],
  });

  const text = string(raw, "eth_call");
  if (!/^0x[0-9a-fA-F]*$/.test(text)) {
    throw new ValidationError("eth_call returned a non-hex value", "eth_call");
  }
  return text === "0x" ? 0n : BigInt(text);
}

/** Reads the ERC-20 decimals. Selector `0x313ce567`. */
export async function getErc20Decimals(
  tokenAddress: string,
  provider?: Eip1193Provider,
): Promise<number> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const token = addressValidator(tokenAddress, "tokenAddress");
  const raw = await active.request({
    method: "eth_call",
    params: [{ to: token, data: "0x313ce567" }, "latest"],
  });

  const text = string(raw, "decimals");
  const parsed = Number.parseInt(text, 16);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 36) {
    throw new ValidationError("token reported implausible decimals", "decimals");
  }
  return parsed;
}

/**
 * Sends a transaction through the wallet and returns the transaction hash.
 *
 * The wallet signs and broadcasts; the page only constructs the request.
 */
export async function sendTransaction(
  transaction: { to: string; data?: string; value?: string; gas?: string },
  provider?: Eip1193Provider,
): Promise<string> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const request: Record<string, string> = {
    to: addressValidator(transaction.to, "to"),
  };

  if (transaction.data !== undefined) {
    request.data = hex(transaction.data, "data");
  }
  if (transaction.value !== undefined) {
    request.value = hex(transaction.value, "value");
  }
  if (transaction.gas !== undefined) {
    request.gas = hex(transaction.gas, "gas");
  }

  const raw = await active.request({ method: "eth_sendTransaction", params: [request] });
  const hash = string(raw, "transactionHash");
  if (!/^0x[0-9a-fA-F]{64}$/.test(hash)) {
    throw new ValidationError("wallet returned an invalid transaction hash", "transactionHash");
  }
  return hash;
}

/**
 * Signs a plain-text message with `personal_sign`.
 *
 * The message is used for login (proving address ownership) and is never a
 * transaction. The returned signature stays in the browser.
 */
export async function signMessage(message: string, account: string, provider?: Eip1193Provider): Promise<string> {
  const active = provider ?? getInjectedProvider();
  if (!active) {
    throw new WalletError("No wallet detected", "no_provider");
  }

  const validAccount = addressValidator(account, "account");
  const raw = await active.request({
    method: "personal_sign",
    params: [message, validAccount],
  });

  const signature = string(raw, "signature");
  if (!/^0x[0-9a-fA-F]{130}$/.test(signature)) {
    throw new ValidationError("wallet returned an invalid signature", "signature");
  }
  return signature;
}

/** Formats a token amount given its decimals, for display. */
export function formatUnits(value: bigint, decimals: number): string {
  if (!Number.isInteger(decimals) || decimals < 0 || decimals > 36) {
    throw new ValidationError("decimals must be an integer between 0 and 36", "decimals");
  }

  const negative = value < 0n;
  const absolute = negative ? -value : value;
  const base = 10n ** BigInt(decimals);
  const whole = absolute / base;
  const fraction = absolute % base;

  if (fraction === 0n) {
    return `${negative ? "-" : ""}${whole.toString()}`;
  }

  const fractionText = fraction.toString().padStart(decimals, "0").replace(/0+$/, "");
  return `${negative ? "-" : ""}${whole.toString()}.${fractionText}`;
}

/** Parses a decimal string into base units, rejecting malformed input. */
export function parseUnits(value: string, decimals: number): bigint {
  if (!Number.isInteger(decimals) || decimals < 0 || decimals > 36) {
    throw new ValidationError("decimals must be an integer between 0 and 36", "decimals");
  }
  if (!/^\d+(\.\d+)?$/.test(value)) {
    throw new ValidationError("amount must be a positive decimal number", "amount");
  }

  const [whole, fraction = ""] = value.split(".");
  if (fraction.length > decimals) {
    throw new ValidationError(`amount has more than ${decimals} decimal places`, "amount");
  }

  const paddedFraction = fraction.padEnd(decimals, "0");
  return BigInt(whole) * 10n ** BigInt(decimals) + BigInt(paddedFraction || "0");
}

/** Shortens an address for display. */
export function formatAddress(value: string, leading = 6, trailing = 4): string {
  const valid = addressValidator(value);
  if (valid.length <= leading + trailing + 2) {
    return valid;
  }
  return `${valid.slice(0, leading + 2)}…${valid.slice(-trailing)}`;
}

/** Validates and normalises a chain id for display. */
export const chainIdValidator = nonNegative;
