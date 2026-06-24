/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        bg: 'hsl(0 0% 8% / <alpha-value>)',
        surface: 'hsl(0 0% 10% / <alpha-value>)',
        elevated: 'hsl(0 0% 14% / <alpha-value>)',
        text: 'hsl(0 0% 98% / <alpha-value>)',
        muted: 'hsl(0 0% 65% / <alpha-value>)',
        border: 'hsl(0 0% 20% / <alpha-value>)',
        accent: 'hsl(145 100% 42% / <alpha-value>)',     // nom-ui green primary
        'accent-fg': 'hsl(0 0% 8% / <alpha-value>)',      // dark text on green
        qsr: 'hsl(217 100% 46% / <alpha-value>)',         // zenon blue #0061EB
        success: 'hsl(145 63% 45% / <alpha-value>)',
        warn: 'hsl(38 95% 55% / <alpha-value>)',
        error: 'hsl(352 86% 58% / <alpha-value>)',
      },
      borderRadius: { DEFAULT: '0.375rem' },
      fontFamily: {
        sans: ['"Space Grotesk Variable"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono Variable"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
    },
  },
  plugins: [],
}
