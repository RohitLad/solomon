<script lang="ts">
  export let value: string = '';
  export let placeholder = '';
  export let type = 'text';
</script>

<input {type} {placeholder} bind:value on:input
  class="flex h-9 w-full rounded-md border border-border bg-background px-3 py-1 text-sm shadow-sm outline-none focus:ring-2 focus:ring-primary/30" />
