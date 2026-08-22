import type { Theme } from "./index";

/**
 * Tokyo Night (Night variant).
 *
 * Palette values are taken from the canonical source — folke/tokyonight.nvim,
 * `lua/tokyonight/colors/night.lua` layered over `storm.lua`, which is where
 * the three background overrides live and everything else is inherited. They
 * are not eyeballed from a screenshot, and not recalled: the exact hexes were
 * read out of that repository.
 *
 *   bg #1a1b26   bg_dark #16161e   bg_highlight #292e42   fg_gutter #3b4261
 *   fg #c0caf5   fg_dark #a9b1d6   comment #565f89        terminal_black #414868
 *   blue #7aa2f7 cyan #7dcfff      magenta #bb9af7        green #9ece6a
 *   red #f7768e  red1 #db4b4b      orange #ff9e64         yellow #e0af68
 *
 * Mapping notes, where the two systems do not line up one-to-one:
 *
 *  - Lumen's `bg` is the page canvas and `bgMenu` the left rail. Tokyo Night
 *    puts its darkest tone behind sidebars, so bg_dark goes to the menu and
 *    bg to the canvas — the reverse of Pure OLED, where the menu is lighter
 *    than a black page.
 *  - `bgElevated` uses bg_highlight. It tints shelves and the top-bar pill,
 *    and is the token the shelf backgrounds derive their alpha from.
 *  - `stroke` is blue rather than white. Pure OLED outlines hover states in
 *    white because everything sits on black; on a blue-grey ground white
 *    reads as harsh and the accent blue is what the palette actually uses.
 *  - `accent` is blue, not cyan. Cyan is close enough to the old teal to
 *    look like an accident rather than a choice.
 */
export const tokyoNight: Theme = {
  id: "tokyo-night",
  name: "Tokyo Night",
  tokens: {
    bg:           "#1a1b26",
    bgMenu:       "#16161e",
    bgElevated:   "#292e42",
    bgInverse:    "#c0caf5",
    text:         "#c0caf5",
    textMuted:    "#a9b1d6",
    textInverse:  "#1a1b26",
    menuIcon:     "#565f89",
    border:       "#3b4261",
    borderSoft:   "rgba(192, 202, 245, 0.08)",
    stroke:       "#7aa2f7",
    statusOnline: "#9ece6a",
    statusOffline: "#565f89",
    shadow:       "0 2px 14px rgba(13, 14, 20, 0.8)",

    accent:         "#7aa2f7",
    accentContrast: "#16161e",
    danger:         "#f7768e",
    dangerStrong:   "#db4b4b",
    warning:        "#e0af68",
    success:        "#9ece6a",
    overlay:        "rgba(13, 14, 20, 0.7)",
    surfaceSubtle:  "rgba(192, 202, 245, 0.06)",
    cardEmpty:      "#292e42",
    shelfOuter:     "rgba(192, 202, 245, 0.04)",
    shelfInner:     "rgba(41, 46, 66, 0.7)",
  },
};
