// Renders Lumen's brand mark to PNGs at every Windows icon size.
//
// The mark is Lucide's "Sparkles" glyph — the same one the SPA shows in the
// top-left brand pill (TopBar.tsx: <span class="logo"><Sparkles/></span>), so
// the window, taskbar and tray icon match what's on screen.
//
// Paths lifted verbatim from lucide-solid/dist/esm/icons/sparkles.mjs (ISC).
// Colours from web/src/themes/pure-oled.ts: bgElevated #0f1729, text #ffffff.
const { chromium } = require("playwright");
const fs = require("fs");
const path = require("path");

const OUT = path.join(__dirname, "icon-png");
const SIZES = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256];

// Full mark. Below 32px the plus and the dot collapse into noise, so small
// frames use STAR_ONLY instead — the standard simplify-at-small-sizes practice,
// not a shortcut.
const SPARKLES = `
  <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/>
  <path d="M20 2v4"/>
  <path d="M22 4h-4"/>
  <circle cx="4" cy="20" r="2"/>
`;

const STAR_ONLY = `
  <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/>
`;

// The glyph is inset inside a rounded square so it reads on both light and
// dark taskbars — a bare white glyph disappears on a light theme.
function page(size) {
  // Stroke has to thicken at small sizes or the mark turns to mush at 16px.
  const simplified = size < 32;
  const glyph = simplified ? STAR_ONLY : SPARKLES;
  // The star alone occupies less of the 24x24 box, so it can take a tighter
  // inset and a heavier stroke without closing up.
  const stroke = simplified ? 2.2 : size <= 32 ? 2.0 : 1.75;
  const box = simplified ? "2 1 20 20" : "0 0 24 24";
  const pad = Math.max(1, Math.round(size * (simplified ? 0.08 : 0.145)));
  const inner = size - pad * 2;
  const radius = Math.round(size * 0.22);
  return `<!doctype html><html><head><meta charset="utf-8"><style>
    *{margin:0;padding:0}
    html,body{background:transparent}
    .icon{
      width:${size}px;height:${size}px;
      background:#0f1729;
      border-radius:${radius}px;
      display:flex;align-items:center;justify-content:center;
    }
  </style></head><body>
    <div class="icon">
      <svg width="${inner}" height="${inner}" viewBox="${box}" fill="none"
           stroke="#ffffff" stroke-width="${stroke}"
           stroke-linecap="round" stroke-linejoin="round">${glyph}</svg>
    </div>
  </body></html>`;
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();
  for (const size of SIZES) {
    const ctx = await browser.newContext({
      viewport: { width: size, height: size },
      deviceScaleFactor: 1,
    });
    const p = await ctx.newPage();
    await p.setContent(page(size));
    const file = path.join(OUT, `icon-${size}.png`);
    await p.screenshot({ path: file, omitBackground: true });
    await ctx.close();
    console.log("rendered", file);
  }
  await browser.close();
  console.log("done");
})();
