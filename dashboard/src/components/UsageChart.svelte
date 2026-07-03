<script lang="ts">
  import { onMount } from 'svelte';
  import { dashboard } from '../stores/dashboard.svelte';

  let canvas: HTMLCanvasElement | null = $state(null);

  onMount(() => {
    dashboard.init(canvas);
    return () => dashboard.destroy();
  });
</script>

<div class="chart-wrap"><canvas id="chart" aria-label="Usage chart" bind:this={canvas}></canvas></div>
<div class="legend" id="chart-legend"></div>
