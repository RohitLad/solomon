// Brand glyphs + avatar helpers for the 7 networks.
//
// Zero-dependency inline SVGs (no icon package, works offline). Each glyph's
// primary shapes use fill="currentColor" so SocialIcon.svelte can render brand
// color (default) or any monochrome override; fixed-contrast cutouts stay #fff.
// Paths are simplified brand marks — recognizable, not pixel-perfect copies.

import type { Network } from './api';

export interface SocialGlyph {
  label: string;
  brand: string; // default render color
  viewBox: string;
  body: string; // inner SVG markup (trusted static strings only)
}

const X_BODY =
  '<path fill="currentColor" d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/>';

const FB_BODY =
  '<path fill="currentColor" d="M24 12.073C24 5.405 18.627 0 12 0S0 5.405 0 12.073C0 18.1 4.388 23.094 10.125 24v-8.437H7.078v-3.49h3.047v-2.66c0-3.025 1.792-4.697 4.533-4.697 1.313 0 2.686.236 2.686.236v2.971H15.83c-1.491 0-1.956.93-1.956 1.886v2.264h3.328l-.532 3.49h-2.796V24C19.612 23.094 24 18.1 24 12.073z"/>';

const IG_BODY =
  '<rect x="2" y="2" width="20" height="20" rx="6" fill="currentColor"/>' +
  '<circle cx="12" cy="12" r="4.6" fill="none" stroke="#fff" stroke-width="2"/>' +
  '<circle cx="17.4" cy="6.6" r="1.5" fill="#fff"/>';

const YT_BODY =
  '<rect x="1.5" y="5" width="21" height="14" rx="4" fill="currentColor"/>' +
  '<path d="M10 9.3v5.4l4.8-2.7z" fill="#fff"/>';

const TIKTOK_NOTE =
  'M16.6 3c.4 2 1.8 3.6 4.4 3.9v3.2c-1.7 0-3.2-.5-4.4-1.4v6.6c0 3.9-2.9 6.2-6.3 6.2-3.4 0-6-2.6-6-6s2.7-6 6.1-6c.3 0 .7 0 1 .1v3.3c-.3-.1-.7-.2-1-.2-1.6 0-2.8 1.2-2.8 2.8s1.2 2.8 2.8 2.8c1.7 0 2.9-1.2 2.9-3V3h3.3z';
const TIKTOK_BODY =
  `<path d="${TIKTOK_NOTE}" fill="#25F4EE" transform="translate(-1.2,0)"/>` +
  `<path d="${TIKTOK_NOTE}" fill="#FE2C55" transform="translate(1.2,0)"/>` +
  `<path d="${TIKTOK_NOTE}" fill="currentColor"/>`;

const LI_BODY =
  '<rect x="1.5" y="1.5" width="21" height="21" rx="4" fill="currentColor"/>' +
  '<circle cx="7" cy="7" r="1.7" fill="#fff"/>' +
  '<rect x="5.5" y="10" width="3" height="9" fill="#fff"/>' +
  '<path d="M12 19v-5.2c0-1.5.9-2.6 2.4-2.6 1.4 0 2.1.9 2.1 2.6V19h3v-5.6c0-3-1.6-4.6-4-4.6-1.7 0-2.9.9-3.5 2V10h-3v9h3z" fill="#fff"/>';

const PIN_BODY =
  '<circle cx="12" cy="12" r="10.5" fill="currentColor"/>' +
  '<path d="M9 5h4.5c2.8 0 4.5 1.6 4.5 4s-1.7 4-4.5 4H11v6H9V5zm2 2v4h2.3c1.5 0 2.5-.8 2.5-2s-1-2-2.5-2H11z" fill="#fff"/>';

const FALLBACK_BODY =
  '<circle cx="6" cy="12" r="2.5" fill="none" stroke="currentColor" stroke-width="2"/>' +
  '<circle cx="17" cy="6" r="2.5" fill="none" stroke="currentColor" stroke-width="2"/>' +
  '<circle cx="17" cy="18" r="2.5" fill="none" stroke="currentColor" stroke-width="2"/>' +
  '<path d="M8.2 10.8l6.6-3.6M8.2 13.2l6.6 3.6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>';

export const SOCIAL_ICONS: Record<Network, SocialGlyph> = {
  twitter: { label: 'X (Twitter)', brand: '#000000', viewBox: '0 0 24 24', body: X_BODY },
  facebook: { label: 'Facebook', brand: '#1877F2', viewBox: '0 0 24 24', body: FB_BODY },
  instagram: { label: 'Instagram', brand: '#E1306C', viewBox: '0 0 24 24', body: IG_BODY },
  youtube: { label: 'YouTube', brand: '#FF0000', viewBox: '0 0 24 24', body: YT_BODY },
  tiktok: { label: 'TikTok', brand: '#010101', viewBox: '0 0 24 24', body: TIKTOK_BODY },
  linkedin: { label: 'LinkedIn', brand: '#0A66C2', viewBox: '0 0 24 24', body: LI_BODY },
  pinterest: { label: 'Pinterest', brand: '#E60023', viewBox: '0 0 24 24', body: PIN_BODY },
};

export const FALLBACK_GLYPH: SocialGlyph = {
  label: 'Unknown network',
  brand: '#64748b',
  viewBox: '0 0 24 24',
  body: FALLBACK_BODY,
};

export function socialGlyph(network: string): SocialGlyph {
  return (SOCIAL_ICONS as Record<string, SocialGlyph>)[network] ?? {
    ...FALLBACK_GLYPH,
    label: network || 'Unknown network',
  };
}

// Avatar helpers: deterministic initials + hue from any account name.
export function initials(name: string): string {
  const clean = (name ?? '').replace(/^@+/, '').trim();
  if (!clean) return '?';
  const words = clean.split(/\s+/);
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase() || '?';
  return ((words[0][0] ?? '') + (words[1][0] ?? '')).toUpperCase() || '?';
}

export function hue(name: string): number {
  let h = 5381;
  const s = name ?? '';
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) | 0;
  return Math.abs(h) % 360;
}
