/**
 * TigerEX REST API Integration
 * 
 * HTTP endpoints for TigerEX integration layer
 * Provides unified access to all Tiger products
 */

import { Request, Response } from 'express';
import { 
  tigerEX, 
  TigerEXIntegration,
  ChainConfig,
  TokenConfig,
  LiquidityPool,
  FarmInfo,
  BridgeInfo,
  CrossChainRoute,
  SwapRoute
} from './tiger_ex_integration';

// ============================================================================
// Chain Management API
// ============================================================================

export async function getChains(req: Request, res: Response): Promise<void> {
  try {
    const { type } = req.query;
    
    let chains: ChainConfig[];
    
    if (type === 'evm') {
      chains = tigerEX.getSupportedEvmChains();
    } else if (type === 'nonevm') {
      chains = tigerEX.getSupportedNonEvmChains();
    } else {
      chains = [...tigerEX.getSupportedEvmChains(), ...tigerEX.getSupportedNonEvmChains()];
    }
    
    res.json({
      success: true,
      data: chains,
      count: chains.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function addChain(req: Request, res: Response): Promise<void> {
  try {
    const { chainConfig, chainType } = req.body;
    
    if (chainType === 'evm') {
      tigerEX.addEvmChain(chainConfig);
    } else if (chainType === 'nonevm') {
      tigerEX.addNonEvmChain(chainConfig);
    } else {
      throw new Error('Invalid chain type');
    }
    
    res.json({
      success: true,
      message: `Chain ${chainConfig.name} added successfully`,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

export async function searchChains(req: Request, res: Response): Promise<void> {
  try {
    const { q } = req.query;
    
    if (!q) {
      return res.status(400).json({ success: false, error: 'Query required' });
    }
    
    const results = tigerEX.searchChains(q as string);
    
    res.json({
      success: true,
      data: results,
      count: results.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function setChainStatus(req: Request, res: Response): Promise<void> {
  try {
    const { chainId, status } = req.body;
    
    tigerEX.setChainStatus(chainId, status);
    
    res.json({
      success: true,
      message: `Chain ${chainId} status updated to ${status}`,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Token Management API
// ============================================================================

export async function getTokens(req: Request, res: Response): Promise<void> {
  try {
    const tokens = tigerEX.getSupportedTokens();
    
    res.json({
      success: true,
      data: tokens,
      count: tokens.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function addToken(req: Request, res: Response): Promise<void> {
  try {
    const token = req.body;
    
    tigerEX.addToken(token);
    
    res.json({
      success: true,
      message: `Token ${token.symbol} added successfully`,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// DEX API
// ============================================================================

export async function getPools(req: Request, res: Response): Promise<void> {
  try {
    const pools = tigerEX.getDexPools();
    
    res.json({
      success: true,
      data: pools,
      count: pools.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function createPool(req: Request, res: Response): Promise<void> {
  try {
    const { tokenA, tokenB, fee } = req.body;
    
    if (!tokenA || !tokenB) {
      return res.status(400).json({ success: false, error: 'tokenA and tokenB required' });
    }
    
    const pool = tigerEX.createPool(tokenA, tokenB, fee || 0.003);
    
    res.json({
      success: true,
      data: pool,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

export async function getSwapQuote(req: Request, res: Response): Promise<void> {
  try {
    const { inputToken, outputToken, amount } = req.body;
    
    if (!inputToken || !outputToken || !amount) {
      return res.status(400).json({ 
        success: false, 
        error: 'inputToken, outputToken, and amount required' 
      });
    }
    
    const amountBigInt = BigInt(amount);
    const result = tigerEX.calculateSwap(inputToken, outputToken, amountBigInt);
    
    res.json({
      success: true,
      data: {
        inputToken,
        outputToken,
        amountIn: amount,
        amountOut: result.amountOut.toString(),
        fee: result.fee.toString(),
        path: result.path,
      },
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Farm API
// ============================================================================

export async function getFarms(req: Request, res: Response): Promise<void> {
  try {
    const farms = tigerEX.getFarms();
    
    res.json({
      success: true,
      data: farms,
      count: farms.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function createFarm(req: Request, res: Response): Promise<void> {
  try {
    const { poolId, rewardToken, apy } = req.body;
    
    if (!poolId || !rewardToken) {
      return res.status(400).json({ 
        success: false, 
        error: 'poolId and rewardToken required' 
      });
    }
    
    const farm = tigerEX.createFarm(poolId, rewardToken, apy || 25);
    
    res.json({
      success: true,
      data: farm,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Bridge API
// ============================================================================

export async function getBridges(req: Request, res: Response): Promise<void> {
  try {
    const bridges = tigerEX.getBridges();
    
    res.json({
      success: true,
      data: bridges,
      count: bridges.length,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

export async function createBridge(req: Request, res: Response): Promise<void> {
  try {
    const { sourceChain, targetChain, token, minAmount, maxAmount, fee, estimatedTime } = req.body;
    
    if (!sourceChain || !targetChain) {
      return res.status(400).json({ 
        success: false, 
        error: 'sourceChain and targetChain required' 
      });
    }
    
    const bridge = tigerEX.addBridge(
      sourceChain,
      targetChain,
      token || '*',
      BigInt(minAmount || 0),
      BigInt(maxAmount || '1000000000000000000000'),
      fee || 0.001,
      estimatedTime || 600000
    );
    
    res.json({
      success: true,
      data: bridge,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

export async function getBridgeQuote(req: Request, res: Response): Promise<void> {
  try {
    const { sourceChain, targetChain, token, amount } = req.body;
    
    if (!sourceChain || !targetChain || !amount) {
      return res.status(400).json({ 
        success: false, 
        error: 'sourceChain, targetChain, and amount required' 
      });
    }
    
    const amountBigInt = BigInt(amount);
    const result = tigerEX.calculateBridge(sourceChain, targetChain, token, amountBigInt);
    
    res.json({
      success: true,
      data: {
        sourceChain,
        targetChain,
        token,
        amountSent: amount,
        amountReceived: result.received.toString(),
        fee: result.fee.toString(),
        estimatedTime: result.estimatedTime,
      },
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Cross-Chain Swap API
// ============================================================================

export async function getCrossChainRoute(req: Request, res: Response): Promise<void> {
  try {
    const { sourceChain, targetChain, inputToken, outputToken, amount } = req.body;
    
    if (!sourceChain || !targetChain || !inputToken || !outputToken || !amount) {
      return res.status(400).json({ 
        success: false, 
        error: 'All fields required' 
      });
    }
    
    const amountBigInt = BigInt(amount);
    const route = tigerEX.getCrossChainRoute(
      sourceChain,
      targetChain,
      inputToken,
      outputToken,
      amountBigInt
    );
    
    res.json({
      success: true,
      data: route,
    });
  } catch (error) {
    res.status(400).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Product Status API
// ============================================================================

export async function getProducts(req: Request, res: Response): Promise<void> {
  try {
    const products = {
      wallet: tigerEX.getProductStatus('wallet'),
      swap: tigerEX.getProductStatus('swap'),
      smartchain: tigerEX.getProductStatus('smartchain'),
      ex: tigerEX.getProductStatus('ex'),
    };
    
    res.json({
      success: true,
      data: products,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Fee Collection API
// ============================================================================

export async function getFees(req: Request, res: Response): Promise<void> {
  try {
    const summary = tigerEX.getFeeSummary();
    
    res.json({
      success: true,
      data: summary,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Stats API
// ============================================================================

export async function getStats(req: Request, res: Response): Promise<void> {
  try {
    const stats = tigerEX.getStats();
    
    res.json({
      success: true,
      data: stats,
    });
  } catch (error) {
    res.status(500).json({ success: false, error: (error as Error).message });
  }
}

// ============================================================================
// Express Router Setup
// ============================================================================

export function setupTigerEXRoutes(app: any): void {
  // Chain routes
  app.get('/api/v1/chains', getChains);
  app.post('/api/v1/chains', addChain);
  app.get('/api/v1/chains/search', searchChains);
  app.put('/api/v1/chains/status', setChainStatus);
  
  // Token routes
  app.get('/api/v1/tokens', getTokens);
  app.post('/api/v1/tokens', addToken);
  
  // DEX routes
  app.get('/api/v1/pools', getPools);
  app.post('/api/v1/pools', createPool);
  app.post('/api/v1/swap/quote', getSwapQuote);
  
  // Farm routes
  app.get('/api/v1/farms', getFarms);
  app.post('/api/v1/farms', createFarm);
  
  // Bridge routes
  app.get('/api/v1/bridges', getBridges);
  app.post('/api/v1/bridges', createBridge);
  app.post('/api/v1/bridge/quote', getBridgeQuote);
  
  // Cross-chain routes
  app.post('/api/v1/route', getCrossChainRoute);
  
  // Product routes
  app.get('/api/v1/products', getProducts);
  
  // Fee routes
  app.get('/api/v1/fees', getFees);
  
  // Stats routes
  app.get('/api/v1/stats', getStats);
}

// ============================================================================
// Export
// ============================================================================

export default {
  setupTigerEXRoutes,
  getChains,
  addChain,
  searchChains,
  setChainStatus,
  getTokens,
  addToken,
  getPools,
  createPool,
  getSwapQuote,
  getFarms,
  createFarm,
  getBridges,
  createBridge,
  getBridgeQuote,
  getCrossChainRoute,
  getProducts,
  getFees,
  getStats,
};