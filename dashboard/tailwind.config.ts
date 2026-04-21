import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: [
    "./index.html",
    "./src/**/*.{ts,tsx}",
    "./node_modules/@tremor/**/*.{js,ts,jsx,tsx}"
  ],
  safelist: [
    {
      pattern: /(bg|text|border|stroke|fill)-(amber|emerald|rose|cyan|slate|blue|indigo|red|violet)-(300|400|500|600)(\/(10|20|25|30|40|50|60|70|75|80|90|100|200|300|400|500|600|700|800|900))?/,
      variants: ["hover", "dark", "dark:hover"]
    }
  ],
  theme: {
    extend: {}
  },
  plugins: []
} satisfies Config;
