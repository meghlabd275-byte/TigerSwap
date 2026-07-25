import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type Theme = 'light' | 'dark' | 'system';

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  resolvedTheme: 'light' | 'dark';
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'dark',
      resolvedTheme: 'dark',
      setTheme: (theme: Theme) => {
        if (theme === 'system') {
          const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
          set({ theme, resolvedTheme: isDark ? 'dark' : 'light' });
          document.documentElement.classList.toggle('dark', isDark);
        } else {
          set({ theme, resolvedTheme: theme });
          document.documentElement.classList.toggle('dark', theme === 'dark');
        }
      },
    }),
    {
      name: 'tigerswap-theme',
    }
  )
);

// Initialize theme on app load
if (typeof window !== 'undefined') {
  const store = useThemeStore.getState();
  const isDark = store.theme === 'dark' || 
    (store.theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.classList.toggle('dark', isDark);
}
