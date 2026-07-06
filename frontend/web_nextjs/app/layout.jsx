import { ThemeProvider } from './components/ThemeProvider';
import './globals.css';
export const metadata = {
    title: 'TigerSwap - Multichain DEX',
    description: 'Enterprise-grade multichain decentralized exchange with cross-chain swaps',
};
export default function RootLayout({ children, }) {
    return (<html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{
            __html: `
              (function() {
                // DEFAULT IS LIGHT THEME - as per requirement
                var theme = localStorage.getItem('tigerswap-theme');
                if (!theme) {
                  theme = 'light'; // Default to light theme
                }
                document.documentElement.classList.add(theme);
              })();
            `,
        }}/>
      </head>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>);
}
//# sourceMappingURL=layout.jsx.map