<script lang="ts">
  import Badge from './Badge.svelte';
  import SocialIcon from './SocialIcon.svelte';
  import { socialGlyph } from '../../social';

  // Network pill: brand glyph + label in one Badge.
  export let network: string;
  export let label: string | null = null;
  export let iconSize = 12;

  $: text = label ?? socialGlyph(network).label;
</script>

<Badge
  ><span class="inline-flex items-center gap-1"><SocialIcon {network} size={iconSize} /><span>{text}</span></span
  ></Badge
>
