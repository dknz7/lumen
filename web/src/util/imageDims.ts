// Image-proxy dimension presets. Matches Plex Web's request shape per
// surface so we share its CDN cache and avoid Stargaze 404s on cold-miss
// permutations.

export const imageDims = {
  poster: { w: 240, h: 360 },   // Card.tsx posters, Watchlist cards
  hero:   { w: 1280, h: 720 },  // Item Detail backdrop
  person: { w: 180, h: 180 },   // Cast/Crew thumbnails (square)
} as const;

export type ImageDimPreset = keyof typeof imageDims;
