'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, IconButton, LinearProgress, Snackbar, Alert,
  CircularProgress, Tabs, Tab, Divider, Avatar, Badge,
  Dialog, DialogTitle, DialogContent
} from '@mui/material';
import {
  HowToVote, CheckCircle, Cancel, Schedule, PlayArrow,
  AccountBalance, Gavel, ThumbUp, ThumbDown, AccessTime,
  TrendingUp, Visibility, Close
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface Proposal {
  id: string;
  title: string;
  description: string;
  proposer: string;
  proposerAvatar?: string;
  status: 'active' | 'pending' | 'passed' | 'failed' | 'executed' | 'queued';
  type: 'governance' | 'treasury' | 'parameter' | 'partnership';
  forVotes: number;
  againstVotes: number;
  totalVotes: number;
  quorumRequired: number;
  startTime: number;
  endTime: number;
  createdAt: number;
  discussionUrl?: string;
  ipfsHash?: string;
  executionData?: string;
  forPercentage: number;
  againstPercentage: number;
  quorumPercentage: number;
  myVote?: 'for' | 'against' | 'abstain';
  myVotingPower?: number;
}

interface Vote {
  proposalId: string;
  voter: string;
  vote: 'for' | 'against' | 'abstain';
  votingPower: number;
  reason?: string;
  timestamp: number;
}

interface Delegate {
  address: string;
  name?: string;
  votingPower: number;
  delegatedPower: number;
  proposalsCreated: number;
  votesCast: number;
  since: number;
}

// ============================================================================
// Constants
// ============================================================================

const PROPOSAL_TYPES = [
  { value: 'governance', label: 'Governance', color: '#00d4aa' },
  { value: 'treasury', label: 'Treasury', color: '#ff9800' },
  { value: 'parameter', label: 'Parameter', color: '#00d4ff' },
  { value: 'partnership', label: 'Partnership', color: '#9c27b0' },
];

const STATUS_CONFIG = {
  active: { color: '#00d4aa', label: 'Active', icon: PlayArrow },
  pending: { color: '#ff9800', label: 'Pending', icon: Schedule },
  passed: { color: '#00d4aa', label: 'Passed', icon: CheckCircle },
  failed: { color: '#ff5722', label: 'Failed', icon: Cancel },
  executed: { color: '#00d4ff', label: 'Executed', icon: CheckCircle },
  queued: { color: '#9c27b0', label: 'Queued', icon: Schedule },
};

// ============================================================================
// Utility Functions
// ============================================================================

function formatAddress(address: string, chars: number = 4): string {
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

function formatNumber(num: number): string {
  if (num >= 1e9) return (num / 1e9).toFixed(2) + 'B';
  if (num >= 1e6) return (num / 1e6).toFixed(2) + 'M';
  if (num >= 1e3) return (num / 1e3).toFixed(2) + 'K';
  return num.toFixed(0);
}

function formatPercent(value: number): string {
  return `${value.toFixed(2)}%`;
}

function timeRemaining(endTime: number): string {
  const diff = endTime - Date.now();
  if (diff <= 0) return 'Ended';
  const days = Math.floor(diff / (24 * 60 * 60 * 1000));
  const hours = Math.floor((diff % (24 * 60 * 60 * 1000)) / (60 * 60 * 1000));
  if (days > 0) return `${days}d ${hours}h remaining`;
  const minutes = Math.floor((diff % (60 * 60 * 1000)) / (60 * 1000));
  return `${hours}h ${minutes}m remaining`;
}

function timeAgo(timestamp: number): string {
  const diff = Date.now() - timestamp;
  const days = Math.floor(diff / (24 * 60 * 60 * 1000));
  if (days > 0) return `${days} days ago`;
  const hours = Math.floor(diff / (60 * 60 * 1000));
  if (hours > 0) return `${hours} hours ago`;
  return 'Recently';
}

// ============================================================================
// Mock Data Generators
// ============================================================================

function generateProposals(): Proposal[] {
  return [
    {
      id: 'prop_1',
      title: 'TIGER Tokenomics v2 - Increase Emissions by 20%',
      description: 'Proposal to increase TIGER token emissions by 20% to support liquidity mining incentives for the next quarter. This will help attract more liquidity providers and increase TVL across all supported chains.',
      proposer: '0x742d35Cc6634C0532925F6e1',
      status: 'active',
      type: 'governance',
      forVotes: 45000000,
      againstVotes: 12000000,
      totalVotes: 57000000,
      quorumRequired: 100000000,
      startTime: Date.now() - 2 * 24 * 60 * 60 * 1000,
      endTime: Date.now() + 5 * 24 * 60 * 60 * 1000,
      createdAt: Date.now() - 3 * 24 * 60 * 60 * 1000,
      discussionUrl: 'https://forum.tigerswap.io/t/prop-1-tiger-tokenomics-v2',
      forPercentage: 78.9,
      againstPercentage: 21.1,
      quorumPercentage: 57,
      myVote: undefined,
      myVotingPower: 5000,
    },
    {
      id: 'prop_2',
      title: 'Treasury Diversification - Convert 10% to USDC',
      description: 'Diversify protocol treasury by converting 10% of TIGER holdings to USDC for operational expenses and risk management.',
      proposer: '0x8Ba1f109551bD432803012645Ac136ddd64DBA72',
      status: 'active',
      type: 'treasury',
      forVotes: 28000000,
      againstVotes: 5000000,
      totalVotes: 33000000,
      quorumRequired: 100000000,
      startTime: Date.now() - 1 * 24 * 60 * 60 * 1000,
      endTime: Date.now() + 6 * 24 * 60 * 60 * 1000,
      createdAt: Date.now() - 2 * 24 * 60 * 60 * 1000,
      forPercentage: 84.8,
      againstPercentage: 15.2,
      quorumPercentage: 33,
      myVote: 'for',
      myVotingPower: 5000,
    },
    {
      id: 'prop_3',
      title: 'Add Support for Solana Chain',
      description: 'Expand multi-chain support by adding Solana blockchain integration, enabling swaps and liquidity provision on Solana DEXs.',
      proposer: '0x3d44D6C896D2346484d5E05b6e3cEb5c5c5d5F9e',
      status: 'pending',
      type: 'governance',
      forVotes: 0,
      againstVotes: 0,
      totalVotes: 0,
      quorumRequired: 100000000,
      startTime: Date.now() + 2 * 24 * 60 * 60 * 1000,
      endTime: Date.now() + 9 * 24 * 60 * 60 * 1000,
      createdAt: Date.now() - 1 * 24 * 60 * 60 * 1000,
      forPercentage: 0,
      againstPercentage: 0,
      quorumPercentage: 0,
    },
    {
      id: 'prop_4',
      title: 'Reduce Trading Fee from 0.3% to 0.25%',
      description: 'Parameter adjustment proposal to reduce the base trading fee from 0.3% to 0.25% to remain competitive with other top DEXs.',
      proposer: '0x5B2b5C9A8B4E3D5f6E8c1A2B3D4E5F6A7B8C9D0e',
      status: 'passed',
      type: 'parameter',
      forVotes: 85000000,
      againstVotes: 15000000,
      totalVotes: 100000000,
      quorumRequired: 100000000,
      startTime: Date.now() - 10 * 24 * 60 * 60 * 1000,
      endTime: Date.now() - 3 * 24 * 60 * 60 * 1000,
      createdAt: Date.now() - 12 * 24 * 60 * 60 * 1000,
      forPercentage: 85,
      againstPercentage: 15,
      quorumPercentage: 100,
      myVote: 'for',
    },
    {
      id: 'prop_5',
      title: 'Partnership with Circle for Native USDC Support',
      description: 'Establish deeper integration with Circle for native USDC minting and improved cross-chain USDC transfers.',
      proposer: '0x9C8B2E4c6D8F0A2B3C4D5E6F7A8B9C0D1E2F3A4b',
      status: 'executed',
      type: 'partnership',
      forVotes: 95000000,
      againstVotes: 5000000,
      totalVotes: 100000000,
      quorumRequired: 100000000,
      startTime: Date.now() - 20 * 24 * 60 * 60 * 1000,
      endTime: Date.now() - 13 * 24 * 60 * 60 * 1000,
      createdAt: Date.now() - 22 * 24 * 60 * 60 * 1000,
      forPercentage: 95,
      againstPercentage: 5,
      quorumPercentage: 100,
      myVote: 'for',
    },
  ];
}

function generateTopHolders(): Delegate[] {
  return [
    { address: '0x742d35Cc6634C0532925F6e1', name: 'TigerDAO Multisig', votingPower: 25000000, delegatedPower: 0, proposalsCreated: 5, votesCast: 42, since: Date.now() - 180 * 24 * 60 * 60 * 1000 },
    { address: '0x8Ba1f109551bD432803012645Ac136ddd64DBA72', name: 'Aave Grants', votingPower: 15000000, delegatedPower: 8000000, proposalsCreated: 2, votesCast: 38, since: Date.now() - 120 * 24 * 60 * 60 * 1000 },
    { address: '0x3d44D6C896D2346484d5E05b6e3cEb5c5c5d5F9e', name: '链上巨鲸', votingPower: 12000000, delegatedPower: 5000000, proposalsCreated: 8, votesCast: 65, since: Date.now() - 90 * 24 * 60 * 60 * 1000 },
    { address: '0x5B2b5C9A8B4E3D5f6E8c1A2B3D4E5F6A7B8C9D0e', name: 'Wintermute Trading', votingPower: 8000000, delegatedPower: 2000000, proposalsCreated: 1, votesCast: 25, since: Date.now() - 60 * 24 * 60 * 60 * 1000 },
    { address: '0x9C8B2E4c6D8F0A2B3C4D5E6F7A8B9C0D1E2F3A4b', name: 'Jump Crypto', votingPower: 6000000, delegatedPower: 1500000, proposalsCreated: 3, votesCast: 30, since: Date.now() - 45 * 24 * 60 * 60 * 1000 },
  ];
}

// ============================================================================
// Main Governance Page Component
// ============================================================================

export default function GovernancePage() {
  // State
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [topHolders, setTopHolders] = useState<Delegate[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState(0);
  const [selectedProposal, setSelectedProposal] = useState<Proposal | null>(null);
  const [votingPower, setVotingPower] = useState(5000);
  const [delegating, setDelegating] = useState(false);
  const [delegateAddress, setDelegateAddress] = useState('');
  const [showProposalDetail, setShowProposalDetail] = useState(false);

  // Snackbar
  const [snackbar, setSnackbar] = useState({
    open: false,
    message: '',
    severity: 'success' as 'success' | 'error' | 'info'
  });

  // ============================================================================
  // Data Loading
  // ============================================================================

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      setProposals(generateProposals());
      setTopHolders(generateTopHolders());
    } catch (error) {
      console.error('Failed to load governance data:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // ============================================================================
  // Voting Functions
  // ============================================================================

  const handleVote = async (proposalId: string, vote: 'for' | 'against' | 'abstain') => {
    try {
      await new Promise(resolve => setTimeout(resolve, 1000));

      setProposals(prev => prev.map(p => {
        if (p.id !== proposalId) return p;

        const newForVotes = vote === 'for' ? p.forVotes + votingPower : p.forVotes;
        const newAgainstVotes = vote === 'against' ? p.againstVotes + votingPower : p.againstVotes;
        const newTotalVotes = newForVotes + newAgainstVotes;

        return {
          ...p,
          forVotes: newForVotes,
          againstVotes: newAgainstVotes,
          totalVotes: newTotalVotes,
          forPercentage: (newForVotes / newTotalVotes) * 100,
          againstPercentage: (newAgainstVotes / newTotalVotes) * 100,
          myVote: vote,
        };
      }));

      setSnackbar({ open: true, message: `Vote cast successfully!`, severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to cast vote', severity: 'error' });
    }
  };

  const handleDelegate = async () => {
    if (!delegateAddress) {
      setSnackbar({ open: true, message: 'Please enter an address', severity: 'error' });
      return;
    }

    setDelegating(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 1500));
      setSnackbar({ open: true, message: `Delegated ${votingPower.toLocaleString()} votes to ${formatAddress(delegateAddress)}`, severity: 'success' });
      setDelegateAddress('');
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to delegate', severity: 'error' });
    } finally {
      setDelegating(false);
    }
  };

  const handleQueueExecution = async (proposalId: string) => {
    try {
      await new Promise(resolve => setTimeout(resolve, 1000));
      setProposals(prev => prev.map(p =>
        p.id === proposalId ? { ...p, status: 'executed' as const } : p
      ));
      setSnackbar({ open: true, message: 'Proposal executed successfully!', severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to execute proposal', severity: 'error' });
    }
  };

  // ============================================================================
  // Statistics
  // ============================================================================

  const activeProposals = proposals.filter(p => p.status === 'active').length;
  const totalVotes = proposals.reduce((sum, p) => sum + p.totalVotes, 0);
  const passedProposals = proposals.filter(p => p.status === 'passed' || p.status === 'executed').length;

  // ============================================================================
  // Render Helpers
  // ============================================================================

  const renderStatusChip = (status: Proposal['status']) => {
    const config = STATUS_CONFIG[status];
    const Icon = config.icon;
    return (
      <Chip
        icon={<Icon sx={{ fontSize: 16 }} />}
        label={config.label}
        size="small"
        sx={{
          bgcolor: `${config.color}20`,
          color: config.color,
          '& .MuiChip-icon': { color: config.color },
        }}
      />
    );
  };

  const renderTypeChip = (type: Proposal['type']) => {
    const config = PROPOSAL_TYPES.find(t => t.value === type);
    return (
      <Chip
        label={config?.label || type}
        size="small"
        sx={{
          bgcolor: `${config?.color}20`,
          color: config?.color,
        }}
      />
    );
  };

  const renderVoteBar = (proposal: Proposal) => {
    return (
      <Box sx={{ width: '100%' }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography variant="caption" sx={{ color: '#00d4aa' }}>
            For {formatPercent(proposal.forPercentage)}
          </Typography>
          <Typography variant="caption" sx={{ color: '#ff5722' }}>
            Against {formatPercent(proposal.againstPercentage)}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', height: 8, borderRadius: 4, overflow: 'hidden', bgcolor: '#ff572220' }}>
          <Box sx={{ width: `${proposal.forPercentage}%`, bgcolor: '#00d4aa', transition: 'width 0.3s' }} />
        </Box>
      </Box>
    );
  };

  // ============================================================================
  // Main Render
  // ============================================================================

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Box>
            <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
              🏛️ Governance
            </Typography>
            <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
              Participate in protocol decisions and shape the future of TigerSwap
            </Typography>
          </Box>
        </Box>

        {/* Stats */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Your Voting Power</Typography>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {votingPower.toLocaleString()}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Active Proposals</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                {activeProposals}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Votes Cast</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                {formatNumber(totalVotes)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Proposals Passed</Typography>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {passedProposals}
              </Typography>
            </CardContent>
          </Card>
        </Box>

        {/* Delegate Card */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 4, border: '1px solid #00d4aa30' }}>
          <CardContent sx={{ p: 3 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 2 }}>
              <Box>
                <Typography variant="h6" sx={{ color: 'white', mb: 1 }}>
                  Delegate Your Voting Power
                </Typography>
                <Typography variant="body2" sx={{ color: '#9ca3af' }}>
                  Delegate your TIGER votes to another address to participate in governance without spending gas.
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                <TextField
                  size="small"
                  placeholder="0x..."
                  value={delegateAddress}
                  onChange={(e) => setDelegateAddress(e.target.value)}
                  sx={{
                    width: 250,
                    '& input': { color: 'white' },
                    '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
                  }}
                />
                <Button
                  variant="contained"
                  onClick={handleDelegate}
                  disabled={delegating}
                  sx={{ bgcolor: '#00d4aa', color: 'black' }}
                >
                  {delegating ? <CircularProgress size={20} sx={{ color: 'black' }} /> : 'Delegate'}
                </Button>
              </Box>
            </Box>
          </CardContent>
        </Card>

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Tabs
            value={activeTab}
            onChange={(_, v) => setActiveTab(v)}
            sx={{
              borderBottom: '1px solid #2a2a3e',
              '& .MuiTab-root': { color: '#9ca3af' },
              '& .Mui-selected': { color: '#00d4aa' },
            }}
          >
            <Tab label="All Proposals" />
            <Tab label="Active" />
            <Tab label="My Votes" />
            <Tab label="Top Holders" />
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {loading ? (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 5 }}>
                <CircularProgress sx={{ color: '#00d4aa' }} />
              </Box>
            ) : (
              <Box>
                {/* All Proposals Tab */}
                {activeTab === 0 && (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell sx={{ color: '#9ca3af' }}>Proposal</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Type</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Votes</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Time Remaining</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Action</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {proposals.map(proposal => (
                          <TableRow key={proposal.id} sx={{ '&:hover': { bgcolor: '#2a2a3e', cursor: 'pointer' } }}
                            onClick={() => { setSelectedProposal(proposal); setShowProposalDetail(true); }}>
                            <TableCell>
                              <Box>
                                <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{proposal.title}</Typography>
                                <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                                  by {formatAddress(proposal.proposer)} • {timeAgo(proposal.createdAt)}
                                </Typography>
                              </Box>
                            </TableCell>
                            <TableCell>{renderTypeChip(proposal.type)}</TableCell>
                            <TableCell>{renderStatusChip(proposal.status)}</TableCell>
                            <TableCell>
                              <Box sx={{ minWidth: 120 }}>
                                {renderVoteBar(proposal)}
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Typography sx={{ color: proposal.status === 'active' ? '#00d4aa' : '#9ca3af', fontSize: '0.85rem' }}>
                                {proposal.status === 'active' ? timeRemaining(proposal.endTime) : STATUS_CONFIG[proposal.status].label}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              {proposal.status === 'active' && (
                                <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                                  <Button
                                    size="small"
                                    variant="contained"
                                    startIcon={<ThumbUp sx={{ fontSize: 14 }} />}
                                    onClick={(e) => { e.stopPropagation(); handleVote(proposal.id, 'for'); }}
                                    disabled={proposal.myVote !== undefined}
                                    sx={{ bgcolor: '#00d4aa', color: 'black', minWidth: 0, px: 1 }}
                                  />
                                  <Button
                                    size="small"
                                    variant="outlined"
                                    startIcon={<ThumbDown sx={{ fontSize: 14 }} />}
                                    onClick={(e) => { e.stopPropagation(); handleVote(proposal.id, 'against'); }}
                                    disabled={proposal.myVote !== undefined}
                                    sx={{ borderColor: '#ff5722', color: '#ff5722', minWidth: 0, px: 1 }}
                                  />
                                </Box>
                              )}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}

                {/* Active Tab */}
                {activeTab === 1 && (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell sx={{ color: '#9ca3af' }}>Proposal</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Votes</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Time Remaining</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Action</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {proposals.filter(p => p.status === 'active').map(proposal => (
                          <TableRow key={proposal.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                            <TableCell>
                              <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{proposal.title}</Typography>
                            </TableCell>
                            <TableCell>
                              <Box sx={{ minWidth: 120 }}>{renderVoteBar(proposal)}</Box>
                            </TableCell>
                            <TableCell>
                              <Typography sx={{ color: '#00d4aa' }}>{timeRemaining(proposal.endTime)}</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                                <Button
                                  size="small"
                                  variant="contained"
                                  startIcon={<ThumbUp sx={{ fontSize: 14 }} />}
                                  onClick={() => handleVote(proposal.id, 'for')}
                                  disabled={proposal.myVote !== undefined}
                                  sx={{ bgcolor: '#00d4aa', color: 'black' }}
                                >
                                  Vote For
                                </Button>
                                <Button
                                  size="small"
                                  variant="outlined"
                                  startIcon={<ThumbDown sx={{ fontSize: 14 }} />}
                                  onClick={() => handleVote(proposal.id, 'against')}
                                  disabled={proposal.myVote !== undefined}
                                  sx={{ borderColor: '#ff5722', color: '#ff5722' }}
                                >
                                  Against
                                </Button>
                              </Box>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}

                {/* My Votes Tab */}
                {activeTab === 2 && (
                  <Box>
                    {proposals.filter(p => p.myVote !== undefined).length === 0 ? (
                      <Box sx={{ textAlign: 'center', py: 5 }}>
                        <Typography sx={{ color: '#9ca3af' }}>You haven't voted on any proposals yet.</Typography>
                      </Box>
                    ) : (
                      <TableContainer>
                        <Table>
                          <TableHead>
                            <TableRow>
                              <TableCell sx={{ color: '#9ca3af' }}>Proposal</TableCell>
                              <TableCell sx={{ color: '#9ca3af' }}>My Vote</TableCell>
                              <TableCell sx={{ color: '#9ca3af' }}>Current Results</TableCell>
                              <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                            </TableRow>
                          </TableHead>
                          <TableBody>
                            {proposals.filter(p => p.myVote !== undefined).map(proposal => (
                              <TableRow key={proposal.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                                <TableCell>
                                  <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{proposal.title}</Typography>
                                </TableCell>
                                <TableCell>
                                  <Chip
                                    icon={proposal.myVote === 'for' ? <ThumbUp sx={{ fontSize: 14 }} /> : <ThumbDown sx={{ fontSize: 14 }} />}
                                    label={proposal.myVote === 'for' ? 'For' : 'Against'}
                                    size="small"
                                    sx={{
                                      bgcolor: proposal.myVote === 'for' ? '#00d4aa20' : '#ff572220',
                                      color: proposal.myVote === 'for' ? '#00d4aa' : '#ff5722',
                                    }}
                                  />
                                </TableCell>
                                <TableCell>
                                  <Box sx={{ minWidth: 100 }}>{renderVoteBar(proposal)}</Box>
                                </TableCell>
                                <TableCell>{renderStatusChip(proposal.status)}</TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </TableContainer>
                    )}
                  </Box>
                )}

                {/* Top Holders Tab */}
                {activeTab === 3 && (
                  <Box>
                    <Typography variant="body2" sx={{ color: '#9ca3af', mb: 3 }}>
                      Top token holders by voting power
                    </Typography>
                    <TableContainer>
                      <Table>
                        <TableHead>
                          <TableRow>
                            <TableCell sx={{ color: '#9ca3af' }}>Address</TableCell>
                            <TableCell sx={{ color: '#9ca3af' }} align="right">Voting Power</TableCell>
                            <TableCell sx={{ color: '#9ca3af' }} align="right">Delegated</TableCell>
                            <TableCell sx={{ color: '#9ca3af' }} align="right">Proposals</TableCell>
                            <TableCell sx={{ color: '#9ca3af' }} align="right">Votes Cast</TableCell>
                            <TableCell sx={{ color: '#9ca3af' }} align="right">Since</TableCell>
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {topHolders.map((holder, idx) => (
                            <TableRow key={holder.address} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                              <TableCell>
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                  <Typography sx={{ color: '#9ca3af', width: 20 }}>{idx + 1}</Typography>
                                  <Avatar sx={{ width: 24, height: 24, bgcolor: '#2a2a3e', fontSize: '0.75rem' }}>
                                    {formatAddress(holder.address, 2)}
                                  </Avatar>
                                  <Box>
                                    <Typography sx={{ color: 'white' }}>{holder.name || formatAddress(holder.address)}</Typography>
                                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>{formatAddress(holder.address)}</Typography>
                                  </Box>
                                </Box>
                              </TableCell>
                              <TableCell align="right">
                                <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{formatNumber(holder.votingPower)}</Typography>
                              </TableCell>
                              <TableCell align="right">
                                <Typography sx={{ color: '#9ca3af' }}>{formatNumber(holder.delegatedPower)}</Typography>
                              </TableCell>
                              <TableCell align="right">
                                <Typography sx={{ color: 'white' }}>{holder.proposalsCreated}</Typography>
                              </TableCell>
                              <TableCell align="right">
                                <Typography sx={{ color: 'white' }}>{holder.votesCast}</Typography>
                              </TableCell>
                              <TableCell align="right">
                                <Typography sx={{ color: '#9ca3af' }}>{timeAgo(holder.since)}</Typography>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </TableContainer>
                  </Box>
                )}
              </Box>
            )}
          </CardContent>
        </Card>

        {/* Proposal Detail Dialog */}
        <Dialog
          open={showProposalDetail}
          onClose={() => setShowProposalDetail(false)}
          maxWidth="md"
          fullWidth
          PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}
        >
          {selectedProposal && (
            <>
              <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between' }}>
                <Box>
                  <Typography variant="h6">{selectedProposal.title}</Typography>
                  <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
                    {renderTypeChip(selectedProposal.type)}
                    {renderStatusChip(selectedProposal.status)}
                  </Box>
                </Box>
                <IconButton onClick={() => setShowProposalDetail(false)} sx={{ color: 'white' }}>
                  <Close />
                </IconButton>
              </DialogTitle>
              <DialogContent>
                <Typography sx={{ color: '#9ca3af', mb: 3 }}>
                  Proposed by {formatAddress(selectedProposal.proposer)} • {timeAgo(selectedProposal.createdAt)}
                </Typography>

                <Typography sx={{ color: 'white', mb: 3, lineHeight: 1.7 }}>
                  {selectedProposal.description}
                </Typography>

                <Divider sx={{ borderColor: '#3a3a4e', my: 3 }} />

                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, mb: 3 }}>
                  <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                    <CardContent>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        <ThumbUp sx={{ color: '#00d4aa', fontSize: 20 }} />
                        <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>For Votes</Typography>
                      </Box>
                      <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                        {formatNumber(selectedProposal.forVotes)}
                      </Typography>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatPercent(selectedProposal.forPercentage)}
                      </Typography>
                    </CardContent>
                  </Card>
                  <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                    <CardContent>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        <ThumbDown sx={{ color: '#ff5722', fontSize: 20 }} />
                        <Typography sx={{ color: '#ff5722', fontWeight: 'bold' }}>Against Votes</Typography>
                      </Box>
                      <Typography variant="h5" sx={{ color: '#ff5722', fontWeight: 'bold' }}>
                        {formatNumber(selectedProposal.againstVotes)}
                      </Typography>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatPercent(selectedProposal.againstPercentage)}
                      </Typography>
                    </CardContent>
                  </Card>
                </Box>

                <Box sx={{ mb: 3 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af', mb: 1, display: 'block' }}>
                    Quorum: {selectedProposal.quorumPercentage}% (required: {formatPercent((selectedProposal.quorumRequired / (selectedProposal.forVotes + selectedProposal.againstVotes + 1) * 100))})
                  </Typography>
                  {renderVoteBar(selectedProposal)}
                </Box>

                {selectedProposal.status === 'active' && (
                  <Box sx={{ display: 'flex', gap: 2 }}>
                    <Button
                      fullWidth
                      variant="contained"
                      startIcon={<ThumbUp />}
                      onClick={() => { handleVote(selectedProposal.id, 'for'); setShowProposalDetail(false); }}
                      sx={{ bgcolor: '#00d4aa', color: 'black' }}
                    >
                      Vote For
                    </Button>
                    <Button
                      fullWidth
                      variant="outlined"
                      startIcon={<ThumbDown />}
                      onClick={() => { handleVote(selectedProposal.id, 'against'); setShowProposalDetail(false); }}
                      sx={{ borderColor: '#ff5722', color: '#ff5722' }}
                    >
                      Vote Against
                    </Button>
                  </Box>
                )}
              </DialogContent>
            </>
          )}
        </Dialog>
      </Box>

      {/* Snackbar */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert
          onClose={() => setSnackbar({ ...snackbar, open: false })}
          severity={snackbar.severity}
          sx={{ bgcolor: snackbar.severity === 'success' ? '#1b5e20' : snackbar.severity === 'error' ? '#b71c1c' : '#1a237e' }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}