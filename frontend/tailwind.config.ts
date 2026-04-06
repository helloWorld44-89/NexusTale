import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          cyan:       '#00F0FF',
          purple:     '#9F4BFF',
          gold:       '#F4C95D',
          bg:         '#0F0F1A',
          'bg-2':     '#12121A',
          'bg-card':  '#1A1A2E',
          'bg-input': '#0D0D1F',
          text:       '#E0E0FF',
          muted:      '#7B7B9E',
          border:     '#2A2A4A',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      backgroundImage: {
        'brand-gradient': 'linear-gradient(135deg, #00F0FF 0%, #9F4BFF 100%)',
        'brand-gradient-gold': 'linear-gradient(135deg, #9F4BFF 0%, #F4C95D 100%)',
      },
      boxShadow: {
        'cyan-glow':   '0 0 20px rgba(0, 240, 255, 0.3)',
        'purple-glow': '0 0 20px rgba(159, 75, 255, 0.3)',
        'card':        '0 8px 32px rgba(0, 0, 0, 0.4)',
      },
      animation: {
        'pulse-slow': 'pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float': 'float 6s ease-in-out infinite',
      },
      keyframes: {
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%':      { transform: 'translateY(-10px)' },
        },
      },
    },
  },
  plugins: [],
} satisfies Config
