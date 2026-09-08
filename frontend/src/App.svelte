<script lang="ts">
  import { onMount } from 'svelte';
  import { api, NETWORKS, type Account, type Post, type Network } from './lib/api';
  import Button from './lib/components/ui/Button.svelte';
  import Card from './lib/components/ui/Card.svelte';
  import Input from './lib/components/ui/Input.svelte';
  import Textarea from './lib/components/ui/Textarea.svelte';
  import Badge from './lib/components/ui/Badge.svelte';
  import { asArray, asRecord } from './lib/normalize';

  let tab: 'compose' | 'scheduled' | 'analytics' | 'evergreen' | 'accounts' = 'compose';
  let accounts: Account[] = [];
  let posts: Post[] = [];
  let limits: Record<string, any> = {};
  let error = '';

  // --- compose state ---
  let title = '';
  let content = '';
  let link = '';
  let scheduledAt = '';
  let selected: Record<string, boolean> = {};
  let customText: Record<string, string> = {};
  let firstComment: Record<string, string> = {};
  let showCustom: Record<string, boolean> = {};
  let mediaIds: string[] = [];
  let mediaTypes: string[] = [];
  let preview: any = null;
  let busy = false;

  // --- ai + smart schedule ---
  let variants: { tone: string; text: string; hashtags: string }[] = [];
  let variantSource = '';
  let suggestions: { at: string; score: number; reason: string }[] = [];

  // --- accounts state ---
  let newNet: Network = 'twitter';
  let newName = '';
  let newToken = '';
  let newExtra = '';
  let expiringIds: Set<string> = new Set();

  // --- analytics ---
  let totals: any = null;
  let arows: any[] = [];

  // --- evergreen + bulk ---
  let rules: any[] = [];
  let ruleName = '';
  let ruleHours = 24;
  let rulePool: Record<string, boolean> = {};
  let ruleAccts: Record<string, boolean> = {};
  let bulkResult: any = null;

  async function refresh() {
    try {
      [accounts, posts, limits] = await Promise.all([api.accounts(), api.posts(), api.limits()]);
      accounts = asArray(accounts);
      posts = asArray(posts);
      limits = asRecord(limits);
      const exp = asArray(await api.expiring().catch((): Account[] => []));
      expiringIds = new Set(exp.map((a) => a.id));
    } catch (e: any) { error = e.message; }
  }
  onMount(refresh);

  $: selectedIds = Object.keys(selected).filter((k) => selected[k]);
  $: selectedNets = [...new Set(accounts.filter((a) => selected[a.id]).map((a) => a.network))];
  $: charCount = [...content].length;

  function limitFor(net: Network) { return limits[net]?.max_chars ?? 0; }

  async function onUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;
    busy = true;
    try {
      const assets = asArray(await api.upload(input.files));
      mediaIds = [...mediaIds, ...assets.map((a) => a.id)];
      mediaTypes = [...mediaTypes, ...assets.map((a) => a.media_type)];
    } catch (e: any) { error = e.message; }
    busy = false;
  }

  async function onPreview() {
    preview = await api.preview({
      title, content, link, media_types: mediaTypes,
      targets: selectedIds.map((id) => ({ account_id: id, custom_text: customText[id] ?? '', first_comment: firstComment[id] ?? '' })),
    });
  }

  async function onVariants() {
    error = '';
    try {
      const net = selectedNets[0] ?? 'twitter';
      const r = await api.captions(content || title, net);
      variants = asArray(r.variants);
      variantSource = r.source;
    } catch (e: any) { error = e.message; }
  }

  async function onSuggest() {
    error = '';
    try {
      const r = await api.suggest(selectedNets.length ? selectedNets : ['twitter'], 3);
      suggestions = asArray(r.suggestions);
      if (suggestions.length) {
        const d = new Date(suggestions[0].at);
        const p = (n: number) => String(n).padStart(2, '0');
        scheduledAt = `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
      }
    } catch (e: any) { error = e.message; }
  }

  async function submit(publishNow: boolean, autoSchedule = false) {
    if (!selectedIds.length) { error = 'Select at least one account.'; return; }
    busy = true; error = '';
    try {
      await api.createPost({
        title, content, link, media_ids: mediaIds,
        publish_now: publishNow, auto_schedule: autoSchedule,
        scheduled_at: publishNow ? null : scheduledAt ? new Date(scheduledAt).toISOString() : null,
        targets: selectedIds.map((id) => ({ account_id: id, custom_text: customText[id] ?? '', first_comment: firstComment[id] ?? '' })),
      });
      title = content = link = scheduledAt = ''; selected = {}; customText = {}; preview = null; mediaIds = []; mediaTypes = []; variants = []; suggestions = [];
      await refresh();
      tab = 'scheduled';
    } catch (e: any) { error = e.message; }
    busy = false;
  }

  async function addAccount() {
    if (!newName) { error = 'Give the account a name.'; return; }
    await api.addAccount({ network: newNet, name: newName, access_token: newToken || `demo-${newNet}`, extra: newExtra });
    newName = newToken = newExtra = '';
    await refresh();
  }

  async function connect(network: Network) {
    const { auth_url } = await api.authUrl(network);
    window.open(auth_url, '_blank', 'width=600,height=700');
  }

  async function loadAnalytics() {
    const r = await api.analytics();
    totals = r.totals; arows = asArray(r.rows);
  }

  async function loadEvergreen() {
    rules = asArray(await api.evergreen());
  }

  async function addRule() {
    const pool = Object.keys(rulePool).filter((k) => rulePool[k]);
    const accts = Object.keys(ruleAccts).filter((k) => ruleAccts[k]);
    if (!ruleName || !pool.length || !accts.length) { error = 'Rule needs a name, ≥1 post and ≥1 account.'; return; }
    await api.addEvergreen({ name: ruleName, pool_post_ids: pool, account_ids: accts, interval_hours: ruleHours });
    ruleName = ''; rulePool = {}; ruleAccts = {};
    await loadEvergreen();
  }

  async function onBulk(e: Event, auto: boolean) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;
    bulkResult = await api.bulkImport(input.files[0], auto);
    await refresh();
  }

  function netLabel(id: string) { return NETWORKS.find((n) => n.id === id)?.label ?? id; }
  function tone(s: string) { return s === 'published' ? 'green' : s === 'failed' ? 'red' : s === 'partial' || s === 'scheduled' ? 'amber' : 'default'; }
  function switchTab(t: typeof tab) {
    tab = t;
    if (t === 'analytics') loadAnalytics();
    if (t === 'evergreen') { loadEvergreen(); refresh(); }
    if (t === 'accounts') refresh();
  }
</script>

<div class="mx-auto max-w-6xl p-4 md:p-6">
  <header class="mb-6 flex flex-wrap items-center justify-between gap-3">
    <div>
      <h1 class="text-2xl font-bold">Solomon <span class="text-sm font-normal text-muted-foreground">social scheduler</span></h1>
      <p class="text-sm text-muted-foreground">X · Facebook · Instagram · YouTube · TikTok · LinkedIn · Pinterest — Fiber + GORM + SQLite</p>
    </div>
    <nav class="flex flex-wrap gap-2">
      <Button variant={tab === 'compose' ? 'default' : 'outline'} on:click={() => switchTab('compose')}>Compose</Button>
      <Button variant={tab === 'scheduled' ? 'default' : 'outline'} on:click={() => switchTab('scheduled')}>Queue ({posts.length})</Button>
      <Button variant={tab === 'analytics' ? 'default' : 'outline'} on:click={() => switchTab('analytics')}>Analytics</Button>
      <Button variant={tab === 'evergreen' ? 'default' : 'outline'} on:click={() => switchTab('evergreen')}>Evergreen</Button>
      <Button variant={tab === 'accounts' ? 'default' : 'outline'} on:click={() => switchTab('accounts')}>Accounts ({accounts.length})</Button>
    </nav>
  </header>

  {#if error}<div class="mb-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>{/if}

  {#if tab === 'compose'}
    <div class="grid gap-4 md:grid-cols-[1fr_340px]">
      <Card><div class="space-y-3 p-4">
        <h2 class="font-semibold">New post <span class="text-xs font-normal text-muted-foreground">— one master post, published to every selected account</span></h2>
        <Input bind:value={title} placeholder="Title (required for YouTube & Pinterest ≤100 chars)" />
        <Textarea bind:value={content} rows={5} placeholder="What's happening? Master text — per-account tweaks below." />
        <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>{charCount} chars · X splits &gt;280 into a thread automatically</span>
          <button class="underline" on:click={onVariants} disabled={!content && !title}>✨ Variants{variantSource ? ` (${variantSource})` : ''}</button>
        </div>
        {#if variants.length}
          <div class="space-y-2 rounded-md bg-secondary/60 p-2">
            {#each variants as v}
              <div class="rounded-md border border-border bg-background p-2 text-xs">
                <Badge>{v.tone}</Badge>
                <p class="mt-1">{v.text}</p>
                <p class="text-muted-foreground">{v.hashtags}</p>
                <button class="mt-1 underline" on:click={() => { content = v.text + '\n' + v.hashtags; variants = []; }}>Use this</button>
              </div>
            {/each}
          </div>
        {/if}
        <Input bind:value={link} placeholder="Link (https://… — recommended for Pinterest/LinkedIn)" />
        <div>
          <div class="text-sm font-medium">Media — images, multi-pics, video / Shorts</div>
          <input type="file" multiple accept="image/*,video/*" on:change={onUpload}
            class="mt-1 block w-full text-sm file:mr-3 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5" />
          {#if mediaIds.length}<div class="mt-1 text-xs text-muted-foreground">{mediaIds.length} file(s) attached ({mediaTypes.join(', ')}) — IG/TikTok/YT/Pinterest REQUIRE media; YouTube needs exactly 1 video.</div>{/if}
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <input type="datetime-local" bind:value={scheduledAt} class="rounded-md border border-border px-3 py-1.5 text-sm" />
          <Button variant="secondary" on:click={onSuggest} disabled={!selectedIds.length}>⚡ Best time</Button>
          <Button variant="secondary" on:click={onPreview} disabled={!selectedIds.length}>Check limits</Button>
          <Button on:click={() => submit(false)} disabled={busy || !selectedIds.length}>Schedule</Button>
          <Button variant="outline" on:click={() => submit(false, true)} disabled={busy || !selectedIds.length || !selectedNets.length}>Auto-schedule</Button>
          <Button variant="outline" on:click={() => submit(true)} disabled={busy || !selectedIds.length}>Post now</Button>
        </div>
        {#if suggestions.length}
          <div class="rounded-md bg-secondary/60 p-3 text-xs">
            <div class="mb-1 font-semibold">Suggested slots (analytics-boosted):</div>
            {#each suggestions as s}
              <div>· {new Date(s.at).toLocaleString()} — {s.reason} (score {s.score})</div>
            {/each}
          </div>
        {/if}
        {#if preview}
          <div class="rounded-md bg-secondary/60 p-3 text-xs">
            <div class="mb-1 font-semibold">Limit check (auto-remedies):</div>
            {#each preview.targets ?? [] as t}
              <div class="mb-1">
                <Badge tone={t.plan.ok ? 'green' : 'red'}>{t.network} {t.plan.ok ? 'OK' : 'NEEDS ATTENTION'}</Badge>
                {#each t.plan.adaptations ?? [] as a}<div>· {a.detail}</div>{/each}
                {#each t.plan.errors ?? [] as e}<div class="text-red-700">· {e}</div>{/each}
              </div>
            {/each}
          </div>
        {/if}
      </div></Card>

      <Card><div class="space-y-2 p-4">
        <h2 class="font-semibold">Post to… <span class="text-xs font-normal text-muted-foreground">(multi-select across networks)</span></h2>
        {#if !accounts.length}<p class="text-sm text-muted-foreground">No accounts yet — add them under the Accounts tab. Demo tokens work instantly.</p>{/if}
        {#each accounts as a}
          <div class="rounded-md border border-border p-2">
            <label class="flex cursor-pointer items-center gap-2 text-sm">
              <input type="checkbox" bind:checked={selected[a.id]} class="h-4 w-4" />
              <Badge>{netLabel(a.network)}</Badge><span class="font-medium">{a.name}</span>
            </label>
            {#if selected[a.id]}
              {@const lim = limitFor(a.network)}
              {@const txt = customText[a.id] ?? ''}
              <button class="mt-1 text-xs text-muted-foreground underline" on:click={() => (showCustom[a.id] = !showCustom[a.id])}>
                {showCustom[a.id] ? 'Hide' : 'Customize'} text for {a.name}{#if lim} ({txt ? [...txt].length : charCount}/{lim}){/if}
              </button>
              {#if showCustom[a.id]}
                <Textarea bind:value={customText[a.id]} rows={2} placeholder="Leave empty = use master text" />
                <Input bind:value={firstComment[a.id]} placeholder="First comment (optional — IG hashtags go here)" />
              {/if}
            {/if}
          </div>
        {/each}
        <div class="rounded-md bg-secondary/60 p-2 text-xs text-muted-foreground">
          Remedies: long X post → thread · over-limit LinkedIn/Pinterest → trimmed, rest in first comment ·
          IG/YT/TikTok/Pinterest without media → skipped with a note.
        </div>
      </div></Card>
    </div>
  {/if}

  {#if tab === 'scheduled'}
    <div class="space-y-3">
      {#each posts as p}
        <Card><div class="p-4">
          <div class="flex flex-wrap items-center gap-2">
            <Badge tone={tone(p.status)}>{p.status}</Badge>
            {#if p.title}<span class="font-semibold">{p.title}</span>{/if}
            <span class="text-xs text-muted-foreground">{p.scheduled_at ? new Date(p.scheduled_at).toLocaleString() : 'no schedule'} · {(p.media ?? []).length} media</span>
            <button class="ml-auto text-xs text-red-600 underline" on:click={async () => { await api.deletePost(p.id); await refresh(); }}>delete</button>
          </div>
          <p class="mt-2 whitespace-pre-wrap text-sm">{p.content}</p>
          {#if p.link}<a href={p.link} class="text-xs text-blue-600 underline" target="_blank" rel="noreferrer">{p.link}</a>{/if}
          <div class="mt-2 flex flex-wrap gap-2">
            {#each p.targets ?? [] as t}
              <Badge tone={t.status === 'published' ? 'green' : t.status === 'failed' ? 'red' : 'amber'}>
                {netLabel(t.account?.network)} {t.account?.name}: {t.status}{t.error ? ` — ${t.error.slice(0, 80)}` : ''}
              </Badge>
            {/each}
          </div>
        </div></Card>
      {:else}
        <p class="text-sm text-muted-foreground">Nothing scheduled yet. Compose your first post!</p>
      {/each}
    </div>
  {/if}

  {#if tab === 'analytics'}
    <Card><div class="space-y-3 p-4">
      <div class="flex items-center gap-2">
        <h2 class="font-semibold">Analytics</h2>
        <Button variant="secondary" on:click={async () => { const r = await api.refreshAnalytics(); if (asArray(r.errors).length) error = asArray(r.errors).join(' | '); await loadAnalytics(); }}>↻ Refresh stats</Button>
        {#if totals}<span class="text-xs text-muted-foreground">{totals.posts} published targets</span>{/if}
      </div>
      {#if totals}
        <div class="grid grid-cols-2 gap-2 md:grid-cols-4">
          <div class="rounded-md bg-secondary p-3 text-center"><div class="text-xl font-bold">{totals.views}</div><div class="text-xs text-muted-foreground">views</div></div>
          <div class="rounded-md bg-secondary p-3 text-center"><div class="text-xl font-bold">{totals.likes}</div><div class="text-xs text-muted-foreground">likes</div></div>
          <div class="rounded-md bg-secondary p-3 text-center"><div class="text-xl font-bold">{totals.comments}</div><div class="text-xs text-muted-foreground">comments</div></div>
          <div class="rounded-md bg-secondary p-3 text-center"><div class="text-xl font-bold">{totals.shares}</div><div class="text-xs text-muted-foreground">shares</div></div>
        </div>
      {/if}
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead><tr class="text-xs text-muted-foreground"><th class="py-1">Network</th><th>Account</th><th>Views</th><th>Likes</th><th>Comments</th><th>Shares</th><th></th></tr></thead>
          <tbody>
            {#each arows as r}
              <tr class="border-t border-border">
                <td class="py-1"><Badge>{r.network}</Badge></td>
                <td>{r.account_name}</td>
                <td>{r.views}</td><td>{r.likes}</td><td>{r.comments}</td><td>{r.shares}</td>
                <td>{#if r.is_demo}<Badge tone="amber">demo</Badge>{/if}</td>
              </tr>
            {:else}
              <tr><td colspan="7" class="py-2 text-muted-foreground">No published posts yet — stats appear after publishing + refresh.</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
      <p class="text-xs text-muted-foreground">Demo tokens show deterministic pseudo-stats. Real-token live fetch is wired for Instagram insights; other networks document the exact scope/endpoint in the README.</p>
    </div></Card>
  {/if}

  {#if tab === 'evergreen'}
    <div class="grid gap-4 md:grid-cols-2">
      <Card><div class="space-y-3 p-4">
        <h2 class="font-semibold">Evergreen rules <span class="text-xs font-normal text-muted-foreground">— recycle top posts forever</span></h2>
        {#each rules as r}
          <div class="flex items-center gap-2 rounded-md border border-border p-2 text-sm">
            <span class="font-medium">{r.name}</span>
            <span class="text-xs text-muted-foreground">every {r.interval_hours}h · next {r.next_run_at ? new Date(r.next_run_at).toLocaleString() : '—'}</span>
            <button class="ml-auto text-xs text-red-600 underline" on:click={async () => { await api.deleteEvergreen(r.id); await loadEvergreen(); }}>remove</button>
          </div>
        {:else}
          <p class="text-sm text-muted-foreground">No rules yet.</p>
        {/each}
        <div class="border-t border-border pt-3">
          <div class="mb-2 text-sm font-medium">New rule</div>
          <Input bind:value={ruleName} placeholder="Rule name (e.g. weekly tips)" />
          <div class="flex items-center gap-2 text-sm">
            <span>Every</span>
            <input type="number" min="1" bind:value={ruleHours} class="w-20 rounded-md border border-border px-2 py-1 text-sm" />
            <span>hours</span>
          </div>
          <div class="text-xs font-medium text-muted-foreground">Pool (posts to recycle):</div>
          <div class="max-h-32 space-y-1 overflow-y-auto">
            {#each posts as p}
              <label class="flex items-center gap-2 text-xs"><input type="checkbox" bind:checked={rulePool[p.id]} />{p.title || p.content.slice(0, 50)}</label>
            {/each}
          </div>
          <div class="text-xs font-medium text-muted-foreground">Republish to:</div>
          <div class="space-y-1">
            {#each accounts as a}
              <label class="flex items-center gap-2 text-xs"><input type="checkbox" bind:checked={ruleAccts[a.id]} /><Badge>{netLabel(a.network)}</Badge>{a.name}</label>
            {/each}
          </div>
          <Button on:click={addRule}>Save rule</Button>
        </div>
      </div></Card>
      <Card><div class="space-y-3 p-4">
        <h2 class="font-semibold">Bulk import <span class="text-xs font-normal text-muted-foreground">— CSV: title,content,link,scheduled_at,accounts</span></h2>
        <label class="block text-sm">Standard import
          <input type="file" accept=".csv" on:change={(e) => onBulk(e, false)} class="mt-1 block w-full text-sm file:mr-3 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5" />
        </label>
        <label class="block text-sm">Import + auto-schedule (3h apart)
          <input type="file" accept=".csv" on:change={(e) => onBulk(e, true)} class="mt-1 block w-full text-sm file:mr-3 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5" />
        </label>
        {#if bulkResult}
          <div class="rounded-md bg-secondary/60 p-2 text-xs">
            Created {bulkResult.created}, skipped {bulkResult.skipped}.
            {#each bulkResult.errors ?? [] as e}<div class="text-red-700">{e}</div>{/each}
          </div>
        {/if}
        <pre class="overflow-x-auto rounded-md bg-secondary/60 p-2 text-[11px]">title,content,link,scheduled_at,accounts
"Tip 1","Post about …","https://…","2026-09-10 09:00","twitter;@acme"</pre>
      </div></Card>
    </div>
  {/if}

  {#if tab === 'accounts'}
    <div class="grid gap-4 md:grid-cols-2">
      <Card><div class="space-y-3 p-4">
        <div class="flex items-center gap-2">
          <h2 class="font-semibold">Connected accounts</h2>
          <Button variant="secondary" on:click={async () => { const r = await api.refreshTokens(); if (asArray(r.errors).length) error = asArray(r.errors).join(' | '); await refresh(); }}>↻ Refresh tokens</Button>
        </div>
        {#each accounts as a}
          {@const exp = a.expires_at ? new Date(a.expires_at) : null}
          {@const bad = expiringIds.has(a.id)}
          <div class="flex items-center gap-2 rounded-md border border-border p-2 text-sm">
            <Badge>{netLabel(a.network)}</Badge><span class="font-medium">{a.name}</span>
            {#if !exp}<Badge tone="amber">no expiry</Badge>
            {:else if exp < new Date()}<Badge tone="red">expired</Badge>
            {:else if bad}<Badge tone="amber">expires {exp.toLocaleDateString()}</Badge>{/if}
            <button class="ml-auto text-xs text-red-600 underline"
              on:click={async () => { await api.deleteAccount(a.id); await refresh(); }}>remove</button>
          </div>
        {:else}
          <p class="text-sm text-muted-foreground">None yet — add your first below (demo token works without API keys).</p>
        {/each}
        <p class="text-xs text-muted-foreground">Tokens expiring within 7 days are auto-refreshed hourly by the server (demo tokens just extend; real tokens use each network's OAuth refresh grant).</p>
      </div></Card>
      <Card><div class="space-y-3 p-4">
        <h2 class="font-semibold">Add account</h2>
        <div class="flex flex-wrap gap-1">
          {#each NETWORKS as n}
            <button on:click={() => (newNet = n.id)}
              class={`rounded-full px-3 py-1 text-xs font-medium ${newNet === n.id ? n.color : 'bg-secondary text-secondary-foreground'}`}>{n.label}</button>
          {/each}
        </div>
        <Input bind:value={newName} placeholder="@handle / Page / Channel name" />
        <Input bind:value={newToken} placeholder="Access token (empty = demo/mock mode)" />
        <Input bind:value={newExtra}
          placeholder='Extra: page_id=… / ig_user_id=… / board_id=… / author_urn=… (JSON or k=v)' />
        <div class="flex gap-2">
          <Button on:click={addAccount}>Save account</Button>
          <Button variant="outline" on:click={() => connect(newNet)}>Get OAuth URL</Button>
        </div>
        <p class="text-xs text-muted-foreground">
          Real keys: copy <code>backend/.env.example</code> → <code>.env</code>, fill client IDs, open the OAuth URL,
          exchange the code per README, paste the token above. Extra holds per-network IDs
          (FB page_id, IG ig_user_id, Pinterest board_id, LinkedIn author_urn).
        </p>
      </div></Card>
    </div>
    <Card><div class="mt-4 p-4 text-xs text-muted-foreground">
      <span class="font-semibold text-foreground">Limits cheat-sheet:</span>
      {#each Object.entries(limits ?? {}) as [net, l]}
        <div><Badge>{net}</Badge> {l.max_chars} chars · {l.max_images} imgs · {l.max_videos} video(s) — {l.notes}</div>
      {/each}
    </div></Card>
  {/if}
</div>
