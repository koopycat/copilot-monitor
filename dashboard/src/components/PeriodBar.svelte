<script lang="ts">
  import type { Period } from '../lib/types';

  interface Props {
    periods: Period[];
    active: string;
    onchange: (value: string) => void;
  }
  let { periods, active, onchange }: Props = $props();
</script>

<div class="period-bar">
  {#each periods as p (p.key)}
    <button
      class="period-btn"
      class:active={active === p.key}
      aria-current={active === p.key ? 'page' : undefined}
      onclick={() => onchange(p.key)}
    >
      {p.label}
    </button>
  {/each}
</div>
