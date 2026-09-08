<script lang="ts">
  export let value: string = '';
  export let placeholder = '';
  export let rows = 4;
</script>

<textarea {placeholder} {rows} bind:value on:input
  class="flex w-full rounded-md border border-border bg-background px-3 py-2 text-sm shadow-sm outline-none focus:ring-2 focus:ring-primary/30"></textarea>
