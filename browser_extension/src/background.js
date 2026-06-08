// TigerSwap Wallet - Background Service Worker

// Wallet state
let wallet = null;
let connectedSites = new Map();

// Initialize wallet
async function initWallet() {
    const stored = await chrome.storage.local.get('wallet');
    if (stored.wallet) {
        wallet = stored.wallet;
    }
}

// Create new wallet
async function createWallet(password) {
    // Generate secure random mnemonic
    const mnemonic = generateMnemonic();
    
    // Derive key from mnemonic
    const key = await deriveKey(mnemonic, password);
    
    wallet = {
        address: key.address,
        privateKey: key.privateKey,
        publicKey: key.publicKey,
        mnemonic: encryptMnemonic(mnemonic, password),
        created: Date.now()
    };
    
    await chrome.storage.local.set({ wallet });
    return wallet;
}

// Import existing wallet
async function importWallet(mnemonic, password) {
    const key = await deriveKey(mnemonic, password);
    
    wallet = {
        address: key.address,
        privateKey: key.privateKey,
        publicKey: key.publicKey,
        mnemonic: encryptMnemonic(mnemonic, password),
        created: Date.now()
    };
    
    await chrome.storage.local.set({ wallet });
    return wallet;
}

// Connect to dApp
async function connectToSite(tabId, origin) {
    if (!wallet) {
        throw new Error('No wallet connected');
    }
    
    connectedSites.set(origin, {
        address: wallet.address,
        chainId: 1,
        connectedAt: Date.now()
    });
    
    // Send connection response
    chrome.tabs.sendMessage(tabId, {
        type: 'CONNECT_RESPONSE',
        payload: {
            address: wallet.address,
            chainId: 1,
            success: true
        }
    });
}

// Handle requests from content scripts
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    const { type, payload } = message;
    
    switch (type) {
        case 'GET_WALLET_STATE':
            sendResponse({ 
                isConnected: !!wallet,
                address: wallet?.address 
            });
            break;
            
        case 'CONNECT':
            connectToSite(sender.tab.id, payload.origin)
                .then(() => sendResponse({ success: true }))
                .catch(err => sendResponse({ success: false, error: err.message }));
            break;
            
        case 'SIGN_MESSAGE':
            signMessage(payload.message)
                .then(signature => sendResponse({ signature }))
                .catch(err => sendResponse({ error: err.message }));
            break;
            
        case 'SIGN_TRANSACTION':
            signTransaction(payload.transaction)
                .then(signedTx => sendResponse({ signedTx }))
                .catch(err => sendResponse({ error: err.message }));
            break;
            
        case 'SEND_TRANSACTION':
            sendTransaction(payload.to, payload.value, payload.data)
                .then(txHash => sendResponse({ txHash }))
                .catch(err => sendResponse({ error: err.message }));
            break;
            
        case 'GET_BALANCE':
            getBalance(payload.address)
                .then(balances => sendResponse({ balances }))
                .catch(err => sendResponse({ error: err.message }));
            break;
            
        case 'DISCONNECT':
            disconnectFromSite(payload.origin)
                .then(() => sendResponse({ success: true }))
                .catch(err => sendResponse({ error: err.message }));
            break;
            
        default:
            sendResponse({ error: 'Unknown request type' });
    }
    
    return true; // Keep message channel open for async response
});

// Sign message
async function signMessage(message) {
    if (!wallet) throw new Error('Wallet not connected');
    
    // Sign with private key (in real implementation)
    const msgHash = keccak256(message);
    return sign(wallet.privateKey, msgHash);
}

// Sign transaction
async function signTransaction(tx) {
    if (!wallet) throw new Error('Wallet not connected');
    
    const txHash = keccak256(encodeTx(tx));
    return sign(wallet.privateKey, txHash);
}

// Send transaction
async function sendTransaction(to, value, data) {
    if (!wallet) throw new Error('Wallet not connected');
    
    // In real implementation, broadcast to network
    const tx = {
        from: wallet.address,
        to,
        value,
        data,
        nonce: await getNonce(wallet.address),
        gasPrice: await getGasPrice(),
        gasLimit: 21000
    };
    
    const signedTx = await signTransaction(tx);
    const txHash = await broadcastTransaction(signedTx);
    
    return txHash;
}

// Get token balances
async function getBalance(address) {
    // In real implementation, query RPC
    return {
        ETH: '1.5',
        tokens: [
            { symbol: 'USDC', balance: '1000' },
            { symbol: 'WETH', balance: '0.5' }
        ]
    };
}

// Disconnect from site
async function disconnectFromSite(origin) {
    connectedSites.delete(origin);
}

// Helper: Generate mnemonic (simplified)
function generateMnemonic() {
    const words = [];
    const wordlist = [
        'abandon', 'ability', 'able', 'about', 'above', 'absent', 'absorb', 'abstract', 
        'absurd', 'abuse', 'access', 'accident', 'account', 'accuse', 'achieve', 'acid',
        'acoustic', 'acquire', 'across', 'act', 'action', 'actor', 'actress', 'actual'
    ];
    
    for (let i = 0; i < 12; i++) {
        words.push(wordlist[Math.floor(Math.random() * wordlist.length)]);
    }
    
    return words.join(' ');
}

// Helper: Derive key from mnemonic
async function deriveKey(mnemonic, password) {
    // Simplified - in real implementation use proper KDF
    const seed = await sha256(mnemonic + password);
    const privateKey = seed.slice(0, 64);
    const publicKey = await derivePublicKey(privateKey);
    const address = await deriveAddress(publicKey);
    
    return { privateKey, publicKey, address };
}

// Helper: Encrypt mnemonic
function encryptMnemonic(mnemonic, password) {
    // Simplified encryption
    return btoa(mnemonic + ':' + password);
}

// Helper: Keccak256
function keccak256(data) {
    // Simplified - use proper crypto in production
    return data;
}

// Helper: Sign
async function sign(privateKey, message) {
    // Simplified - use proper ECDSA in production
    return '0x' + privateKey.slice(0, 64) + message.slice(0, 64);
}

// Helper: Encode transaction
function encodeTx(tx) {
    return JSON.stringify(tx);
}

// Helper: Get nonce
async function getNonce(address) {
    // Query RPC
    return 0;
}

// Helper: Get gas price
async function getGasPrice() {
    // Query RPC
    return '20000000000'; // 20 Gwei
}

// Helper: Broadcast transaction
async function broadcastTransaction(signedTx) {
    // Submit to RPC
    return '0x' + Math.random().toString(16).slice(2, 66);
}

// Helper: Derive public key
async function derivePublicKey(privateKey) {
    return privateKey; // Simplified
}

// Helper: Derive address
async function deriveAddress(publicKey) {
    const hash = await sha256(publicKey);
    return '0x' + hash.slice(-40);
}

// Helper: SHA256
async function sha256(data) {
    // Use Web Crypto API
    const encoder = new TextEncoder();
    const dataBuffer = encoder.encode(data);
    const hashBuffer = await crypto.subtle.digest('SHA-256', dataBuffer);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
}

// Initialize on install
chrome.runtime.onInstalled.addListener(initWallet);
chrome.runtime.onStartup.addListener(initWallet);
