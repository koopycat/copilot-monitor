<script lang="ts">
  interface Props {
    value: string;
    label: string;
  }
  let { value, label }: Props = $props();
</script>

<div class="metric">
  <div class="metric-value">{value}</div>
  <div class="metric-label">{label}</div>
</div>
