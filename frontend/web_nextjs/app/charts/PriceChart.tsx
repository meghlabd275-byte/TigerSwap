'use client';

import React, { useEffect, useRef, useState } from 'react';
import { Box, Typography, ToggleButtonGroup, ToggleButton, Card, CardContent } from '@mui/material';

interface CandlestickData {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume?: number;
}

interface PriceChartProps {
  tokenPair?: string;
  tokenIn?: string;
  tokenOut?: string;
  height?: number;
  showVolume?: boolean;
  showOrderBook?: boolean;
}

// Generate realistic candlestick data
function generateCandlestickData(
  basePrice: number,
  days: number,
  volatility: number = 0.02
): CandlestickData[] {
  const data: CandlestickData[] = [];
  let currentPrice = basePrice;
  const now = new Date();
  
  for (let i = days; i >= 0; i--) {
    const date = new Date(now);
    date.setDate(date.getDate() - i);
    
    // Random OHLC with realistic movement
    const change = (Math.random() - 0.5) * 2 * volatility * currentPrice;
    const open = currentPrice;
    const close = currentPrice + change;
    const high = Math.max(open, close) + Math.random() * volatility * currentPrice * 0.5;
    const low = Math.min(open, close) - Math.random() * volatility * currentPrice * 0.5;
    
    data.push({
      time: date.toISOString().split('T')[0],
      open,
      high,
      low,
      close,
      volume: Math.random() * 1000000 + 100000
    });
    
    currentPrice = close;
  }
  
  return data;
}

