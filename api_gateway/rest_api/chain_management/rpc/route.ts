/**
 * TigerSwap RPC Management API
 */

import { NextRequest, NextResponse } from 'next/server';
import { chainRegistry } from '../../../../../libs/chain_registry/universal_chain_registry';

// ============================================================================
// GET /api/chains/rpc?chainId=xxx - Get RPC endpoints
// ============================================================================

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const chainId = searchParams.get('chainId');
    const best = searchParams.get('best') === 'true';

    if (!chainId) {
      return NextResponse.json({
        success: false,
        error: 'Chain ID is required'
      }, { status: 400 });
    }

    // Get best RPC
    if (best) {
      const bestRPC = chainRegistry.getBestRPC(chainId);
      if (!bestRPC) {
        return NextResponse.json({
          success: false,
          error: `No healthy RPC endpoints for chain ${chainId}`
        }, { status: 404 });
      }
      return NextResponse.json({
        success: true,
        data: { rpc: bestRPC, chainId }
      });
    }

    // Get all RPCs for chain
    const chain = chainRegistry.getChain(chainId);
    if (!chain) {
      return NextResponse.json({
        success: false,
        error: `Chain ${chainId} not found`
      }, { status: 404 });
    }

    return NextResponse.json({
      success: true,
      data: {
        chainId,
        chainName: chain.name,
        rpcs: chain.rpcUrls,
        explorerUrls: chain.explorerUrls
      }
    });

  } catch (error: any) {
    console.error('RPC API error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Internal server error'
    }, { status: 500 });
  }
}

// ============================================================================
// POST /api/chains/rpc - Add RPC endpoint
// ============================================================================

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    
    if (!body.chainId || !body.url) {
      return NextResponse.json({
        success: false,
        error: 'Chain ID and URL are required'
      }, { status: 400 });
    }

    // Validate chain exists
    const chain = chainRegistry.getChain(body.chainId);
    if (!chain) {
      return NextResponse.json({
        success: false,
        error: `Chain ${body.chainId} not found`
      }, { status: 404 });
    }

    // Add RPC endpoint
    chainRegistry.addRPCEndpoint(body.chainId, {
      url: body.url,
      name: body.name || `${chain.name} RPC`,
      priority: body.priority || 1,
      isHealthy: true,
      latencyMs: 0,
      lastCheck: Date.now(),
      isWebSocket: body.url.startsWith('ws'),
      isBackup: body.isBackup || false,
    });

    return NextResponse.json({
      success: true,
      message: `RPC endpoint added to ${chain.name}`
    }, { status: 201 });

  } catch (error: any) {
    console.error('Add RPC error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Failed to add RPC endpoint'
    }, { status: 500 });
  }
}

// ============================================================================
// DELETE /api/chains/rpc - Remove RPC endpoint
// ============================================================================

export async function DELETE(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const chainId = searchParams.get('chainId');
    const url = searchParams.get('url');

    if (!chainId || !url) {
      return NextResponse.json({
        success: false,
        error: 'Chain ID and URL are required'
      }, { status: 400 });
    }

    chainRegistry.removeRPCEndpoint(chainId, url);

    return NextResponse.json({
      success: true,
      message: 'RPC endpoint removed'
    });

  } catch (error: any) {
    console.error('Delete RPC error:', error);
    return NextResponse.json({
      success: false,
      error: error.message || 'Failed to remove RPC endpoint'
    }, { status: 500 });
  }
}