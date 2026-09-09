<script lang="ts">
  import SocialIcon from './SocialIcon.svelte';
  import { hue, initials } from '../../social';

  // Account photo with initials fallback + network logo badge.
  // Broken/stale URLs degrade to initials via on:error; demo/empty URLs
  // never try to load at all.
  export let name = '';
  export let avatarUrl: string | null = null;
  export let network = '';
  export let size = 32;

  let broken = false;
  $: avatarUrl, (broken = false);
  $: showImg = !!avatarUrl && !broken;
  $: fg = `hsl(${hue(name)}, 45%, 32%)`;
  $: bg = `hsl(${hue(name)}, 60%, 90%)`;
  $: fs = Math.max(9, Math.round(size * 0.36));
  $: badge = Math.max(12, Math.round(size * 0.45));
</script>

<span class="relative inline-block align-middle" style={`width: ${size}px; height: ${size}px; flex-shrink: 0;`}>
  {#if showImg}
    <img
      src={avatarUrl}
      alt={name}
      width={size}
      height={size}
      loading="lazy"
      on:error={() => (broken = true)}
      class="h-full w-full rounded-full border border-border object-cover"
    />
  {:else}
    <span
      class="flex h-full w-full items-center justify-center rounded-full border border-border font-semibold"
      style={`background: ${bg}; color: ${fg}; font-size: ${fs}px;`}
    >
      {initials(name)}
    </span>
  {/if}
  {#if network}
    <span class="absolute -bottom-0.5 -right-0.5 rounded-full bg-white p-[1px] shadow-sm">
      <SocialIcon {network} size={badge} />
    </span>
  {/if}
</span>
