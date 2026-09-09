// Pure calendar helpers (no Svelte imports) — unit-tested in calendar.test.ts.
// Weeks start on Monday. Drafts (no scheduled_at) have no effective date and
// live in the unscheduled tray instead of the grid.

export interface DatedPost {
  id: string;
  status: string;
  scheduled_at: string | null;
  created_at: string;
  title?: string;
  content?: string;
}

export function dayKey(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function todayKey(): string {
  return dayKey(new Date());
}

// Monday-first month grid; null = padding cell outside the month.
export function monthGrid(year: number, month: number): (Date | null)[][] {
  const first = new Date(year, month, 1);
  const lead = (first.getDay() + 6) % 7; // Mon=0 … Sun=6
  const days = new Date(year, month + 1, 0).getDate();
  const cells: (Date | null)[] = [];
  for (let i = 0; i < lead; i++) cells.push(null);
  for (let d = 1; d <= days; d++) cells.push(new Date(year, month, d));
  while (cells.length % 7 !== 0) cells.push(null);
  const weeks: (Date | null)[][] = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));
  return weeks;
}

// Where a post lives on the calendar. Drafts (no date by backend convention)
// return null → unscheduled tray. Published posts fall back to created_at so
// legacy rows without a date still show up instead of vanishing.
export function effectiveDate(p: DatedPost): Date | null {
  const raw = p.scheduled_at ?? (p.status === 'draft' ? null : p.created_at ?? null);
  if (!raw) return null;
  const d = new Date(raw);
  return isNaN(d.getTime()) ? null : d;
}

export function groupByDay(posts: DatedPost[]): Map<string, DatedPost[]> {
  const m = new Map<string, DatedPost[]>();
  for (const p of posts) {
    const d = effectiveDate(p);
    if (!d) continue;
    const k = dayKey(d);
    const arr = m.get(k);
    if (arr) arr.push(p);
    else m.set(k, [p]);
  }
  return m;
}

// Only future-editable posts move. Published history is fixed —
// delete or duplicate instead (the API rejects PATCH on them too).
export function isMovable(p: { status: string } | null | undefined): boolean {
  return !!p && (p.status === 'draft' || p.status === 'scheduled');
}

// Dropping onto a day keeps the original time-of-day; dateless drafts land
// at 09:00 local. Returns an ISO string for PATCH /api/posts/:id.
export function dropDateTime(day: Date, post?: DatedPost | null): string {
  const raw = post?.scheduled_at;
  const src = raw ? new Date(raw) : null;
  const ok = src && !isNaN(src.getTime()) ? src : null;
  const at = new Date(
    day.getFullYear(), day.getMonth(), day.getDate(),
    ok ? ok.getHours() : 9,
    ok ? ok.getMinutes() : 0,
  );
  return at.toISOString();
}

export function monthLabel(year: number, month: number): string {
  return new Date(year, month, 1).toLocaleString(undefined, { month: 'long', year: 'numeric' });
}

// Dot color per post status (calendar day cells).
export function dotClass(s: string): string {
  if (s === 'published') return 'bg-green-500';
  if (s === 'failed') return 'bg-red-500';
  if (s === 'draft') return 'bg-zinc-400';
  if (s === 'external') return 'bg-zinc-300';
  return 'bg-amber-500';
}

// Map analytics "outside" rows to plottable calendar items (read-only:
// status 'external' is never movable, tooltips show the native snippet).
export interface ExternalRow {
  target_id: string;
  published_at: string | null;
  text?: string;
}

export function externalToDated(rows: ExternalRow[] | null | undefined): DatedPost[] {
  const list = Array.isArray(rows) ? rows : [];
  const out: DatedPost[] = [];
  for (const r of list) {
    if (!r?.published_at) continue;
    out.push({
      id: 'ext:' + r.target_id,
      status: 'external',
      scheduled_at: r.published_at,
      created_at: r.published_at,
      content: r.text ?? '',
    });
  }
  return out;
}
