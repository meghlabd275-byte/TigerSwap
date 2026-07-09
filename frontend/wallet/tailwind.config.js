/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#fef7ee',
          100: '#fdedd6',
          200: '#fad7ae',
          300: '#f6b87d',
          400: '#f29042',
          500: '#ee700d',
          600: '#de5808',
          700: '#b84307',
          800: '#933509',
          900: '#762c08',
        },
        tiger: {
          orange: '#FF6B35',
          yellow: '#FFD23F',
          black: '#1A1A1A',
          dark: '#0D0D0D',
          light: '#F7F7F7',
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'bounce-slow': 'bounce 2s infinite',
      },
    },
  },
  plugins: [],
}
