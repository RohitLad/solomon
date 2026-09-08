import { describe, expect, it } from 'vitest';
import { asArray, asRecord } from './normalize';

// Guards for the "can't access property map, exp is null" regression:
// Go nil slices serialize as `null`, so every list-typed API field must be
// normalized before the UI calls .map() / .length / {#each} on it.

describe('asArray', () => {
  it('passes real arrays through untouched', () => {
    const input = [{ id: 'a' }, { id: 'b' }];
    expect(asArray(input)).toBe(input);
    expect(asArray([])).toEqual([]);
  });

  it('converts null/undefined to []', () => {
    expect(asArray(null)).toEqual([]);
    expect(asArray(undefined)).toEqual([]);
  });

  it('converts non-array garbage to [] (stronger than ?? [])', () => {
    expect(asArray('nope' as unknown as null)).toEqual([]);
    expect(asArray({} as unknown as null)).toEqual([]);
    expect(asArray(42 as unknown as null)).toEqual([]);
  });
});

describe('asRecord', () => {
  it('passes real objects through untouched', () => {
    const input = { twitter: { max_chars: 280 } };
    expect(asRecord(input)).toBe(input);
  });

  it('converts null/undefined/arrays to {}', () => {
    expect(asRecord(null)).toEqual({});
    expect(asRecord(undefined)).toEqual({});
    expect(asRecord([] as unknown as Record<string, unknown>)).toEqual({});
  });
});

describe('original crash, documented', () => {
  it('raw null.map() throws — this is what white-screened the app', () => {
    const exp = null as unknown as { id: string }[];
    expect(() => (exp as unknown as { map: unknown }).map).toThrow(TypeError);
    expect(() => exp.map((a) => a.id)).toThrow(TypeError);
  });

  it('normalized exp.map() never throws', () => {
    const exp = null as unknown as { id: string }[];
    expect(() => new Set(asArray(exp).map((a) => a.id))).not.toThrow();
    expect(new Set(asArray(exp).map((a) => a.id))).toEqual(new Set());
  });
});

describe('fresh-DB refresh() payload (all lists null)', () => {
  it('normalizes and derives safely', () => {
    const accounts = asArray(null as unknown as { id: string; network: string }[]);
    const posts = asArray(null as unknown as { id: string }[]);
    const limits = asRecord(null as unknown as Record<string, { max_chars: number }>);
    const exp = asArray(null as unknown as { id: string }[]);

    // mirrors App.svelte refresh() + reactive derivations
    const expiringIds = new Set(exp.map((a) => a.id));
    const selectedNets = [...new Set(accounts.filter(() => false).map((a) => a.network))];

    expect(expiringIds.size).toBe(0);
    expect(selectedNets).toEqual([]);
    expect(posts.length).toBe(0);
    expect(Object.entries(limits)).toEqual([]);
    // queue/accounts tab counters
    expect(accounts.length).toBe(0);
  });
});

describe('other endpoints with null fields', () => {
  it('analytics rows null → empty table, no throw', () => {
    const rows = asArray(null as unknown as { views: number }[]);
    const totals = { views: 0, likes: 0, comments: 0, shares: 0, posts: rows.length };
    expect(rows).toEqual([]);
    expect(totals.posts).toBe(0);
  });

  it('suggest/captions/evergreen/upload null → safe', () => {
    expect(asArray(null as unknown as { at: string }[])).toEqual([]);
    expect(asArray(null as unknown as { tone: string }[])).toEqual([]);
    expect(asArray(null as unknown as { name: string }[])).toEqual([]);
    expect(asArray(null as unknown as { id: string }[]).map((a) => a.id)).toEqual([]);
  });

  it('preview targets/adaptations/errors null → safe', () => {
    const preview = { targets: null } as unknown as {
      targets: { plan: { adaptations: { detail: string }[] | null; errors: string[] | null } }[] | null;
    };
    const targets = asArray(preview.targets);
    expect(targets).toEqual([]);
    for (const t of targets) {
      expect(asArray(t.plan.adaptations)).toEqual([]);
      expect(asArray(t.plan.errors)).toEqual([]);
    }
  });

  it('post targets/media null → safe (scheduled tab)', () => {
    const post = { targets: null, media: null } as unknown as {
      targets: { status: string }[] | null;
      media: { id: string }[] | null;
    };
    expect(asArray(post.targets)).toEqual([]);
    expect(asArray(post.media).length).toBe(0);
  });

  it('refresh/bulk errors null → .length/.join safe', () => {
    const errors = asArray(null as unknown as string[]);
    expect(errors.length).toBe(0);
    expect(errors.join(' | ')).toBe('');
  });
});
