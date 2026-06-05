/**
 * TigerSwap Chain Management REST API
 * Built from scratch - no dependencies on other protocols
 */

import { NextRequest, NextResponse } from 'next/server';
import { chainRegistry, ChainConfig, ChainCategory, ChainStatus } from '../../../../libs/chain_registry/universal_chain_registry';

// ============================================================================
// GET /api/chains - List all chains
// ============================================================================

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    
    const category = searchParams.get('category') as ChainCategory | null;
    const status = searchParams.get('status') as ChainStatus | null;
    const search = searchParams.get('search');
    const chainId = searchParams.get('chainId');
    const statsOnly = searchParams.get('stats') === 'true';

    // Get stats if requested
    if (statsOnly) {
      const stats = chainRegistry.getChainStats();
      return NextResponse.json({
        success: true,
        data: stats
      });
    }

    // Get single chain by ID
    if (chainId) {
      const chain = chainRegistry.getChain(chainId);
      if (!chain) {
        return NextResponse.json({
          success: false,
          error: `Chain ${chainId} not found`
        }, { status: 404 });
      }
      return NextResponse.json({
        success: true,
        data: chain
      });
    }

    // Get chains by category
    if (category) {
      const chains = chainRegistry.getChainsByCategory(category);
      return NextResponse.json({
        success: true,
        data: chains,
        count: chains.length
      });
    }

    // Get chains by status
    if (status) {
      const chains = chainRegistry.getChainsByStatus(status);
      return NextResponse.json({
        success: true,
        data: chains,
        count: chains.length
      });
    }

    // Search chains
    if (search) {
      const chains = chainRegistry.searchChains(search);
      return NextResponse.json({
        success: true,
        data: chains,
        count: chains.length,
        query: search
      });
    }

    // Get all chains
    const allChains = chainRegistry.getAllChains();
    return NextResponse.json({
      success: true,
      data: allChains,
      count: allChains.length,
      stats: chainRegistry.getChainStats()
    });

  } catch (error: any) {
    console.error('Chain API error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Internal server error'
    }, { status: 500 });
  }
}

// ============================================================================
// POST /api/chains - Add new chain
// ============================================================================

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    
    // Validate required fields
    const required = ['id', 'name', 'category', 'chainId', 'rpcUrls', 'nativeCurrency'];
    for (const field of required) {
      if (!body[field]) {
        return NextResponse.json({
          success: false,
          error: `Missing required field: ${field}`
        }, { status: 400 });
      }
    }

    // Validate native currency
    if (!body.nativeCurrency.name || !body.nativeCurrency.symbol) {
      return NextResponse.json({
        success: false,
        error: 'Native currency must have name and symbol'
      }, { status: 400 });
    }

    // Create chain config
    const chainConfig: ChainConfig = {
      id: body.id.toLowerCase().replace(/\s+/g, '-'),
      name: body.name,
      symbol: body.symbol || body.nativeCurrency.symbol,
      category: body.category,
      status: body.status || 'active',
      chainId: body.chainId,
      networkId: body.networkId,
      rpcUrls: Array.isArray(body.rpcUrls) ? body.rpcUrls : [body.rpcUrls],
      explorerUrls: Array.isArray(body.explorerUrls) ? body.explorerUrls : body.explorerUrls ? [body.explorerUrls] : [],
      nativeCurrency: {
        name: body.nativeCurrency.name,
        symbol: body.nativeCurrency.symbol,
        decimals: body.nativeCurrency.decimals || 18,
        logoUrl: body.nativeCurrency.logoUrl,
      },
      blockTime: body.blockTime,
      gasLimit: body.gasLimit,
      supportsEIP1559: body.supportsEIP1559,
      supportsFlashbots: body.supportsFlashbots,
      supportsMEV: body.supportsMEV,
      supportsMulticall: body.supportsMulticall,
      notes: body.notes,
    };

    // Add chain
    chainRegistry.addChain(chainConfig);

    return NextResponse.json({
      success: true,
      message: `Chain ${body.name} added successfully`,
      data: chainConfig
    }, { status: 201 });

  } catch (error: any) {
    console.error('Add chain error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Failed to add chain'
    }, { status: 500 });
  }
}

// ============================================================================
// PUT /api/chains - Update chain
// ============================================================================

export async function PUT(request: NextRequest) {
  try {
    const body = await request.json();
    
    if (!body.id) {
      return NextResponse.json({
        success: false,
        error: 'Chain ID is required'
      }, { status: 400 });
    }

    // Get existing chain
    const existingChain = chainRegistry.getChain(body.id);
    if (!existingChain) {
      return NextResponse.json({
        success: false,
        error: `Chain ${body.id} not found`
      }, { status: 404 });
    }

    // Build update object (only include provided fields)
    const updates: Partial<ChainConfig> = {};
    
    if (body.name !== undefined) updates.name = body.name;
    if (body.symbol !== undefined) updates.symbol = body.symbol;
    if (body.status !== undefined) updates.status = body.status;
    if (body.rpcUrls !== undefined) updates.rpcUrls = Array.isArray(body.rpcUrls) ? body.rpcUrls : [body.rpcUrls];
    if (body.explorerUrls !== undefined) updates.explorerUrls = Array.isArray(body.explorerUrls) ? body.explorerUrls : [body.explorerUrls];
    if (body.blockTime !== undefined) updates.blockTime = body.blockTime;
    if (body.gasLimit !== undefined) updates.gasLimit = body.gasLimit;
    if (body.supportsEIP1559 !== undefined) updates.supportsEIP1559 = body.supportsEIP1559;
    if (body.supportsFlashbots !== undefined) updates.supportsFlashbots = body.supportsFlashbots;
    if (body.supportsMEV !== undefined) updates.supportsMEV = body.supportsMEV;
    if (body.supportsMulticall !== undefined) updates.supportsMulticall = body.supportsMulticall;
    if (body.notes !== undefined) updates.notes = body.notes;
    
    if (body.nativeCurrency) {
      updates.nativeCurrency = {
        ...existingChain.nativeCurrency,
        ...body.nativeCurrency
      };
    }

    // Update chain
    chainRegistry.updateChain(body.id, updates);

    return NextResponse.json({
      success: true,
      message: `Chain ${body.id} updated successfully`,
      data: chainRegistry.getChain(body.id)
    });

  } catch (error: any) {
    console.error('Update chain error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Failed to update chain'
    }, { status: 500 });
  }
}

// ============================================================================
// DELETE /api/chains - Remove chain
// ============================================================================

export async function DELETE(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const chainId = searchParams.get('id');

    if (!chainId) {
      return NextResponse.json({
        success: false,
        error: 'Chain ID is required'
      }, { status: 400 });
    }

    // Get chain before deletion
    const chain = chainRegistry.getChain(chainId);
    if (!chain) {
      return NextResponse.json({
        success: false,
        error: `Chain ${chainId} not found`
      }, { status: 404 });
    }

    // Delete chain
    chainRegistry.removeChain(chainId);

    return NextResponse.json({
      success: true,
      message: `Chain ${chain.name} (${chainId}) deleted successfully`
    });

  } catch (error: any) {
    console.error('Delete chain error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Failed to delete chain'
    }, { status: 500 });
  }
}