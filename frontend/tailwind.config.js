/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{html,ts}",
  ],
  theme: {
    extend: {
      colors: {
        // "Signal Console" identity — teal accent + graphite neutrals,
        // replaces the old indigo/slate "AI dashboard" look.
        // `slate` itself is overridden below so every existing bg-slate-*/
        // border-slate-*/text-slate-* class picks up the new graphite tone
        // without touching each component.
        slate: {
          50: '#eef3f3',
          100: '#dbe4e5',
          200: '#b4c4c6',
          300: '#8fa3a6',
          400: '#5c6d70',
          500: '#425256',
          600: '#324044',
          700: '#28353a',
          800: '#1a2427',
          900: '#131b1e',
          950: '#0d1416',
        },
        signal: {
          50: '#eafaf8',
          100: '#cdf0ec',
          200: '#9fe0d9',
          300: '#7ed6ce',  // signal-hi: links, active mono data
          400: '#63c2ba',
          500: '#4fb3ac',  // primary accent — replaces indigo-600
          600: '#2e8f89',
          700: '#256f6b',
          800: '#1d5451',
          900: '#123230',
        },
        up: {
          400: '#a8d187',
          500: '#8fbb6c',
          600: '#6f9c4f',
        },
        pending: {
          400: '#e3b471',
          500: '#d7a153',
          600: '#b8823a',
        },
        down: {
          400: '#e2917f',
          500: '#d97b68',
          600: '#b85c4c',
        },
        'brand-primary': '#4fb3ac',
        'brand-dark': '#131b1e',
        'brand-accent': '#8fbb6c',
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
    },
  },
  plugins: [],
}
