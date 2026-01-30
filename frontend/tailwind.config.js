/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{html,ts}",
  ],
  theme: {
    extend: {
      colors: {
        // Colores temáticos para una app de redes (estilo oscuro/profesional)
        'brand-primary': '#3b82f6', // blue-500
        'brand-dark': '#1e293b',    // slate-800
        'brand-accent': '#10b981',  // emerald-500
      }
    },
  },
  plugins: [],
}
