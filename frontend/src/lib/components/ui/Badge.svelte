<script lang="ts">
  export let tone: 'default' | 'green' | 'red' | 'amber' = 'default';
  const map = {
    default: 'bg-secondary text-secondary-foreground',
    green: 'bg-green-100 text-green-800',
    red: 'bg-red-100 text-red-800',
    amber: 'bg-amber-100 text-amber-800',
  };
</script>

<span class={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${map[tone]}`}>
  <slot />
</span>
