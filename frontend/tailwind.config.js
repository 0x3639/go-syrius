/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        bg: '#0e0f13', surface: '#1a1c22', text: '#e6e8ee', muted: '#9aa0ad',
        accent: '#4f8cff', success: '#3fb950', warn: '#d29922', error: '#f85149',
      },
      fontFamily: { mono: ['ui-monospace', 'SFMono-Regular', 'monospace'] },
    },
  },
  plugins: [],
}
