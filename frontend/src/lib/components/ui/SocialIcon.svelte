<script lang="ts">
  import { socialGlyph } from '../../social';

  // Brand glyph for one network. Default color = brand; pass monochrome
  // (any CSS color) for quiet spots.
  export let network: string;
  export let size = 16;
  export let monochrome: string | null = null;

  $: glyph = socialGlyph(network);
  $: color = monochrome ?? glyph.brand;
</script>

<svg
  width={size}
  height={size}
  viewBox={glyph.viewBox}
  role="img"
  aria-label={glyph.label}
  style={`color: ${color}; flex-shrink: 0; display: inline-block; vertical-align: -2px;`}
>{@html glyph.body}</svg>
