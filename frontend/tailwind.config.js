/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{html,ts}",
  ],
  theme: {
    extend: {
      colors: {
        // Graphite neutrals with restrained blue accents.
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
          50: '#eef4f8',
          100: '#d7e5ef',
          200: '#b0cddd',
          300: '#83b9df',  // signal-hi: links, active mono data
          400: '#76a4c5',
          500: '#648eac',  // primary accent — replaces indigo-600
          600: '#476d89',
          700: '#37566e',
          800: '#293f52',
          900: '#1c2d3a',
        },
        up: {
          400: '#a0b9a5',
          500: '#8eaf9f',
          600: '#648775',
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
        'brand-primary': '#648eac',
        'brand-dark': '#131b1e',
        'brand-accent': '#8eaf9f',
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
    },
  },
  plugins: [],
}
