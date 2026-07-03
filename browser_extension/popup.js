// TigerSwap Wallet - Popup Script

const TIGERSWAP_API_BASE_URL = 'https://api.tigerswap.com/v1';


class TigerSwapWallet {
    constructor() {
        this.wallet = null;
        this.network = 'ethereum';
        this.assets = [];
        this.isLoading = false;
        
        this.init();
    }
    
    async init() {
        // Load wallet from storage
        await this.loadWallet();
        
        // Setup event listeners
        this.setupEventListeners();
        
        // Update UI
        this.updateUI();
    }
    
    async loadWallet() {
        try {
            const result = await chrome.storage.local.get('wallet');
            this.wallet = result.wallet || null;
            
            // Load assets if wallet exists
            if (this.wallet) {
                await this.loadAssets();
            }
        } catch (error) {
            console.error('Failed to load wallet:', error);
        }
    }
    
    async loadAssets() {
        if (!this.wallet) return;
        
        this.isLoading = true;
        this.updateLoadingState();
        
        try {
            const response = await fetch(`${TIGERSWAP_API_BASE_URL}/portfolio/${this.wallet.address}`);
            if (!response.ok) {
                throw new Error(`Portfolio request failed with HTTP ${response.status}`);
            }
            const data = await response.json();
            this.assets = data.tokens || [];
        } catch (error) {
            console.error('Failed to load assets:', error);
            this.assets = [];
            this.showMessage('Unable to load portfolio from TigerSwap API.', 'error');
        }
        
        this.isLoading = false;
        this.updateAssetsList();
    }
    
