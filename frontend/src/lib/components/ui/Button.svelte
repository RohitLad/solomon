<script lang="ts">
  export let variant: 'default' | 'secondary' | 'outline' | 'destructive' = 'default';
  export let type: 'button' | 'submit' = 'button';
  export let disabled = false;
  const styles: Record<string, string> = {
    default: 'bg-primary text-primary-foreground hover:opacity-90',
    secondary: 'bg-secondary text-secondary-foreground hover:opacity-80',
    outline: 'border border-border bg-background hover:bg-secondary',
    destructive: 'bg-destructive text-destructive-foreground hover:opacity-90',
  };
</script>

<button {type} {disabled} on:click
  class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition disabled:opacity-50 ${styles[variant]}`}>
  <slot />
</button>
