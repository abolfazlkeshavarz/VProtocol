/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        brand: {
          50: "#eefcf7",
          100: "#d6f7ea",
          200: "#aeeed6",
          300: "#79e0bc",
          400: "#44cb9d",
          500: "#1fb084",
          600: "#138e6c",
          700: "#107159",
          800: "#115a49",
          900: "#0f4a3d",
          950: "#062a23",
        },
        surface: {
          DEFAULT: "#0b0f14",
          raised: "#10151c",
          card: "#141a22",
          border: "#1f2731",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "sans-serif"],
      },
      boxShadow: {
        glow: "0 0 40px -10px rgba(31, 176, 132, 0.45)",
      },
      keyframes: {
        pulseRing: {
          "0%": { transform: "scale(0.9)", opacity: "0.7" },
          "70%": { transform: "scale(1.6)", opacity: "0" },
          "100%": { transform: "scale(1.6)", opacity: "0" },
        },
      },
      animation: {
        pulseRing: "pulseRing 2.2s cubic-bezier(0.4,0,0.6,1) infinite",
      },
    },
  },
  plugins: [],
};
