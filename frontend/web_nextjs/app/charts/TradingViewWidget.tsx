'use client';

import React, { useEffect, useRef, useState } from 'react';
import { Box, Typography, ToggleButtonGroup, ToggleButton, Card, CardContent, Select, MenuItem, FormControl, InputLabel } from '@mui/material';

interface TradingViewWidgetProps {
  symbol?: string;
  interval?: string;
  theme?: 'light' | 'dark';
  height?: number;
  width?: string | number;
  locale?: string;
  autosize?: boolean;
}

export default function TradingViewWidget({
  symbol = 'BINANCE:ETHUSDC',
  interval = '60',
  theme = 'dark',
  height = 500,
  width = '100%',
  locale = 'en',
  autosize = true,
}: TradingViewWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [selectedSymbol, setSelectedSymbol] = useState(symbol);
  const [selectedInterval, setSelectedInterval] = useState(interval);

  // Predefined symbols
  const symbols = [
    { value: 'BINANCE:ETHUSDC', label: 'ETH/USDC' },
    { value: 'BINANCE:WBTCUSDT', label: 'WBTC/USDT' },
    { value: 'BINANCE:LINKUSDT', label: 'LINK/USDT' },
    { value: 'BINANCE:UNIUSDT', label: 'UNI/USDT' },
    { value: 'BINANCE:AAVEUSDT', label: 'AAVE/USDT' },
  ];

  // Intervals
  const intervals = [
    { value: '1', label: '1m' },
    { value: '5', label: '5m' },
    { value: '15', label: '15m' },
    { value: '60', label: '1H' },
    { value: '240', label: '4H' },
    { value: 'D', label: '1D' },
    { value: 'W', label: '1W' },
  ];

  useEffect(() => {
    // Load TradingView script
    const script = document.createElement('script');
    script.src = 'https://s3.tradingview.com/tv.js';
    script.async = true;
    script.onload = () => {
      if (typeof TradingView === 'undefined') return;

      // Create widget
      new TradingView.widget({
        autosize,
        symbol: selectedSymbol,
        interval: selectedInterval,
        timezone: 'Etc/UTC',
        theme: theme === 'dark' ? 'dark' : 'light',
        style: '1',
        locale: locale,
        toolbar_bg: theme === 'dark' ? '#1a1a2e' : '#f1f3f5',
        enable_publishing: false,
        hide_top_toolbar: false,
        hide_legend: false,
        allow_symbol_change: true,
        container_id: containerRef.current?.id,
        studies: ['RSI@tv-basicstudies', 'MASimple@tv-basicstudies'],
        overrides: {
          'mainSeriesProperties.candleStyle.upColor': '#00d4aa',
          'mainSeriesProperties.candleStyle.downColor': '#ff4757',
          'mainSeriesProperties.candleStyle.wickUpColor': '#00d4aa',
          'mainSeriesProperties.candleStyle.wickDownColor': '#ff4757',
          'mainSeriesProperties.candleStyle.borderUpColor': '#00d4aa',
          'mainSeriesProperties.candleStyle.borderDownColor': '#ff4757',
        },
        studies_defaults: {
          'RSI.plot.color': '#b3ffb3',
          'MASimple.plots.primary.color': '#00d4ff',
        },
      });
    };
    
    document.head.appendChild(script);

    return () => {
      // Cleanup
      if (script.parentNode) {
        script.parentNode.removeChild(script);
      }
    };
  }, [selectedSymbol, selectedInterval, theme, locale, autosize]);

  const handleSymbolChange = (event: any) => {
    setSelectedSymbol(event.target.value);
  };

  const handleIntervalChange = (_event: React.MouseEvent<HTMLElement>, newInterval: string | null) => {
    if (newInterval) {
      setSelectedInterval(newInterval);
    }
  };

  return (
    <Card sx={{ bgcolor: theme === 'dark' ? '#1a1a2e' : '#ffffff', borderRadius: 3, border: `1px solid ${theme === 'dark' ? '#2a2a3e' : '#e0e0e0'}` }}>
      <CardContent>
        {/* Header Controls */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <InputLabel>Pair</InputLabel>
              <Select
                value={selectedSymbol}
                label="Pair"
                onChange={handleSymbolChange}
                sx={{
                  color: theme === 'dark' ? 'white' : 'black',
                  '.MuiOutlinedInput-notchedOutline': {
                    borderColor: theme === 'dark' ? '#2a2a3e' : '#e0e0e0',
                  },
                }}
              >
                {symbols.map((s) => (
                  <MenuItem key={s.value} value={s.value}>
                    {s.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          <ToggleButtonGroup
            value={selectedInterval}
            exclusive
            onChange={handleIntervalChange}
            size="small"
            sx={{
              '& .MuiToggleButton-root': {
                color: theme === 'dark' ? '#9ca3af' : '#666',
                borderColor: theme === 'dark' ? '#2a2a3e' : '#e0e0e0',
                '&.Mui-selected': {
                  bgcolor: '#ff6b00',
                  color: 'white',
                },
              },
            }}
          >
            {intervals.map((i) => (
              <ToggleButton key={i.value} value={i.value}>
                {i.label}
              </ToggleButton>
            ))}
          </ToggleButtonGroup>
        </Box>

        {/* TradingView Container */}
        <Box
          ref={containerRef}
          id="tradingview_widget_container"
          sx={{
            width: width,
            height: height,
            '& iframe': {
              borderRadius: 2,
            },
          }}
        />

        {/* Info Footer */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 2, pt: 2, borderTop: `1px solid ${theme === 'dark' ? '#2a2a3e' : '#e0e0e0'}` }}>
          <Box>
            <Typography variant="caption" sx={{ color: theme === 'dark' ? 'gray' : '#666' }}>
              Powered by TradingView
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Typography
              variant="caption"
              component="a"
              href="https://www.tradingview.com/"
              target="_blank"
              sx={{ color: '#ff6b00', textDecoration: 'none', cursor: 'pointer' }}
            >
              Learn More
            </Typography>
            <Typography
              variant="caption"
              component="a"
              href="https://www.tradingview.com/symbols/"
              target="_blank"
              sx={{ color: '#ff6b00', textDecoration: 'none', cursor: 'pointer' }}
            >
              Full Chart
            </Typography>
          </Box>
        </Box>
      </CardContent>
    </Card>
  );
}