export default function PriceChart({
  tokenPair = 'ETH_USDC',
  tokenIn,
  tokenOut,
  height = 400,
  showVolume = true,
  showOrderBook = false
}: PriceChartProps) {
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<any>(null);
  const candlestickSeriesRef = useRef<any>(null);
  const volumeSeriesRef = useRef<any>(null);
  
  const [timeframe, setTimeframe] = useState('1D');
  const [currentPrice, setCurrentPrice] = useState(2450.0);
  const [priceChange, setPriceChange] = useState(2.35);
  const [priceChangePercent, setPriceChangePercent] = useState(0.096);
  const [high24h, setHigh24h] = useState(2480.0);
  const [low24h, setLow24h] = useState(2420.0);
  const [volume24h, setVolume24h] = useState(1250000000);
  const [chartType, setChartType] = useState<'candle' | 'line'>('candle');

  useEffect(() => {
    // Dynamically import lightweight-charts
    const initChart = async () => {
      if (typeof window === 'undefined') return;
      
      try {
        // @ts-ignore
        const { createChart, ColorType, CrosshairMode } = await import('lightweight-charts');
        
        if (!chartContainerRef.current || chartRef.current) return;
        
        const chart = createChart(chartContainerRef.current, {
          layout: {
            background: { type: ColorType.Solid, color: '#1a1a2e' },
            textColor: '#9ca3af',
          },
          grid: {
            vertLines: { color: '#2a2a3e' },
            horzLines: { color: '#2a2a3e' },
          },
          width: chartContainerRef.current.clientWidth,
          height: height,
          crosshair: {
            mode: CrosshairMode.Normal,
          },
          timeScale: {
            borderColor: '#2a2a3e',
            timeVisible: true,
          },
          rightPriceScale: {
            borderColor: '#2a2a3e',
          },
        });
        
        chartRef.current = chart;
        
        // Add candlestick series
        const candlestickSeries = chart.addCandlestickSeries({
          upColor: '#00d4aa',
          downColor: '#ff4757',
          borderVisible: false,
          wickUpColor: '#00d4aa',
          wickDownColor: '#ff4757',
        });
        candlestickSeriesRef.current = candlestickSeries;
        
        // Add volume series
        if (showVolume) {
          const volumeSeries = chart.addHistogramSeries({
            color: '#26a69a',
            priceFormat: {
              type: 'volume',
            },
            priceScaleId: '',
          });
          volumeSeries.priceScale().applyOptions({
            scaleMargins: {
              top: 0.8,
              bottom: 0,
            },
          });
          volumeSeriesRef.current = volumeSeries;
        }
        
        // Determine base price from token pair
        let basePrice = 2450.0;
        if (tokenPair.includes('BTC') || tokenPair.includes('WBTC')) {
          basePrice = 62500.0;
        } else if (tokenPair.includes('LINK')) {
          basePrice = 18.5;
        } else if (tokenPair.includes('UNI')) {
          basePrice = 12.5;
        } else if (tokenPair.includes('AAVE')) {
          basePrice = 285.0;
        }
        
        // Generate data based on timeframe
        let days = 30;
        if (timeframe === '1H') days = 1;
        else if (timeframe === '4H') days = 7;
        else if (timeframe === '1D') days = 30;
        else if (timeframe === '1W') days = 90;
        
        const candleData = generateCandlestickData(basePrice, days);
        
        candlestickSeries.setData(candleData);
        
        if (showVolume && volumeSeriesRef.current) {
          const volumeData = candleData.map(c => ({
            time: c.time,
            value: c.volume || 0,
            color: c.close >= c.open ? 'rgba(0, 212, 170, 0.5)' : 'rgba(255, 71, 87, 0.5)',
          }));
          volumeSeriesRef.current.setData(volumeData);
        }
        
        // Fit content
        chart.timeScale().fitContent();
        
        // Update stats
        if (candleData.length > 0) {
          const latest = candleData[candleData.length - 1];
          const first = candleData[0];
          setCurrentPrice(latest.close);
          setHigh24h(Math.max(...candleData.slice(-24).map(c => c.high)));
          setLow24h(Math.min(...candleData.slice(-24).map(c => c.low)));
          
          const change = latest.close - first.open;
          setPriceChange(change);
          setPriceChangePercent((change / first.open) * 100);
          
          const totalVolume = candleData.reduce((sum, c) => sum + (c.volume || 0), 0);
          setVolume24h(totalVolume);
        }
        
        // Handle resize
        const handleResize = () => {
          if (chartContainerRef.current && chartRef.current) {
            chartRef.current.applyOptions({
              width: chartContainerRef.current.clientWidth,
            });
          }
        };
        
        window.addEventListener('resize', handleResize);
        
        return () => {
          window.removeEventListener('resize', handleResize);
          if (chartRef.current) {
            chartRef.current.remove();
            chartRef.current = null;
          }
        };
      } catch (error) {
        console.error('Failed to load charting library:', error);
      }
    };
    
    initChart();
    
    return () => {
      if (chartRef.current) {
        chartRef.current.remove();
        chartRef.current = null;
      }
    };
  }, [tokenPair, timeframe, height, showVolume]);

  const handleTimeframeChange = (
    _event: React.MouseEvent<HTMLElement>,
    newTimeframe: string | null
  ) => {
    if (newTimeframe) {
      setTimeframe(newTimeframe);
    }
  };

  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(6);
  };

  const formatVolume = (vol: number) => {
    if (vol >= 1e9) return `$${(vol / 1e9).toFixed(2)}B`;
    if (vol >= 1e6) return `$${(vol / 1e6).toFixed(2)}M`;
    if (vol >= 1e3) return `$${(vol / 1e3).toFixed(2)}K`;
    return `$${vol.toFixed(2)}`;
  };

  return (
    <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
      <CardContent>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <Box>
            <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
              {tokenPair.replace('_', '/')}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
                ${formatPrice(currentPrice)}
              </Typography>
              <Typography
                sx={{
                  color: priceChange >= 0 ? '#00d4aa' : '#ff4757',
                  fontWeight: 'bold',
                }}
              >
                {priceChange >= 0 ? '+' : ''}{formatPrice(priceChange)} ({priceChangePercent >= 0 ? '+' : ''}{priceChangePercent.toFixed(2)}%)
              </Typography>
            </Box>
          </Box>
          
          <ToggleButtonGroup
            value={timeframe}
            exclusive
            onChange={handleTimeframeChange}
            size="small"
            sx={{
              '& .MuiToggleButton-root': {
                color: '#9ca3af',
                borderColor: '#2a2a3e',
                '&.Mui-selected': {
                  bgcolor: '#00d4ff',
                  color: 'black',
                },
              },
            }}
          >
            <ToggleButton value="1H">1H</ToggleButton>
            <ToggleButton value="4H">4H</ToggleButton>
            <ToggleButton value="1D">1D</ToggleButton>
            <ToggleButton value="1W">1W</ToggleButton>
          </ToggleButtonGroup>
        </Box>

        {/* Stats */}
        <Box sx={{ display: 'flex', gap: 4, mb: 2 }}>
          <Box>
            <Typography variant="body2" sx={{ color: 'gray' }}>24h High</Typography>
            <Typography sx={{ color: '#00d4aa' }}>${formatPrice(high24h)}</Typography>
          </Box>
          <Box>
            <Typography variant="body2" sx={{ color: 'gray' }}>24h Low</Typography>
            <Typography sx={{ color: '#ff4757' }}>${formatPrice(low24h)}</Typography>
          </Box>
          <Box>
            <Typography variant="body2" sx={{ color: 'gray' }}>24h Volume</Typography>
            <Typography sx={{ color: 'white' }}>{formatVolume(volume24h)}</Typography>
          </Box>
        </Box>

        {/* Chart */}
        <Box ref={chartContainerRef} sx={{ width: '100%', height: height }} />
        
        {/* Order Book Preview */}
        {showOrderBook && (
          <Box sx={{ mt: 2, display: 'flex', gap: 2 }}>
            <Box sx={{ flex: 1 }}>
              <Typography variant="body2" sx={{ color: 'gray', mb: 1 }}>Bids</Typography>
              {[0.5, 0.3, 0.2, 0.1, 0.05].map((pct, i) => (
                <Box key={i} sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                  <Typography variant="body2" sx={{ color: '#00d4aa' }}>
                    ${formatPrice(currentPrice * (1 - pct * 0.001))}
                  </Typography>
                  <Typography variant="body2" sx={{ color: 'white' }}>
                    {(10 - i * 2).toFixed(2)} ETH
                  </Typography>
                </Box>
              ))}
            </Box>
            <Box sx={{ flex: 1 }}>
              <Typography variant="body2" sx={{ color: 'gray', mb: 1 }}>Asks</Typography>
              {[0.05, 0.1, 0.2, 0.3, 0.5].map((pct, i) => (
                <Box key={i} sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                  <Typography variant="body2" sx={{ color: '#ff4757' }}>
                    ${formatPrice(currentPrice * (1 + pct * 0.001))}
                  </Typography>
                  <Typography variant="body2" sx={{ color: 'white' }}>
                    {(10 - i * 2).toFixed(2)} ETH
                  </Typography>
                </Box>
              ))}
            </Box>
          </Box>
        )}
      </CardContent>
    </Card>
  );
}
