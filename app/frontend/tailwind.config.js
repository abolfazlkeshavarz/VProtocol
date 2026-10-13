/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // Matches the VProtocol logo's blue gradient (branding/logo.png).
        brand: {
          50: "#eef6ff",
          100: "#dbeafe",
          200: "#bfdcfe",
          300: "#8fc3fd",
          400: "#4fa3f8",
          500: "#2f7fed",
          600: "#2063d6",
          700: "#1c4fac",
          800: "#1c4288",
          900: "#1b396c",
          950: "#142447",
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
        glow: "0 0 40px -10px rgba(47, 127, 237, 0.45)",
        // Elevated button: a tight contact shadow plus a softer ambient one
        // beneath, with an inner top highlight — reads as "raised toward
        // the viewer" instead of a flat fill with a detached glow (which
        // looked like the button was sinking/floating away from the UI).
        button:
          "0 1px 0 0 rgba(255,255,255,0.18) inset, 0 -2px 0 0 rgba(0,0,0,0.12) inset, 0 6px 14px -4px rgba(47,127,237,0.55), 0 2px 4px -1px rgba(0,0,0,0.3)",
        "button-hover":
          "0 1px 0 0 rgba(255,255,255,0.22) inset, 0 -2px 0 0 rgba(0,0,0,0.12) inset, 0 8px 20px -4px rgba(47,127,237,0.65), 0 3px 6px -1px rgba(0,0,0,0.35)",
        "button-active":
          "0 1px 2px 0 rgba(0,0,0,0.25) inset, 0 2px 6px -2px rgba(47,127,237,0.4)",
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
