// Theme System Configuration
// Supports Light/Dark theme with system preference detection

export interface ThemeConfig {
  mode: 'light' | 'dark' | 'system';
  primaryColor: string;
  accentColor: string;
  fontFamily: string;
  borderRadius: number;
  animations: boolean;
}

export interface ThemeColors {
  // Backgrounds
  background: {
    primary: string;
    secondary: string;
    tertiary: string;
    card: string;
    modal: string;
    overlay: string;
  };
  
  // Text
  text: {
    primary: string;
    secondary: string;
    tertiary: string;
    inverse: string;
    link: string;
  };
  
  // Borders
  border: {
    default: string;
    hover: string;
    focus: string;
    active: string;
  };
  
  // Status
  status: {
    success: string;
    warning: string;
    error: string;
    info: string;
  };
  
  // Gradients
  gradient: {
    primary: string;
    secondary: string;
    card: string;
  };
}

// Light Theme Colors
export const lightTheme: ThemeColors = {
  background: {
    primary: '#FFFFFF',
    secondary: '#F8FAFC',
    tertiary: '#F1F5F9',
    card: '#FFFFFF',
    modal: '#FFFFFF',
    overlay: 'rgba(0, 0, 0, 0.5)',
  },
  
  text: {
    primary: '#0F172A',
    secondary: '#475569',
    tertiary: '#94A3B8',
    inverse: '#FFFFFF',
    link: '#2563EB',
  },
  
  border: {
    default: '#E2E8F0',
    hover: '#CBD5E1',
    focus: '#3B82F6',
    active: '#2563EB',
  },
  
  status: {
    success: '#10B981',
    warning: '#F59E0B',
    error: '#EF4444',
    info: '#3B82F6',
  },
  
  gradient: {
    primary: 'linear-gradient(135deg, #FF6B35 0%, #F7931A 100%)',
    secondary: 'linear-gradient(135deg, #1E293B 0%, #0F172A 100%)',
    card: 'linear-gradient(180deg, #FFFFFF 0%, #F8FAFC 100%)',
  },
};

// Dark Theme Colors
export const darkTheme: ThemeColors = {
  background: {
    primary: '#0B0E14',
    secondary: '#151A23',
    tertiary: '#1E2433',
    card: '#1A1F2E',
    modal: '#1E2433',
    overlay: 'rgba(0, 0, 0, 0.7)',
  },
  
  text: {
    primary: '#F8FAFC',
    secondary: '#94A3B8',
    tertiary: '#64748B',
    inverse: '#0F172A',
    link: '#60A5FA',
  },
  
  border: {
    default: '#2D3748',
    hover: '#4A5568',
    focus: '#3B82F6',
    active: '#60A5FA',
  },
  
  status: {
    success: '#34D399',
    warning: '#FBBF24',
    error: '#F87171',
    info: '#60A5FA',
  },
  
  gradient: {
    primary: 'linear-gradient(135deg, #FF6B35 0%, #F7931A 100%)',
    secondary: 'linear-gradient(135deg, #1E293B 0%, #0F172A 100%)',
    card: 'linear-gradient(180deg, #1A1F2E 0%, #151A23 100%)',
  },
};

// Default Theme Configuration
export const defaultThemeConfig: ThemeConfig = {
  mode: 'system',
  primaryColor: '#FF6B35',
  accentColor: '#F7931A',
  fontFamily: 'Inter, -apple-system, BlinkMacSystemFont, sans-serif',
  borderRadius: 12,
  animations: true,
};

// Theme Storage Keys
export const THEME_STORAGE_KEY = 'tigerswap_theme';
export const THEME_COLORS_KEY = 'tigerswap_colors';

// CSS Variables Generator
export function generateCSSVariables(colors: ThemeColors): string {
  const vars = [
    `--bg-primary: ${colors.background.primary}`,
    `--bg-secondary: ${colors.background.secondary}`,
    `--bg-tertiary: ${colors.background.tertiary}`,
    `--bg-card: ${colors.background.card}`,
    `--bg-modal: ${colors.background.modal}`,
    `--bg-overlay: ${colors.background.overlay}`,
    `--text-primary: ${colors.text.primary}`,
    `--text-secondary: ${colors.text.secondary}`,
    `--text-tertiary: ${colors.text.tertiary}`,
    `--text-inverse: ${colors.text.inverse}`,
    `--text-link: ${colors.text.link}`,
    `--border-default: ${colors.border.default}`,
    `--border-hover: ${colors.border.hover}`,
    `--border-focus: ${colors.border.focus}`,
    `--border-active: ${colors.border.active}`,
    `--status-success: ${colors.status.success}`,
    `--status-warning: ${colors.status.warning}`,
    `--status-error: ${colors.status.error}`,
    `--status-info: ${colors.status.info}`,
    `--gradient-primary: ${colors.gradient.primary}`,
    `--gradient-secondary: ${colors.gradient.secondary}`,
    `--gradient-card: ${colors.gradient.card}`,
  ];
  
  return `:root { ${vars.join('; ')} }`;
}

// Theme Context
export interface ThemeContextValue {
  theme: ThemeConfig;
  colors: ThemeColors;
  isDark: boolean;
  setTheme: (theme: ThemeConfig) => void;
  toggleTheme: () => void;
}

export default {
  lightTheme,
  darkTheme,
  defaultThemeConfig,
  generateCSSVariables,
  THEME_STORAGE_KEY,
  THEME_COLORS_KEY,
};