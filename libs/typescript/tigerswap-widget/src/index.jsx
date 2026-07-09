/**
 * TigerSwap Widget SDK
 *
 * Embeddable swap widget for external websites
 * Similar to 1inch Widget, ParaSwap Widget
 *
 * @package tigerswap-widget
 * @version 1.0.0
 */
import React, { useState, useEffect, useCallback } from 'react';
// API Configuration
const API_BASE_URL = 'https://api.tigerswap.io/v1';
const WSS_URL = 'wss://stream.tigerswap.io/v1';
// API Client
class TigerSwapAPIClient {
    baseUrl;
    apiKey;
    constructor(baseUrl = API_BASE_URL) {
        this.baseUrl = baseUrl;
    }
    setApiKey(key) {
        this.apiKey = key;
    }
    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...(this.apiKey && { 'X-API-Key': this.apiKey }),
            ...options.headers,
        };
        const response = await fetch(`${this.baseUrl}${endpoint}`, {
            ...options,
            headers,
        });
        if (!response.ok) {
            const error = await response.json().catch(() => ({ message: 'Request failed' }));
            throw new Error(error.message || `HTTP ${response.status}`);
        }
        return response.json();
    }
    async getQuote(request) {
        return this.request('/quote', {
            method: 'POST',
            body: JSON.stringify(request),
        });
    }
    async getTokens(chainId) {
        return this.request(`/tokens/${chainId}`);
    }
    async getChains() {
        return this.request('/chains');
    }
    async buildSwap(request) {
        return this.request('/swap/build', {
            method: 'POST',
            body: JSON.stringify(request),
        });
    }
    async executeSwap(tx) {
        return this.request('/swap/execute', {
            method: 'POST',
            body: JSON.stringify({ tx }),
        });
    }
    async getGasPrice(chainId) {
        return this.request(`/gas-price/${chainId}`);
    }
    async getTokenAllowance(chainId, token, owner) {
        return this.request(`/allowance/${chainId}/${token}/${owner}`);
    }
}
// WebSocket Client for Real-time Updates
class TigerSwapWSClient {
    ws = null;
    reconnectAttempts = 0;
    maxReconnectAttempts = 5;
    messageHandlers = new Map();
    connect(onOpen) {
        this.ws = new WebSocket(WSS_URL);
        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            onOpen?.();
        };
        this.ws.onmessage = (event) => {
            try {
                const { type: msgType, data } = JSON.parse(event.data);
                const handler = this.messageHandlers.get(msgType);
                handler?.(data);
            }
            catch (e) {
                console.error('Failed to parse WS message:', e);
            }
        };
        this.ws.onclose = () => {
            if (this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                setTimeout(() => this.connect(), 1000 * this.reconnectAttempts);
            }
        };
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }
    subscribe(type, handler) {
        this.messageHandlers.set(type, handler);
        this.ws?.send(JSON.stringify({ action: 'subscribe', type }));
    }
    unsubscribe(type) {
        this.messageHandlers.delete(type);
        this.ws?.send(JSON.stringify({ action: 'unsubscribe', type }));
    }
    disconnect() {
        this.ws?.close();
        this.ws = null;
    }
    send(type, data) {
        this.ws?.send(JSON.stringify({ action: type, ...data }));
    }
}
// React Hook for Widget
export function useTigerSwapWidget(config, callbacks) {
    const [api] = useState(() => new TigerSwapAPIClient());
    const [ws] = useState(() => new TigerSwapWSClient());
    const [tokens, setTokens] = useState([]);
    const [quote, setQuote] = useState(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [wallet, setWallet] = useState({
        address: null,
        chainId: null,
        isConnected: false,
        balance: '0',
    });
    // Initialize
    useEffect(() => {
        if (config.enabledTokens) {
            setTokens(config.enabledTokens);
        }
        else if (config.defaultChainId) {
            api.getTokens(config.defaultChainId)
                .then(setTokens)
                .catch(console.error);
        }
        // Connect WebSocket for real-time updates
        ws.connect();
        return () => {
            ws.disconnect();
        };
    }, [config.defaultChainId, config.enabledTokens]);
    // Subscribe to price updates
    useEffect(() => {
        ws.subscribe('price-update', (data) => {
            callbacks?.onQuoteUpdate?.(data.quote);
        });
        return () => {
            ws.unsubscribe('price-update');
        };
    }, [ws, callbacks]);
    // Get quote
    const getQuote = useCallback(async (request) => {
        setLoading(true);
        setError(null);
        try {
            const result = await api.getQuote(request);
            setQuote(result);
            callbacks?.onQuoteUpdate?.(result);
            return result;
        }
        catch (e) {
            const err = e;
            setError(err);
            callbacks?.onError?.(err);
            throw err;
        }
        finally {
            setLoading(false);
        }
    }, [api, callbacks]);
    // Execute swap
    const executeSwap = useCallback(async (request) => {
        setLoading(true);
        setError(null);
        try {
            const { tx, quote: swapQuote } = await api.buildSwap(request);
            // Execute via wallet
            // This would connect to user's wallet (MetaMask, WalletConnect, etc.)
            const result = await api.executeSwap(tx);
            callbacks?.onSwap?.(result);
            return result;
        }
        catch (e) {
            const err = e;
            setError(err);
            callbacks?.onError?.(err);
            throw err;
        }
        finally {
            setLoading(false);
        }
    }, [api, callbacks]);
    // Connect wallet
    const connectWallet = useCallback(async () => {
        // This would integrate with wallet libraries
        callbacks?.onConnectWallet?.();
        // Mock wallet connection
        setWallet({
            address: '0x...',
            chainId: config.defaultChainId || 1,
            isConnected: true,
            balance: '0',
        });
    }, [config.defaultChainId, callbacks]);
    // Disconnect wallet
    const disconnectWallet = useCallback(() => {
        callbacks?.onDisconnectWallet?.();
        setWallet({
            address: null,
            chainId: null,
            isConnected: false,
            balance: '0',
        });
    }, [callbacks]);
    return {
        tokens,
        quote,
        loading,
        error,
        wallet,
        getQuote,
        executeSwap,
        connectWallet,
        disconnectWallet,
    };
}
// React Component - Swap Widget
export function TigerSwapWidget({ config, callbacks, className }) {
    const { tokens, quote, loading, error, wallet, getQuote, executeSwap, connectWallet, disconnectWallet, } = useTigerSwapWidget(config, callbacks);
    const [fromToken, setFromToken] = useState(null);
    const [toToken, setToToken] = useState(null);
    const [amount, setAmount] = useState(config.defaultAmount || '');
    // Set default tokens
    useEffect(() => {
        if (tokens.length > 0 && !fromToken) {
            const from = tokens.find(t => t.address === config.defaultFromToken) || tokens[0];
            const to = tokens.find(t => t.address === config.defaultToToken) || tokens[1] || tokens[0];
            setFromToken(from);
            setToToken(to);
        }
    }, [tokens, config.defaultFromToken, config.defaultToToken, fromToken]);
    // Handle quote request
    const handleGetQuote = useCallback(async () => {
        if (!fromToken || !toToken || !amount)
            return;
        await getQuote({
            chainId: config.defaultChainId || 1,
            fromToken: fromToken.address,
            toToken: toToken.address,
            amount,
            slippageTolerance: config.slippageTolerance || 50,
        });
    }, [fromToken, toToken, amount, config, getQuote]);
    // Handle swap
    const handleSwap = useCallback(async () => {
        if (!quote || !wallet.address)
            return;
        await executeSwap({
            quote,
            fromAddress: wallet.address,
        });
    }, [quote, wallet.address, executeSwap]);
    // Theme
    const theme = config.theme || 'dark';
    const themeClass = `tigerswap-widget-${theme}`;
    return (<div className={`tigerswap-widget ${themeClass} ${className || ''}`} style={{
            width: config.width || '100%',
            height: config.height || '500px',
            borderRadius: '12px',
            overflow: 'hidden',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        }}>
      {/* Header */}
      <div className="widget-header" style={{
            padding: '16px',
            borderBottom: '1px solid rgba(255,255,255,0.1)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
        }}>
        <div className="logo" style={{ fontWeight: 'bold', fontSize: '18px' }}>
          🐯 TigerSwap
        </div>

        {wallet.isConnected ? (<button onClick={disconnectWallet} style={{
                padding: '8px 16px',
                borderRadius: '8px',
                border: 'none',
                background: 'rgba(255,255,255,0.1)',
                color: 'white',
                cursor: 'pointer',
            }}>
            {wallet.address?.slice(0, 6)}...{wallet.address?.slice(-4)}
          </button>) : (<button onClick={connectWallet} style={{
                padding: '8px 16px',
                borderRadius: '8px',
                border: 'none',
                background: '#f59e0b',
                color: 'black',
                cursor: 'pointer',
                fontWeight: 'bold',
            }}>
            Connect Wallet
          </button>)}
      </div>

      {/* Swap Form */}
      <div className="widget-body" style={{ padding: '16px' }}>
        {/* From Token */}
        <div className="token-input" style={{
            background: 'rgba(255,255,255,0.05)',
            borderRadius: '12px',
            padding: '16px',
            marginBottom: '12px',
        }}>
          <div style={{ marginBottom: '8px', color: 'rgba(255,255,255,0.6)', fontSize: '14px' }}>
            You Pay
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <input type="text" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0.00" style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            fontSize: '24px',
            color: 'white',
            outline: 'none',
        }}/>

            <select value={fromToken?.address || ''} onChange={(e) => {
            const token = tokens.find(t => t.address === e.target.value);
            setFromToken(token || null);
        }} style={{
            padding: '8px 12px',
            borderRadius: '8px',
            background: 'rgba(255,255,255,0.1)',
            color: 'white',
            border: 'none',
            cursor: 'pointer',
        }}>
              {tokens.map(token => (<option key={token.address} value={token.address}>
                  {token.symbol}
                </option>))}
            </select>
          </div>
        </div>

        {/* Swap Button */}
        <div style={{ textAlign: 'center', margin: '-8px 0', position: 'relative', zIndex: 1 }}>
          <button onClick={handleGetQuote} disabled={loading} style={{
            width: '40px',
            height: '40px',
            borderRadius: '50%',
            border: 'none',
            background: '#f59e0b',
            cursor: 'pointer',
            fontSize: '20px',
        }}>
            ⇅
          </button>
        </div>

        {/* To Token */}
        <div className="token-output" style={{
            background: 'rgba(255,255,255,0.05)',
            borderRadius: '12px',
            padding: '16px',
            marginBottom: '16px',
        }}>
          <div style={{ marginBottom: '8px', color: 'rgba(255,255,255,0.6)', fontSize: '14px' }}>
            You Receive
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ flex: 1, fontSize: '24px', color: 'white' }}>
              {quote ? quote.toAmount : '0.00'}
            </div>

            <select value={toToken?.address || ''} onChange={(e) => {
            const token = tokens.find(t => t.address === e.target.value);
            setToToken(token || null);
        }} style={{
            padding: '8px 12px',
            borderRadius: '8px',
            background: 'rgba(255,255,255,0.1)',
            color: 'white',
            border: 'none',
            cursor: 'pointer',
        }}>
              {tokens.map(token => (<option key={token.address} value={token.address}>
                  {token.symbol}
                </option>))}
            </select>
          </div>
        </div>

        {/* Quote Details */}
        {quote && (<div className="quote-details" style={{
                background: 'rgba(255,255,255,0.05)',
                borderRadius: '12px',
                padding: '12px',
                marginBottom: '16px',
                fontSize: '14px',
            }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
              <span style={{ color: 'rgba(255,255,255,0.6)' }}>Rate</span>
              <span>1 {quote.fromToken.symbol} = {quote.toAmount / parseFloat(quote.fromAmount)} {quote.toToken.symbol}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
              <span style={{ color: 'rgba(255,255,255,0.6)' }}>Price Impact</span>
              <span style={{ color: quote.priceImpact > 5 ? '#ef4444' : '#22c55e' }}>
                {quote.priceImpact.toFixed(2)}%
              </span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ color: 'rgba(255,255,255,0.6)' }}>Est. Gas</span>
              <span>~{quote.estimatedGas}</span>
            </div>
          </div>)}

        {/* Swap Button */}
        <button onClick={handleSwap} disabled={loading || !quote || !wallet.isConnected} style={{
            width: '100%',
            padding: '16px',
            borderRadius: '12px',
            border: 'none',
            background: wallet.isConnected ? '#f59e0b' : 'rgba(255,255,255,0.1)',
            color: wallet.isConnected ? 'black' : 'rgba(255,255,255,0.5)',
            fontSize: '16px',
            fontWeight: 'bold',
            cursor: wallet.isConnected ? 'pointer' : 'not-allowed',
            opacity: loading ? 0.7 : 1,
        }}>
          {loading ? 'Processing...' : !wallet.isConnected ? 'Connect Wallet to Swap' : 'Swap'}
        </button>

        {/* Error Display */}
        {error && (<div style={{
                marginTop: '12px',
                padding: '12px',
                borderRadius: '8px',
                background: 'rgba(239, 68, 68, 0.2)',
                color: '#ef4444',
                fontSize: '14px',
            }}>
            {error.message}
          </div>)}
      </div>
    </div>);
}
// Export for standalone usage
export default TigerSwapWidget;
export { TigerSwapAPIClient, TigerSwapWSClient };
// Utility function to embed widget
export function embedTigerSwapWidget(container, config, callbacks) {
    const wrapper = document.createElement('div');
    container.appendChild(wrapper);
    // Render React widget (would use ReactDOM in real implementation)
    console.log('TigerSwap Widget initialized with config:', config);
    return {
        updateConfig: (newConfig) => {
            Object.assign(config, newConfig);
        },
        destroy: () => {
            wrapper.remove();
        },
    };
}
//# sourceMappingURL=index.jsx.map