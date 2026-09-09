<script lang="ts">
  export let cls = '';
</script>

<div class={`rounded-lg border border-border bg-card text-card-foreground shadow-sm ${cls}`}>
  <slot />
</div>
