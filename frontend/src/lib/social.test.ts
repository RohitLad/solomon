import { describe, expect, it } from 'vitest';
import {
  FALLBACK_GLYPH,
  SOCIAL_ICONS,
  hue,
  initials,
  socialGlyph,
} from './social';
import type { Network } from './api';

const NETWORKS: Network[] = [
  'twitter',
  'facebook',
  'instagram',
  'youtube',
  'tiktok',
  'linkedin',
  'pinterest',
];

describe('SOCIAL_ICONS', () => {
  it('covers every supported network', () => {
    for (const n of NETWORKS) {
      expect(SOCIAL_ICONS[n], n).toBeDefined();
    }
    expect(Object.keys(SOCIAL_ICONS).sort()).toEqual([...NETWORKS].sort());
  });

  it('has renderable bodies and brand colors', () => {
    for (const [id, g] of Object.entries(SOCIAL_ICONS)) {
      expect(g.body.length, `${id} body`).toBeGreaterThan(20);
      expect(g.body, `${id} shapes`).toMatch(/<(path|rect|circle|g)[ >]/);
      expect(g.brand, `${id} brand`).toMatch(/^#[0-9a-fA-F]{6}$/);
      expect(g.viewBox, `${id} viewBox`).toBe('0 0 24 24');
      expect(g.label.length, `${id} label`).toBeGreaterThan(0);
    }
  });
});

describe('socialGlyph', () => {
  it('falls back gracefully for unknown networks', () => {
    expect(socialGlyph('myspace').body).toBe(FALLBACK_GLYPH.body);
    expect(socialGlyph('myspace').label).toBe('myspace');
    expect(socialGlyph('').label).toBe('Unknown network');
  });
});

describe('initials', () => {
  it('derives up-to-2-letter caps', () => {
    expect(initials('@acme')).toBe('AC');
    expect(initials('Acme Corp')).toBe('AC');
    expect(initials('a')).toBe('A');
    expect(initials('  spaced  out  ')).toBe('SO');
  });

  it('never returns empty', () => {
    expect(initials('')).toBe('?');
    expect(initials('   ')).toBe('?');
    expect(initials('@@@')).toBe('?');
  });
});

describe('hue', () => {
  it('is deterministic and in range', () => {
    expect(hue('@acme')).toBe(hue('@acme'));
    expect(hue('@acme')).toBeGreaterThanOrEqual(0);
    expect(hue('@acme')).toBeLessThan(360);
    expect(hue('a')).not.toBe(hue('b'));
  });
});