    setupEventListeners() {
        // Navigation buttons
        document.getElementById('connect-wallet')?.addEventListener('click', () => this.showView('create-view'));
        document.getElementById('create-wallet')?.addEventListener('click', () => this.showView('create-view'));
        document.getElementById('import-wallet')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('import-view');
        });
        
        // Back buttons
        document.getElementById('back-to-main')?.addEventListener('click', () => this.showMainView());
        document.getElementById('back-to-main-send')?.addEventListener('click', () => this.showMainView());
        document.getElementById('back-to-main-receive')?.addEventListener('click', () => this.showMainView());
        document.getElementById('back-to-connect')?.addEventListener('click', () => this.showView('not-connected'));
        document.getElementById('back-to-import')?.addEventListener('click', () => this.showView('not-connected'));
        
        // Quick actions
        document.getElementById('swap-btn')?.addEventListener('click', () => this.showView('swap-view'));
        document.getElementById('send-btn')?.addEventListener('click', () => this.showView('send-view'));
        document.getElementById('receive-btn')?.addEventListener('click', () => this.showView('receive-view'));
        
        // Swap functionality
        document.getElementById('swap-tokens')?.addEventListener('click', () => this.swapTokens());
        document.getElementById('from-amount')?.addEventListener('input', () => this.calculateQuote());
        document.getElementById('execute-swap')?.addEventListener('click', () => this.executeSwap());
        
        // Slippage buttons
        document.querySelectorAll('.slippage-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.slippage-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
            });
        });
        
        // Send functionality
        document.getElementById('send-max')?.addEventListener('click', () => this.setMaxAmount());
        document.getElementById('execute-send')?.addEventListener('click', () => this.executeSend());
        
        // Receive functionality
        document.getElementById('copy-address')?.addEventListener('click', () => this.copyAddress());
        
        // Create wallet
        document.getElementById('generate-wallet')?.addEventListener('click', () => this.createWallet());
        
        // Import wallet
        document.getElementById('import-wallet-btn')?.addEventListener('click', () => this.importWallet());
        
        // Network selector
        document.getElementById('network')?.addEventListener('change', (e) => {
            this.network = e.target.value;
            this.loadAssets();
        });
    }
    
    updateUI() {
        if (this.wallet) {
            this.showView('connected');
            this.updateBalance();
        } else {
            this.showView('not-connected');
        }
    }
    
    showView(viewId) {
        // Hide all views
        document.querySelectorAll('.view').forEach(view => {
            view.classList.add('hidden');
        });
        
        // Show selected view
        document.getElementById(viewId)?.classList.remove('hidden');
    }
    
    showMainView() {
        if (this.wallet) {
            this.showView('connected');
        } else {
            this.showView('not-connected');
        }
    }
    
    updateBalance() {
        const totalBalance = this.assets.reduce((sum, asset) => sum + asset.valueUSD, 0);
        document.getElementById('total-balance').textContent = `$${totalBalance.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
        
        document.getElementById('balance-change').textContent = 'Live portfolio data';
    }
    
    updateAssetsList() {
        const list = document.getElementById('assets-list');
        if (!list) return;
        
        list.innerHTML = this.assets.map(asset => `
            <div class="asset-item">
                <div class="asset-info">
                    <div class="asset-icon">${asset.symbol.charAt(0)}</div>
                    <div class="asset-details">
                        <div class="asset-symbol">${asset.symbol}</div>
                        <div class="asset-balance">${parseFloat(asset.balance).toLocaleString()}</div>
                    </div>
                </div>
                <div class="asset-value">$${asset.valueUSD.toLocaleString()}</div>
            </div>
        `).join('');
    }
    
    updateLoadingState() {
        const swapBtn = document.getElementById('execute-swap');
        if (swapBtn) {
            swapBtn.disabled = this.isLoading;
            swapBtn.textContent = this.isLoading ? 'Loading...' : 'Swap';
        }
    }
    
    async calculateQuote() {
        const fromAmount = document.getElementById('from-amount')?.value;
        const fromToken = document.getElementById('from-token')?.value;
        const toToken = document.getElementById('to-token')?.value;
        
        if (!fromAmount || parseFloat(fromAmount) <= 0) {
            document.getElementById('to-amount').value = '';
            return;
        }
        
        try {
            const response = await fetch(
                `${TIGERSWAP_API_BASE_URL}/quote?fromToken=${fromToken}&toToken=${toToken}&amount=${fromAmount}`
            );
            
            if (!response.ok) {
                throw new Error(`Quote request failed with HTTP ${response.status}`);
            }
            const data = await response.json();
            document.getElementById('to-amount').value = data.toAmount;
            document.getElementById('exchange-rate').textContent = `1 ${fromToken} = ${data.rate} ${toToken}`;
        } catch (error) {
            console.error('Failed to calculate quote:', error);
            document.getElementById('to-amount').value = '';
            document.getElementById('exchange-rate').textContent = 'Quote unavailable';
            this.showMessage('Unable to fetch a live quote. Please try again.', 'error');
        }
    }
    
    swapTokens() {
        const fromSelect = document.getElementById('from-token');
        const toSelect = document.getElementById('to-token');
        
        const temp = fromSelect.value;
        fromSelect.value = toSelect.value;
        toSelect.value = temp;
        
        document.getElementById('from-amount').value = '';
        document.getElementById('to-amount').value = '';
    }
    
    async executeSwap() {
        const fromAmount = document.getElementById('from-amount')?.value;
        const fromToken = document.getElementById('from-token')?.value;
        const toToken = document.getElementById('to-token')?.value;
        
        if (!fromAmount || !this.wallet) {
            this.showMessage('Please connect your wallet first', 'error');
            return;
        }
        
        this.isLoading = true;
        this.updateLoadingState();
        
        try {
            // Send transaction request
            const response = await fetch(`${TIGERSWAP_API_BASE_URL}/swap`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    fromToken,
                    toToken,
                    amount: fromAmount,
                    fromAddress: this.wallet.address
                })
            });
            
            if (!response.ok) {
                throw new Error(`Swap request failed with HTTP ${response.status}`);
            }
            const data = await response.json();
            this.showMessage(`Swap submitted! Tx: ${data.txHash.slice(0, 10)}...`, 'success');
            await this.loadAssets();
        } catch (error) {
            console.error('Failed to execute swap:', error);
            this.showMessage('Swap was not submitted. Please review the error and try again.', 'error');
        }
        
        this.isLoading = false;
        this.updateLoadingState();
    }
    
    setMaxAmount() {
        const sendToken = document.getElementById('send-token')?.value;
        const asset = this.assets.find(a => a.symbol === sendToken);
        
        if (asset) {
            document.getElementById('send-amount').value = asset.balance;
        }
    }
    
    async executeSend() {
        const recipient = document.getElementById('recipient-address')?.value;
        const amount = document.getElementById('send-amount')?.value;
        const token = document.getElementById('send-token')?.value;
        
        if (!recipient || !amount || !this.wallet) {
            this.showMessage('Please fill in all fields', 'error');
            return;
        }
        
        if (!this.isValidAddress(recipient)) {
            this.showMessage('Invalid recipient address', 'error');
            return;
        }
        
        this.isLoading = true;
        this.updateLoadingState();
        
        try {
            const response = await fetch(`${TIGERSWAP_API_BASE_URL}/transfer`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    to: recipient,
                    amount,
                    token,
                    fromAddress: this.wallet.address
                })
            });
            
            if (!response.ok) {
                throw new Error(`Transfer request failed with HTTP ${response.status}`);
            }
            const data = await response.json();
            this.showMessage(`Transfer submitted! Tx: ${data.txHash.slice(0, 10)}...`, 'success');
            await this.loadAssets();
        } catch (error) {
            console.error('Failed to execute transfer:', error);
            this.showMessage('Transfer was not submitted. Please review the error and try again.', 'error');
        }
        
        this.isLoading = false;
        this.updateLoadingState();
    }
    
    copyAddress() {
        const addressInput = document.getElementById('receive-address');
        if (addressInput && this.wallet) {
            navigator.clipboard.writeText(this.wallet.address);
            this.showMessage('Address copied!', 'success');
        }
    }
    
    isValidAddress(address) {
        return /^0x[a-fA-F0-9]{40}$/.test(address);
    }
    
    async createWallet() {
        const password = document.getElementById('create-password')?.value;
        const confirmPassword = document.getElementById('confirm-password')?.value;
        
        if (!password || password.length < 8) {
            this.showMessage('Password must be at least 8 characters', 'error');
            return;
        }
        
        if (password !== confirmPassword) {
            this.showMessage('Passwords do not match', 'error');
            return;
        }
        
        this.isLoading = true;
        
        try {
            // Generate mnemonic
            const mnemonic = this.generateMnemonic();
            
            // Derive keys
            const key = await this.deriveKey(mnemonic, password);
            
            this.wallet = {
                address: key.address,
                privateKey: key.privateKey,
                publicKey: key.publicKey,
                mnemonic: this.encryptMnemonic(mnemonic, password),
                created: Date.now()
            };
            
            // Save to storage
            await chrome.storage.local.set({ wallet: this.wallet });
            
            this.showMessage('Wallet created successfully!', 'success');
            this.showView('connected');
            await this.loadAssets();
        } catch (error) {
            this.showMessage('Failed to create wallet', 'error');
        }
        
        this.isLoading = false;
    }
    
    async importWallet() {
        const mnemonic = document.getElementById('import-mnemonic')?.value.trim();
        const password = document.getElementById('import-password')?.value;
        
        if (!mnemonic || mnemonic.split(' ').length < 12) {
            this.showMessage('Please enter a valid recovery phrase', 'error');
            return;
        }
        
        if (!password || password.length < 8) {
            this.showMessage('Password must be at least 8 characters', 'error');
            return;
        }
        
        this.isLoading = true;
        
        try {
            const key = await this.deriveKey(mnemonic, password);
            
            this.wallet = {
                address: key.address,
                privateKey: key.privateKey,
                publicKey: key.publicKey,
                mnemonic: this.encryptMnemonic(mnemonic, password),
                created: Date.now()
            };
            
            await chrome.storage.local.set({ wallet: this.wallet });
            
            this.showMessage('Wallet imported successfully!', 'success');
            this.showView('connected');
            await this.loadAssets();
        } catch (error) {
            this.showMessage('Failed to import wallet', 'error');
        }
        
        this.isLoading = false;
    }
    
    // Cryptographic functions (simplified for browser extension)
    generateMnemonic() {
        const words = [
            'abandon', 'ability', 'able', 'about', 'above', 'absent', 'absorb', 'abstract', 
            'absurd', 'abuse', 'access', 'accident', 'account', 'accuse', 'achieve', 'acid',
            'acoustic', 'acquire', 'across', 'act', 'action', 'actor', 'actress', 'actual'
        ];
        
        let mnemonic = [];
        for (let i = 0; i < 12; i++) {
            mnemonic.push(words[Math.floor(Math.random() * words.length)]);
        }
        return mnemonic.join(' ');
    }
    
    async deriveKey(mnemonic, password) {
        // Simplified key derivation - in production use proper PBKDF2
        const encoder = new TextEncoder();
        const data = encoder.encode(mnemonic + password);
        
        // Use SubtleCrypto for hashing
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        
        const privateKey = '0x' + hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
        const address = '0x' + hashArray.slice(0, 20).map(b => b.toString(16).padStart(2, '0')).join('');
        
        return {
            privateKey,
            publicKey: '0x' + hashArray.slice(0, 33).map(b => b.toString(16).padStart(2, '0')).join(''),
            address
        };
    }
    
    encryptMnemonic(mnemonic, password) {
        // Simplified encryption - in production use proper AES-GCM
        return btoa(mnemonic);
    }
    
    showMessage(message, type) {
        // Remove existing message
        const existing = document.querySelector('.message');
        existing?.remove();
        
        // Create new message
        const messageEl = document.createElement('div');
        messageEl.className = `message ${type}`;
        messageEl.textContent = message;
        
        // Insert after header
        document.querySelector('.main-content')?.prepend(messageEl);
        
        // Auto-remove after 3 seconds
        setTimeout(() => messageEl.remove(), 3000);
    }
}

// Initialize wallet
document.addEventListener('DOMContentLoaded', () => {
    window.tigerSwapWallet = new TigerSwapWallet();
});