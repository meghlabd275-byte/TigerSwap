'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField,
  Chip, CircularProgress, Snackbar, Alert, LinearProgress,
  Tabs, Tab, Divider, Avatar
} from '@mui/material';
import {
  RocketLaunch, Timer, TrendingUp, People, Ballot,
  AccessTime, Verified, Warning, CheckCircle, Cancel
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';

// ============================================================================
// Types
// ============================================================================

interface IDOProject {
  id: string;
  name: string;
  symbol: string;
  description: string;
  logo: string;
  status: 'upcoming' | 'live' | 'completed' | 'cancelled';
  chain: string;
  chainIcon: string;
  tokenPrice: number;
  totalRaise: number;
  hardCap: number;
  softCap: number;
  startTime: number;
  endTime: number;
  minAllocation: number;
  maxAllocation: number;
  participants: number;
  totalRaised: number;
  tokenSymbol: string;
  saleTokenAddress: string;
  acceptedTokens: string[];
  vestingPercent: number;
  vestingCliff: number;
  vestingPeriod: number;
  website: string;
  twitter: string;
  telegram: string;
  whitepaper: string;
  kycRequired: boolean;
  auditStatus: 'completed' | 'in-progress' | 'pending';
}

interface UserAllocation {
  projectId: string;
  allocatedAmount: number;
  claimedAmount: number;
  claimableAmount: number;
  status: 'pending' | 'approved' | 'rejected';
}

// ============================================================================
// Mock Data
// ============================================================================

function generateProjects(): IDOProject[] {
  return [
    {
      id: 'proj_1',
      name: 'MetaFi Arena',
      symbol: 'MFA',
      description: 'Next-gen Web3 gaming platform with NFT integration and play-to-earn mechanics. Build your virtual arena and compete with players worldwide.',
      logo: '🎮',
      status: 'live',
      chain: 'Ethereum',
      chainIcon: '🔷',
      tokenPrice: 0.05,
      totalRaise: 500000,
      hardCap: 500000,
      softCap: 100000,
      startTime: Date.now() - 3600000 * 24,
      endTime: Date.now() + 3600000 * 48,
      minAllocation: 100,
      maxAllocation: 5000,
      participants: 3420,
      totalRaised: 325000,
      tokenSymbol: 'MFA',
      saleTokenAddress: '0x1234...5678',
      acceptedTokens: ['USDC', 'USDT', 'ETH'],
      vestingPercent: 10,
      vestingCliff: 0,
      vestingPeriod: 6,
      website: 'https://metafi.game',
      twitter: '@MetaFiArena',
      telegram: '@MetaFiArena',
      whitepaper: 'https://docs.metafi.game',
      kycRequired: true,
      auditStatus: 'completed',
    },
    {
      id: 'proj_2',
      name: 'Quantum DeFi',
      symbol: 'QDF',
      description: 'Advanced algorithmic stablecoin protocol with AI-powered risk management and cross-chain liquidity.',
      logo: '⚛️',
      status: 'upcoming',
      chain: 'BNB Chain',
      chainIcon: '🟡',
      tokenPrice: 0.12,
      totalRaise: 750000,
      hardCap: 750000,
      softCap: 200000,
      startTime: Date.now() + 3600000 * 72,
      endTime: Date.now() + 3600000 * 168,
      minAllocation: 50,
      maxAllocation: 10000,
      participants: 0,
      totalRaised: 0,
      tokenSymbol: 'QDF',
      saleTokenAddress: '0xabcd...efgh',
      acceptedTokens: ['USDC', 'USDT', 'BNB'],
      vestingPercent: 20,
      vestingCliff: 3,
      vestingPeriod: 9,
      website: 'https://quantumdefi.io',
      twitter: '@QuantumDeFi',
      telegram: '@QuantumDeFi',
      whitepaper: 'https://docs.quantumdefi.io',
      kycRequired: true,
      auditStatus: 'completed',
    },
    {
      id: 'proj_3',
      name: 'SolarFi Energy',
      symbol: 'SFE',
      description: 'Renewable energy staking protocol tokenizing solar and wind energy production for passive income.',
      logo: '☀️',
      status: 'live',
      chain: 'Polygon',
      chainIcon: '🟣',
      tokenPrice: 0.08,
      totalRaise: 300000,
      hardCap: 300000,
      softCap: 50000,
      startTime: Date.now() - 3600000 * 12,
      endTime: Date.now() + 3600000 * 36,
      minAllocation: 25,
      maxAllocation: 2500,
      participants: 1856,
      totalRaised: 245000,
      tokenSymbol: 'SFE',
      saleTokenAddress: '0x9876...5432',
      acceptedTokens: ['USDC', 'MATIC'],
      vestingPercent: 30,
      vestingCliff: 0,
      vestingPeriod: 6,
      website: 'https://solarfi.io',
      twitter: '@SolarFiEnergy',
      telegram: '@SolarFiEnergy',
      whitepaper: 'https://docs.solarfi.io',
      kycRequired: false,
      auditStatus: 'in-progress',
    },
    {
      id: 'proj_4',
      name: 'Nexus Finance',
      symbol: 'NXF',
      description: 'Multi-chain yield aggregator with auto-compounding vaults and institutional-grade strategies.',
      logo: '🔱',
      status: 'completed',
      chain: 'Arbitrum',
      chainIcon: '🔵',
      tokenPrice: 0.15,
      totalRaise: 1000000,
      hardCap: 1000000,
      softCap: 250000,
      startTime: Date.now() - 3600000 * 24 * 14,
      endTime: Date.now() - 3600000 * 24 * 7,
      minAllocation: 100,
      maxAllocation: 25000,
      participants: 5620,
      totalRaised: 1000000,
      tokenSymbol: 'NXF',
      saleTokenAddress: '0xfedc...ba98',
      acceptedTokens: ['USDC', 'USDT', 'ETH', 'ARB'],
      vestingPercent: 0,
      vestingCliff: 0,
      vestingPeriod: 0,
      website: 'https://nexusfinance.xyz',
      twitter: '@NexusFinance',
      telegram: '@NexusFinance',
      whitepaper: 'https://docs.nexusfinance.xyz',
      kycRequired: true,
      auditStatus: 'completed',
    },
  ];
}

// ============================================================================
// Utility Functions
// ============================================================================

function formatUSD(amount: number): string {
  if (amount >= 1e6) return `$${(amount / 1e6).toFixed(2)}M`;
  if (amount >= 1e3) return `$${(amount / 1e3).toFixed(2)}K`;
  return `$${amount.toFixed(2)}`;
}

function formatNumber(num: number): string {
  return new Intl.NumberFormat('en-US').format(num);
}

function formatDateTime(timestamp: number): string {
  return new Date(timestamp).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

function timeUntil(timestamp: number): string {
  const diff = timestamp - Date.now();
  if (diff <= 0) return 'Ended';
  const days = Math.floor(diff / (24 * 60 * 60 * 1000));
  const hours = Math.floor((diff % (24 * 60 * 60 * 1000)) / (60 * 60 * 1000));
  if (days > 0) return `${days}d ${hours}h`;
  const minutes = Math.floor((diff % (60 * 60 * 1000)) / (60 * 1000));
  return `${hours}h ${minutes}m`;
}

// ============================================================================
// Main Launchpad Page
// ============================================================================

export default function LaunchpadPage() {
  const [projects, setProjects] = useState<IDOProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState(0);
  const [selectedProject, setSelectedProject] = useState<IDOProject | null>(null);
  const [allocationAmount, setAllocationAmount] = useState('');
  const [purchasing, setPurchasing] = useState(false);
  const [claiming, setClaiming] = useState(false);

  const [snackbar, setSnackbar] = useState({
    open: false,
    message: '',
    severity: 'success' as 'success' | 'error' | 'info'
  });

  useEffect(() => {
    const loadProjects = async () => {
      setLoading(true);
      await new Promise(resolve => setTimeout(resolve, 500));
      setProjects(generateProjects());
      setLoading(false);
    };
    loadProjects();
  }, []);

  const handlePurchase = async () => {
    if (!selectedProject || !allocationAmount) return;

    setPurchasing(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 2000));
      setSnackbar({
        open: true,
        message: `Successfully allocated ${allocationAmount} USDC to ${selectedProject.name}!`,
        severity: 'success'
      });
      setSelectedProject(null);
      setAllocationAmount('');
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to complete allocation', severity: 'error' });
    } finally {
      setPurchasing(false);
    }
  };

  const handleClaim = async (projectId: string) => {
    setClaiming(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 1500));
      setSnackbar({ open: true, message: 'Tokens claimed successfully!', severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to claim tokens', severity: 'error' });
    } finally {
      setClaiming(false);
    }
  };

  const liveProjects = projects.filter(p => p.status === 'live');
  const upcomingProjects = projects.filter(p => p.status === 'upcoming');
  const completedProjects = projects.filter(p => p.status === 'completed');

  const renderStatusChip = (status: IDOProject['status']) => {
    const configs = {
      upcoming: { color: '#00d4ff', label: 'Upcoming', icon: AccessTime },
      live: { color: '#00d4aa', label: 'Live Now', icon: Timer },
      completed: { color: '#9ca3af', label: 'Completed', icon: CheckCircle },
      cancelled: { color: '#ff5722', label: 'Cancelled', icon: Cancel },
    };
    const config = configs[status];
    const Icon = config.icon;
    return (
      <Chip
        icon={<Icon sx={{ fontSize: 14 }} />}
        label={config.label}
        sx={{
          bgcolor: `${config.color}20`,
          color: config.color,
          fontWeight: 'bold'
        }}
      />
    );
  };

  const renderProgress = (raised: number, hardCap: number) => {
    const percent = Math.min((raised / hardCap) * 100, 100);
    return (
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography variant="caption" sx={{ color: '#00d4aa' }}>
            {formatUSD(raised)} raised
          </Typography>
          <Typography variant="caption" sx={{ color: '#9ca3af' }}>
            {formatUSD(hardCap)} hard cap
          </Typography>
        </Box>
        <LinearProgress
          variant="determinate"
          value={percent}
          sx={{
            height: 8,
            borderRadius: 4,
            bgcolor: '#2a2a3e',
            '& .MuiLinearProgress-bar': {
              bgcolor: percent >= 90 ? '#ff9800' : '#00d4aa',
              borderRadius: 4,
            }
          }}
        />
      </Box>
    );
  };

  const renderProjectCard = (project: IDOProject) => (
    <Card
      key={project.id}
      sx={{
        bgcolor: '#2a2a3e',
        borderRadius: 3,
        cursor: 'pointer',
        transition: 'transform 0.2s, border-color 0.2s',
        '&:hover': {
          transform: 'translateY(-4px)',
          borderColor: '#00d4aa'
        },
        border: '2px solid transparent'
      }}
      onClick={() => { setSelectedProject(project); setAllocationAmount(''); }}
    >
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Typography sx={{ fontSize: 48 }}>{project.logo}</Typography>
            <Box>
              <Typography variant="h6" sx={{ color: 'white', fontWeight: 'bold' }}>
                {project.name}
              </Typography>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                {project.symbol} • {project.chainIcon} {project.chain}
              </Typography>
            </Box>
          </Box>
          {renderStatusChip(project.status)}
        </Box>

        <Typography variant="body2" sx={{ color: '#9ca3af', mb: 2, minHeight: 40, overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {project.description}
        </Typography>

        {project.status !== 'upcoming' && (
          <Box sx={{ mb: 2 }}>
            {renderProgress(project.totalRaised, project.hardCap)}
          </Box>
        )}

        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 2, mb: 2 }}>
          <Box>
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>Token Price</Typography>
            <Typography sx={{ color: 'white', fontWeight: 'bold' }}>
              ${project.tokenPrice}
            </Typography>
          </Box>
          <Box>
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>
              {project.status === 'upcoming' ? 'Starts In' : 'Time Left'}
            </Typography>
            <Typography sx={{ color: project.status === 'live' ? '#00d4aa' : 'white', fontWeight: 'bold' }}>
              {timeUntil(project.status === 'upcoming' ? project.startTime : project.endTime)}
            </Typography>
          </Box>
          <Box>
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>Participants</Typography>
            <Typography sx={{ color: 'white', fontWeight: 'bold' }}>
              {formatNumber(project.participants)}
            </Typography>
          </Box>
        </Box>

        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {project.kycRequired && (
            <Chip label="KYC Required" size="small" sx={{ bgcolor: '#ff980020', color: '#ff9800', fontSize: '0.65rem' }} />
          )}
          {project.auditStatus === 'completed' && (
            <Chip label="✅ Audited" size="small" sx={{ bgcolor: '#00d4aa20', color: '#00d4aa', fontSize: '0.65rem' }} />
          )}
          {project.vestingPercent > 0 && (
            <Chip label={`${project.vestingPercent}% TGE`} size="small" sx={{ bgcolor: '#00d4ff20', color: '#00d4ff', fontSize: '0.65rem' }} />
          )}
        </Box>

        {project.status === 'live' && (
          <Button
            fullWidth
            variant="contained"
            sx={{
              mt: 2,
              bgcolor: '#00d4aa',
              color: 'black',
              fontWeight: 'bold',
              '&:hover': { bgcolor: '#00b894' }
            }}
            onClick={(e) => { e.stopPropagation(); setSelectedProject(project); }}
          >
            Participate Now
          </Button>
        )}
        {project.status === 'upcoming' && (
          <Button
            fullWidth
            variant="outlined"
            sx={{
              mt: 2,
              borderColor: '#00d4aa',
              color: '#00d4aa',
              '&:hover': { bgcolor: '#00d4aa20' }
            }}
            onClick={(e) => { e.stopPropagation(); setSelectedProject(project); }}
          >
            View Details
          </Button>
        )}
      </CardContent>
    </Card>
  );

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ mb: 4, textAlign: 'center' }}>
          <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold', mb: 1 }}>
            🚀 TigerSwap Launchpad
          </Typography>
          <Typography variant="body1" sx={{ color: '#9ca3af', maxWidth: 600, mx: 'auto' }}>
            Discover and participate in the next generation of Web3 projects. Fair launch, transparent allocation, secure investing.
          </Typography>
        </Box>

        {/* Stats */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ textAlign: 'center' }}>
              <RocketLaunch sx={{ color: '#00d4aa', fontSize: 32, mb: 1 }} />
              <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>{projects.length}</Typography>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Projects</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ textAlign: 'center' }}>
              <People sx={{ color: '#00d4aa', fontSize: 32, mb: 1 }} />
              <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
                {formatNumber(projects.reduce((s, p) => s + p.participants, 0))}
              </Typography>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Participants</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ textAlign: 'center' }}>
              <TrendingUp sx={{ color: '#00d4aa', fontSize: 32, mb: 1 }} />
              <Typography variant="h4" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {formatUSD(projects.reduce((s, p) => s + p.totalRaised, 0))}
              </Typography>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Raised</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ textAlign: 'center' }}>
              <Ballot sx={{ color: '#00d4aa', fontSize: 32, mb: 1 }} />
              <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
                {upcomingProjects.length}
              </Typography>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Upcoming</Typography>
            </CardContent>
          </Card>
        </Box>

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
          <Tabs
            value={activeTab}
            onChange={(_, v) => setActiveTab(v)}
            sx={{
              borderBottom: '1px solid #2a2a3e',
              '& .MuiTab-root': { color: '#9ca3af' },
              '& .Mui-selected': { color: '#00d4aa' }
            }}
          >
            <Tab label={`Live (${liveProjects.length})`} />
            <Tab label={`Upcoming (${upcomingProjects.length})`} />
            <Tab label={`Completed (${completedProjects.length})`} />
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {loading ? (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
                <CircularProgress sx={{ color: '#00d4aa' }} />
              </Box>
            ) : (
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(350px, 1fr))', gap: 3 }}>
                {activeTab === 0 && liveProjects.map(renderProjectCard)}
                {activeTab === 1 && upcomingProjects.map(renderProjectCard)}
                {activeTab === 2 && completedProjects.map(renderProjectCard)}
                {((activeTab === 0 && liveProjects.length === 0) ||
                  (activeTab === 1 && upcomingProjects.length === 0) ||
                  (activeTab === 2 && completedProjects.length === 0)) && (
                  <Box sx={{ gridColumn: '1 / -1', textAlign: 'center', py: 8 }}>
                    <Typography sx={{ color: '#9ca3af' }}>
                      No projects found in this category
                    </Typography>
                  </Box>
                )}
              </Box>
            )}
          </CardContent>
        </Card>
      </Box>

      {/* Project Detail Modal */}
      {selectedProject && (
        <Box
          sx={{
            position: 'fixed',
            inset: 0,
            bgcolor: 'rgba(0,0,0,0.8)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            p: 3
          }}
          onClick={() => setSelectedProject(null)}
        >
          <Card
            sx={{
              bgcolor: '#1a1a2e',
              borderRadius: 3,
              maxWidth: 600,
              width: '100%',
              maxHeight: '90vh',
              overflow: 'auto'
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <CardContent sx={{ p: 4 }}>
              {/* Header */}
              <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
                <Typography sx={{ fontSize: 64 }}>{selectedProject.logo}</Typography>
                <Box sx={{ flex: 1 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <Box>
                      <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                        {selectedProject.name}
                      </Typography>
                      <Typography sx={{ color: '#9ca3af' }}>
                        {selectedProject.symbol} • {selectedProject.chainIcon} {selectedProject.chain}
                      </Typography>
                    </Box>
                    {renderStatusChip(selectedProject.status)}
                  </Box>
                </Box>
              </Box>

              <Typography sx={{ color: '#9ca3af', mb: 3, lineHeight: 1.7 }}>
                {selectedProject.description}
              </Typography>

              {/* Stats Grid */}
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 2, mb: 3 }}>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Token Price</Typography>
                  <Typography sx={{ color: 'white', fontWeight: 'bold', fontSize: 20 }}>
                    ${selectedProject.tokenPrice}
                  </Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Raise</Typography>
                  <Typography sx={{ color: '#00d4aa', fontWeight: 'bold', fontSize: 20 }}>
                    {formatUSD(selectedProject.totalRaise)}
                  </Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                    {selectedProject.status === 'upcoming' ? 'Starts' : 'Ends'}
                  </Typography>
                  <Typography sx={{ color: 'white', fontWeight: 'bold' }}>
                    {formatDateTime(selectedProject.status === 'upcoming' ? selectedProject.startTime : selectedProject.endTime)}
                  </Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Participants</Typography>
                  <Typography sx={{ color: 'white', fontWeight: 'bold' }}>
                    {formatNumber(selectedProject.participants)}
                  </Typography>
                </Box>
              </Box>

              {/* Progress */}
              {selectedProject.status !== 'upcoming' && (
                <Box sx={{ mb: 3 }}>
                  {renderProgress(selectedProject.totalRaised, selectedProject.hardCap)}
                </Box>
              )}

              {/* Allocation Info */}
              <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2, mb: 3 }}>
                <Typography variant="caption" sx={{ color: '#9ca3af', mb: 1, display: 'block' }}>
                  Allocation Range
                </Typography>
                <Typography sx={{ color: 'white' }}>
                  {formatUSD(selectedProject.minAllocation)} - {formatUSD(selectedProject.maxAllocation)}
                </Typography>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                  Accepted: {selectedProject.acceptedTokens.join(', ')}
                </Typography>
              </Box>

              {/* Vesting */}
              {selectedProject.vestingPercent > 0 && (
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2, mb: 3 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af', mb: 1, display: 'block' }}>
                    Vesting Schedule
                  </Typography>
                  <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                    {selectedProject.vestingPercent}% at TGE
                    {selectedProject.vestingCliff > 0 && `, ${selectedProject.vestingCliff} month cliff`}
                    {selectedProject.vestingPeriod > 0 && `, ${selectedProject.vestingPeriod} months vesting`}
                  </Typography>
                </Box>
              )}

              {/* Purchase Form */}
              {selectedProject.status === 'live' && (
                <Box>
                  <TextField
                    fullWidth
                    type="number"
                    label="Allocation Amount (USDC)"
                    value={allocationAmount}
                    onChange={(e) => setAllocationAmount(e.target.value)}
                    sx={{
                      mb: 2,
                      '& .MuiInputLabel-root': { color: '#9ca3af' },
                      '& input': { color: 'white' },
                      '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } }
                    }}
                  />
                  <Button
                    fullWidth
                    variant="contained"
                    size="large"
                    disabled={purchasing || !allocationAmount}
                    onClick={handlePurchase}
                    sx={{
                      bgcolor: '#00d4aa',
                      color: 'black',
                      fontWeight: 'bold',
                      py: 1.5,
                      '&:hover': { bgcolor: '#00b894' }
                    }}
                  >
                    {purchasing ? <CircularProgress size={24} sx={{ color: 'black' }} /> : 'Confirm Allocation'}
                  </Button>
                </Box>
              )}

              {/* Links */}
              <Box sx={{ display: 'flex', gap: 2, mt: 3 }}>
                <Button size="small" href={selectedProject.website} target="_blank" sx={{ color: '#00d4aa' }}>
                  Website
                </Button>
                <Button size="small" href={`https://twitter.com/${selectedProject.twitter}`} target="_blank" sx={{ color: '#00d4aa' }}>
                  Twitter
                </Button>
                <Button size="small" href={selectedProject.whitepaper} target="_blank" sx={{ color: '#00d4aa' }}>
                  Docs
                </Button>
              </Box>
            </CardContent>
          </Card>
        </Box>
      )}

      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={snackbar.severity} sx={{ bgcolor: snackbar.severity === 'success' ? '#1b5e20' : snackbar.severity === 'error' ? '#b71c1c' : '#1a237e' }}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}