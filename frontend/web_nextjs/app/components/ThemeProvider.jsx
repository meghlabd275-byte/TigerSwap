'use client';
import { createContext, useContext, useEffect, useState } from 'react';
const ThemeContext = createContext(undefined);
export function ThemeProvider({ children }) {
    // DEFAULT IS LIGHT THEME - as per requirement
    const [theme, setThemeState] = useState('light');
    const [mounted, setMounted] = useState(false);
    useEffect(() => {
        const stored = localStorage.getItem('tigerswap-theme');
        if (stored) {
            setThemeState(stored);
        }
        // Default is light theme - only use system preference if explicitly set
        setMounted(true);
    }, []);
    useEffect(() => {
        if (mounted) {
            localStorage.setItem('tigerswap-theme', theme);
            document.documentElement.classList.remove('light', 'dark');
            document.documentElement.classList.add(theme);
        }
    }, [theme, mounted]);
    const toggleTheme = () => {
        setThemeState(prev => prev === 'dark' ? 'light' : 'dark');
    };
    const setTheme = (newTheme) => {
        setThemeState(newTheme);
    };
    return (<ThemeContext.Provider value={{ theme, toggleTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>);
}
export function useTheme() {
    const context = useContext(ThemeContext);
    if (!context) {
        throw new Error('useTheme must be used within ThemeProvider');
    }
    return context;
}
//# sourceMappingURL=ThemeProvider.jsx.map