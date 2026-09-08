// Tiny API client for the Go backend (same origin in prod, :8080 in dev via proxy).
const BASE = '';

export type Network =
  | 'twitter' | 'facebook' | 'instagram' | 'youtube' | 'tiktok' | 'linkedin' | 'pinterest';

export interface Account { id: string; network: Network; name: string; external_id: string; extra: string; is_active: boolean; expires_at?: string | null; }
export interface MediaAsset { id: string; post_id: string; file_path: string; media_type: 'image' | 'video'; }
export interface PostTarget {
  id: string; account_id: string; account: Account;
  custom_text: string; first_comment: string; status: string;
  network_post_id: string; error: string;
}
export interface Post {
  id: string; title: string; content: string; link: string;
  scheduled_at: string | null; status: string;
  targets: PostTarget[]; media: MediaAsset[]; created_at: string;
}

async function req<T>(path: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(BASE + path, opts);
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<T>;
}

export const api = {
  health: () => req<{ ok: boolean }>('/api/health'),
  limits: () => req<Record<Network, any>>('/api/limits'),
  accounts: () => req<Account[]>('/api/accounts'),
  addAccount: (body: object) =>
    req<Account>('/api/accounts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  deleteAccount: (id: string) => req<{ ok: boolean }>(`/api/accounts/${id}`, { method: 'DELETE' }),
  authUrl: (n: Network) => req<{ auth_url: string }>(`/api/accounts/${n}/auth-url`),
  posts: (status = '') => req<Post[]>(`/api/posts${status ? `?status=${status}` : ''}`),
  createPost: (body: object) =>
    req<Post>('/api/posts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  preview: (body: object) =>
    req<any>('/api/posts/preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  deletePost: (id: string) => req<{ ok: boolean }>(`/api/posts/${id}`, { method: 'DELETE' }),
  upload: async (files: FileList): Promise<MediaAsset[]> => {
    const fd = new FormData();
    for (const f of files) fd.append('files', f);
    const r = await fetch('/api/upload', { method: 'POST', body: fd });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  captions: (text: string, network: Network) =>
    req<{ variants: { tone: string; text: string; hashtags: string }[]; source: string }>(
      '/api/ai/captions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text, network }) }),
  suggest: (networks: string[], count = 3) =>
    req<{ suggestions: { at: string; score: number; networks: string[]; reason: string }[] }>(
      `/api/schedule/suggest?networks=${networks.join(',')}&count=${count}`),
  analytics: () => req<{ totals: any; rows: any[] }>('/api/analytics'),
  refreshAnalytics: () => req<{ refreshed: number; errors: string[] }>('/api/analytics/refresh', { method: 'POST' }),
  expiring: () => req<Account[]>('/api/accounts/expiring'),
  refreshTokens: () => req<{ refreshed: number; errors: string[] }>('/api/accounts/refresh', { method: 'POST' }),
  bulkImport: async (file: File, auto = false) => {
    const fd = new FormData();
    fd.append('file', file);
    const r = await fetch(`/api/posts/bulk${auto ? '?auto_schedule=1' : ''}`, { method: 'POST', body: fd });
    if (!r.ok) throw new Error(await r.text());
    return r.json() as Promise<{ created: number; skipped: number; errors: string[] }>;
  },
  evergreen: () => req<any[]>('/api/evergreen'),
  addEvergreen: (body: object) =>
    req<any>('/api/evergreen', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  deleteEvergreen: (id: string) => req<{ ok: boolean }>(`/api/evergreen/${id}`, { method: 'DELETE' }),
};

export const NETWORKS: { id: Network; label: string; color: string }[] = [
  { id: 'twitter', label: 'X (Twitter)', color: 'bg-black text-white' },
  { id: 'facebook', label: 'Facebook', color: 'bg-blue-600 text-white' },
  { id: 'instagram', label: 'Instagram', color: 'bg-pink-600 text-white' },
  { id: 'youtube', label: 'YouTube', color: 'bg-red-600 text-white' },
  { id: 'tiktok', label: 'TikTok', color: 'bg-zinc-900 text-white' },
  { id: 'linkedin', label: 'LinkedIn', color: 'bg-sky-700 text-white' },
  { id: 'pinterest', label: 'Pinterest', color: 'bg-red-700 text-white' },
];
