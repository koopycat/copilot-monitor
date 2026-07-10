<script lang="ts">
  import { usd, intl } from '../lib/format';
  import { dashboard } from '../stores/dashboard.svelte';

  const cost = $derived(usd(dashboard.current.session?.cost ?? 0));
  const requests = $derived(intl(dashboard.current.session?.request_count ?? 0));
  const tokens = $derived(intl(dashboard.current.session?.token_count ?? 0));
  const compressionRemoved = $derived(
    dashboard.current.models.reduce((sum, m) => sum + (m.compression_removed_tokens ?? 0), 0),
  );
</script>

<div
  class="live-session"
  class:active={dashboard.liveSessionActive}
  class:pulse={dashboard.sessionPulse}
>
  <div class="live-session-head">
    <div class="live-session-title">Live Session</div>
    <div class="live-session-status">
      <span class="status-dot" class:active={dashboard.liveSessionActive}></span>
      {dashboard.sessionStatusText}
    </div>
  </div>
  <div class="live-session-cost">{cost}</div>
  <div class="live-session-grid">
    <div class="live-session-item"><span>Requests</span><strong>{requests}</strong></div>
    <div class="live-session-item"><span>Tokens</span><strong>{tokens}</strong></div>
    <div class="live-session-item">
      <span>Duration</span><strong>{dashboard.sessionDurationText}</strong>
    </div>
  </div>
  <div class="live-session-models">Models: {dashboard.sessionModelsText}</div>
  {#if compressionRemoved > 0}
    <div class="live-session-compression">Compression saved {intl(compressionRemoved)} tokens</div>
  {/if}
</div>
