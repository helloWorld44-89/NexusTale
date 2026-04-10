import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          // All colors defined as RGB channels so Tailwind opacity modifiers
          // (e.g. bg-brand-cyan/20) work correctly with CSS variables.
          cyan:         'rgb(var(--brand-cyan) / <alpha-value>)',
          purple:       'rgb(var(--brand-purple) / <alpha-value>)',
          gold:         'rgb(var(--brand-gold) / <alpha-value>)',
          bg:           'rgb(var(--brand-bg) / <alpha-value>)',
          'bg-2':       'rgb(var(--brand-bg-2) / <alpha-value>)',
          'bg-card':    'rgb(var(--brand-bg-card) / <alpha-value>)',
          'bg-input':   'rgb(var(--brand-bg-input) / <alpha-value>)',
          text:         'rgb(var(--brand-text) / <alpha-value>)',
          muted:        'rgb(var(--brand-muted) / <alpha-value>)',
          'text-muted': 'rgb(var(--brand-text-muted) / <alpha-value>)',
          border:       'rgb(var(--brand-border) / <alpha-value>)',
        },
      },
      fontFamily: {
        sans:  ['Inter', 'system-ui', 'sans-serif'],
        serif: ['Georgia', 'serif'],
      },
      backgroundImage: {
        'brand-gradient':      'linear-gradient(135deg, #00F0FF 0%, #9F4BFF 100%)',
        'brand-gradient-gold': 'linear-gradient(135deg, #9F4BFF 0%, #F4C95D 100%)',
      },
      boxShadow: {
        'cyan-glow':   '0 0 20px rgba(0, 240, 255, 0.3)',
        'purple-glow': '0 0 20px rgba(159, 75, 255, 0.3)',
        'card':        '0 8px 32px rgba(0, 0, 0, 0.4)',
      },
      animation: {
        'pulse-slow': 'pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float':      'float 6s ease-in-out infinite',
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
