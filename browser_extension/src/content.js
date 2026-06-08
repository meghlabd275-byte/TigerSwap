// TigerSwap Wallet - Content Script (Ethereum Provider Injection)

(function() {
    'use strict';
    
    // Provider configuration
    const PROVIDER_NAME = 'TigerSwap';
    const PROVIDER_VERSION = '1.0.0';
    
    // TigerSwap provider implementation
    class TigerSwapProvider {
        constructor() {
            this.isTigerSwap = true;
            this.isMetaMask = false;
            this.isConnected = false;
            this.chainId = null;
            this.networkVersion = null;
            this.selectedAddress = null;
            this._events = {};
            this._requestId = 0;
            this._callbacks = new Map();
        }
        
        // Request handler
        async request(args) {
            const id = ++this._requestId;
            
            return new Promise((resolve, reject) => {
                this._callbacks.set(id, { resolve, reject });
                
                // Send to background script
                chrome.runtime.sendMessage({
                    type: 'PROVIDER_REQUEST',
                    id,
                    method: args.method,
                    params: args.params || []
                }, (response) => {
                    if (response && response.error) {
                        reject(new Error(response.error));
                    } else if (response && response.result !== undefined) {
                        resolve(response.result);
                    } else {
                        reject(new Error('No response'));
                    }
                    this._callbacks.delete(id);
                });
            });
        }
        
        // Legacy methods
        async enable() {
            return this.request({ method: 'eth_requestAccounts' });
        }
        
        async send(method, params) {
            return this.request({ method, params });
        }
        
        async sendAsync(payload, callback) {
            try {
                const result = await this.request(payload);
                callback(null, { result, id: payload.id, jsonrpc: '2.0' });
            } catch (error) {
                callback(error, { error: error.message, id: payload.id, jsonrpc: '2.0' });
            }
        }
        
        // Event handlers
        on(event, listener) {
            if (!this._events[event]) {
                this._events[event] = [];
            }
            this._events[event].push(listener);
        }
        
        removeListener(event, listener) {
            if (!this._events[event]) return;
            this._events[event] = this._events[event].filter(l => l !== listener);
        }
        
        emit(event, ...args) {
            if (!this._events[event]) return;
            this._events[event].forEach(listener => {
                try {
                    listener(...args);
                } catch (e) {
                    console.error('Event listener error:', e);
                }
            });
        }
        
        // EIP-1193 events
        _setChainId(chainId) {
            this.chainId = chainId;
            this.emit('chainChanged', chainId);
        }
        
        _setAddress(address) {
            this.selectedAddress = address;
            this.isConnected = !!address;
            this.emit('accountsChanged', address ? [address] : []);
        }
        
        _resetState() {
            this.chainId = null;
            this.networkVersion = null;
            this.selectedAddress = null;
            this.isConnected = false;
            this.emit('disconnect');
        }
    }
    
    // Request accounts
    async function requestAccounts(provider) {
        const accounts = await provider.request({ method: 'eth_requestAccounts' });
        if (accounts && accounts.length > 0) {
            provider._setAddress(accounts[0]);
        }
        return accounts;
    }
    
    // Get chain id
    async function getChainId(provider) {
        const chainId = await provider.request({ method: 'eth_chainId' });
        provider._setChainId(chainId);
        return chainId;
    }
    
    // Get network version
    async function getNetworkVersion(provider) {
        const networkVersion = await provider.request({ method: 'net_version' });
        provider.networkVersion = networkVersion;
        return networkVersion;
    }
    
    // Get balance
    async function getBalance(provider, address) {
        return await provider.request({
            method: 'eth_getBalance',
            params: [address, 'latest']
        });
    }
    
    // Get code
    async function getCode(provider, address) {
        return await provider.request({
            method: 'eth_getCode',
            params: [address, 'latest']
        });
    }
    
    // Get storage at
    async function getStorageAt(provider, address, position) {
        return await provider.request({
            method: 'eth_getStorageAt',
            params: [address, position, 'latest']
        });
    }
    
    // Get block by number
    async function getBlockByNumber(provider, blockNumber) {
        return await provider.request({
            method: 'eth_getBlockByNumber',
            params: [blockNumber, true]
        });
    }
    
    // Get block by hash
    async function getBlockByHash(provider, blockHash) {
        return await provider.request({
            method: 'eth_getBlockByHash',
            params: [blockHash, true]
        });
    }
    
    // Get transaction by hash
    async function getTransactionByHash(provider, txHash) {
        return await provider.request({
            method: 'eth_getTransactionByHash',
            params: [txHash]
        });
    }
    
    // Get transaction receipt
    async function getTransactionReceipt(provider, txHash) {
        return await provider.request({
            method: 'eth_getTransactionReceipt',
            params: [txHash]
        });
    }
    
    // Estimate gas
    async function estimateGas(provider, transaction) {
        return await provider.request({
            method: 'eth_estimateGas',
            params: [transaction]
        });
    }
    
    // Get gas price
    async function getGasPrice(provider) {
        return await provider.request({ method: 'eth_gasPrice' });
    }
    
    // Call contract
    async function call(provider, transaction) {
        return await provider.request({
            method: 'eth_call',
            params: [transaction, 'latest']
        });
    }
    
    // Send raw transaction
    async function sendRawTransaction(provider, signedTx) {
        return await provider.request({
            method: 'eth_sendRawTransaction',
            params: [signedTx]
        });
    }
    
    // Get logs
    async function getLogs(provider, filter) {
        return await provider.request({
            method: 'eth_getLogs',
            params: [filter]
        });
    }
    
    // Get transaction count
    async function getTransactionCount(provider, address) {
        return await provider.request({
            method: 'eth_getTransactionCount',
            params: [address, 'latest']
        });
    }
    
    // Get block number
    async function getBlockNumber(provider) {
        return await provider.request({ method: 'eth_blockNumber' });
    }
    
    // Subscribe to events
    async function subscribe(provider, subscription, params) {
        return await provider.request({
            method: 'eth_subscribe',
            params: [subscription, params]
        });
    }
    
    // Unsubscribe
    async function unsubscribe(provider, subscriptionId) {
        return await provider.request({
            method: 'eth_unsubscribe',
            params: [subscriptionId]
        });
    }
    
    // ChainChanged listener
    function handleChainChanged(provider, chainId) {
        provider._setChainId(chainId);
    }
    
    // AccountsChanged listener
    function handleAccountsChanged(provider, accounts) {
        if (accounts.length === 0) {
            provider._resetState();
        } else {
            provider._setAddress(accounts[0]);
        }
    }
    
    // Disconnect listener
    function handleDisconnect(provider, error) {
        provider._resetState();
    }
    
    // Message listener
    function handleMessage(provider, message) {
        provider.emit('message', message);
    }
    
    // Inject provider into window
    function injectProvider() {
        // Check if already injected
        if (window.ethereum) {
            // Already has provider, just add TigerSwap
            window.ethereum.isTigerSwap = true;
            return;
        }
        
        // Create provider
        const provider = new TigerSwapProvider();
        
        // Set as window.ethereum
        Object.defineProperty(window, 'ethereum', {
            value: provider,
            writable: false,
            configurable: false
        });
        
        // Auto-connect on injection
        (async () => {
            try {
                await requestAccounts(provider);
                await getChainId(provider);
                await getNetworkVersion(provider);
            } catch (e) {
                console.log('TigerSwap: Auto-connect failed:', e);
            }
        })();
        
        // Listen for background messages
        chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
            if (message.type === 'PROVIDER_RESPONSE') {
                const { id, result, error } = message;
                // Handle response
                provider.emit('response', { id, result, error });
            }
        });
        
        console.log('TigerSwap Wallet Provider injected');
    }
    
    // Inject when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', injectProvider);
    } else {
        injectProvider();
    }
})();
