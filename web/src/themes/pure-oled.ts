import type { Theme } from "./index";

export const pureOled: Theme = {
  id: "pure-oled",
  name: "Pure OLED",
  tokens: {
    bg:           "#000000",
    bgMenu:       "#1a1a1a",
    bgElevated:   "#0f1729",
    bgInverse:    "#ffffff",
    text:         "#ffffff",
    textMuted:    "#9ca3af",
    textInverse:  "#000000",
    menuIcon:     "#d1d5db",
    border:       "#262626",
    borderSoft:   "rgba(255, 255, 255, 0.08)",
    stroke:       "#ffffff",
    statusOnline: "#4caf50",
    statusOffline:"#6b7280",
    shadow:       "0 2px 14px rgba(0, 0, 0, 0.7)",

    // Every value below is the literal these tokens replaced in component
    // CSS, so extending the token set left this theme rendering identically.
    accent:         "#2dd4bf",
    accentContrast: "#000000",
    danger:         "#f07878",
    dangerStrong:   "#c83c3c",
    warning:        "#e0b050",
    success:        "#4caf50",
    overlay:        "rgba(0, 0, 0, 0.65)",
    surfaceSubtle:  "rgba(255, 255, 255, 0.06)",
    cardEmpty:      "#262626",
    shelfOuter:     "rgba(255, 255, 255, 0.04)",
    shelfInner:     "rgba(15, 23, 41, 0.7)",
  },
};
