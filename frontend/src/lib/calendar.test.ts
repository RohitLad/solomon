import { describe, expect, it } from 'vitest';
import {
  dayKey,
  dotClass,
  dropDateTime,
  effectiveDate,
  externalToDated,
  groupByDay,
  isMovable,
  monthGrid,
  type DatedPost,
} from './calendar';

const mk = (over: Partial<DatedPost> = {}): DatedPost => ({
  id: 'p1',
  status: 'scheduled',
  scheduled_at: '2030-07-04T15:30:00.000Z',
  created_at: '2030-06-01T00:00:00.000Z',
  ...over,
});

describe('monthGrid', () => {
  it('builds Monday-first weeks of 7 with padding', () => {
    // July 2030: Jul 1 is a Monday -> no lead padding, 31 days -> 5 rows
    const weeks = monthGrid(2030, 6);
    expect(weeks).toHaveLength(5);
    for (const w of weeks) expect(w).toHaveLength(7);
    expect(weeks[0][0]?.getDate()).toBe(1);
    expect(weeks[0][0]?.getDay()).toBe(1); // Monday
    expect(weeks[4][2]?.getDate()).toBe(31);
    expect(weeks[4][3]).toBeNull(); // trailing padding
  });

  it('pads leading days when the month starts mid-week', () => {
    // June 2030: Jun 1 is a Saturday -> 5 lead cells (Mon-Fri)
    const weeks = monthGrid(2030, 5);
    expect(weeks[0].slice(0, 5)).toEqual([null, null, null, null, null]);
    expect(weeks[0][5]?.getDate()).toBe(1);
    expect(weeks[0][6]?.getDate()).toBe(2); // Sunday
  });

  it('covers February in leap and non-leap years', () => {
    expect(monthGrid(2031, 1).flat().filter(Boolean)).toHaveLength(28);
    expect(monthGrid(2032, 1).flat().filter(Boolean)).toHaveLength(29);
  });
});

describe('dayKey', () => {
  it('formats local YYYY-MM-DD', () => {
    expect(dayKey(new Date(2030, 6, 4))).toBe('2030-07-04');
    expect(dayKey(new Date(2030, 0, 9))).toBe('2030-01-09');
  });
});

describe('effectiveDate', () => {
  it('prefers scheduled_at', () => {
    expect(effectiveDate(mk())?.toISOString()).toBe('2030-07-04T15:30:00.000Z');
  });

  it('sends drafts to the tray (null)', () => {
    expect(effectiveDate(mk({ status: 'draft', scheduled_at: null }))).toBeNull();
  });

  it('falls back to created_at for dated statuses without a date', () => {
    const d = effectiveDate(mk({ status: 'published', scheduled_at: null }));
    expect(d?.toISOString()).toBe('2030-06-01T00:00:00.000Z');
  });

  it('returns null for garbage dates', () => {
    expect(effectiveDate(mk({ scheduled_at: 'not-a-date' }))).toBeNull();
  });
});

describe('groupByDay', () => {
  it('groups by local day and skips dateless drafts', () => {
    const posts = [
      mk({ id: 'a', scheduled_at: '2030-07-04T10:00:00Z' }),
      mk({ id: 'b', scheduled_at: '2030-07-04T22:00:00Z' }),
      mk({ id: 'c', scheduled_at: '2030-07-05T10:00:00Z' }),
      mk({ id: 'd', status: 'draft', scheduled_at: null }),
    ];
    const m = groupByDay(posts);
    const keys = [...m.keys()];
    // a/b share a UTC day; c is its own; d excluded (may shift by TZ — assert counts instead)
    expect(m.size).toBeLessThanOrEqual(3);
    expect([...m.values()].flat().map((p) => p.id).sort()).toEqual(['a', 'b', 'c']);
    expect(keys.every((k) => /^\d{4}-\d{2}-\d{2}$/.test(k))).toBe(true);
  });
});

describe('isMovable', () => {
  it('allows draft/scheduled, freezes history', () => {
    expect(isMovable({ status: 'draft' })).toBe(true);
    expect(isMovable({ status: 'scheduled' })).toBe(true);
    for (const s of ['published', 'partial', 'failed']) {
      expect(isMovable({ status: s })).toBe(false);
    }
  });
});

describe('dropDateTime', () => {  it('keeps the original time-of-day', () => {
    const iso = dropDateTime(new Date(2030, 7, 1), mk());
    const d = new Date(iso);
    expect(d.getFullYear()).toBe(2030);
    expect(d.getMonth()).toBe(7);
    expect(d.getDate()).toBe(1);
    // same wall-clock time as the source post, in local terms
    const src = new Date('2030-07-04T15:30:00.000Z');
    expect(d.getHours() * 60 + d.getMinutes()).toBe(src.getHours() * 60 + src.getMinutes());
  });

  it('lands dateless drafts at 09:00', () => {
    const iso = dropDateTime(new Date(2030, 7, 1), mk({ status: 'draft', scheduled_at: null }));
    const d = new Date(iso);
    expect(d.getHours()).toBe(9);
    expect(d.getMinutes()).toBe(0);
  });
});

describe('dotClass', () => {
  it('maps every status to a dot color', () => {
    expect(dotClass('published')).toBe('bg-green-500');
    expect(dotClass('failed')).toBe('bg-red-500');
    expect(dotClass('draft')).toBe('bg-zinc-400');
    expect(dotClass('external')).toBe('bg-zinc-300');
    expect(dotClass('scheduled')).toBe('bg-amber-500');
    expect(dotClass('partial')).toBe('bg-amber-500');
    expect(dotClass('anything-else')).toBe('bg-amber-500');
  });
});

describe('externalToDated', () => {
  it('maps outside rows to plottable, non-movable items', () => {
    const rows = [
      { target_id: 'abc', published_at: '2030-07-04T10:00:00.000Z', text: 'native post' },
      { target_id: 'dateless', published_at: null },
    ];
    const out = externalToDated(rows);
    expect(out).toHaveLength(1);
    expect(out[0]).toMatchObject({
      id: 'ext:abc',
      status: 'external',
      scheduled_at: '2030-07-04T10:00:00.000Z',
      content: 'native post',
    });
    expect(isMovable(out[0])).toBe(false);
    // plottable by groupByDay
    expect(groupByDay(out).size).toBe(1);
  });

  it('tolerates null/undefined/garbage input', () => {
    expect(externalToDated(null)).toEqual([]);
    expect(externalToDated(undefined)).toEqual([]);
    expect(externalToDated('nope' as unknown as null)).toEqual([]);
    expect(externalToDated([])).toEqual([]);
  });
});
