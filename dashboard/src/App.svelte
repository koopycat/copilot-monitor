<script lang="ts">
  import AnomalyFeed from './components/AnomalyFeed.svelte';
  import LiveSessionCard from './components/LiveSessionCard.svelte';
  import MetricCard from './components/MetricCard.svelte';
  import ModelsTable from './components/ModelsTable.svelte';
  import PeriodBar from './components/PeriodBar.svelte';
  import PolicyPanel from './components/PolicyPanel.svelte';
  import RoutesPanel from './components/RoutesPanel.svelte';
  import SessionsTable from './components/SessionsTable.svelte';
  import ToggleGroup from './components/ToggleGroup.svelte';
  import UpstreamFilter from './components/UpstreamFilter.svelte';
  import UsageChart from './components/UsageChart.svelte';
  import { usd } from './lib/format';
  import { dashboard } from './stores/dashboard.svelte';

  const costText = $derived(dashboard.cost !== null ? usd(dashboard.cost) : '-');
</script>

<h1>
  Copilot Monitor
  <button
    class="refresh-btn"
    class:loading={dashboard.loading}
    aria-label="Refresh now"
    aria-busy={dashboard.loading}
    disabled={dashboard.loading}
    onclick={() => dashboard.load()}
    title="Refresh now"
  >↻</button>
</h1>

<PeriodBar
  periods={dashboard.periods}
  active={dashboard.period}
  onchange={(v) => dashboard.switchPeriod(v as typeof dashboard.period)}
/>

<p class="subtitle">
  {#if dashboard.error}
    <span class="subtitle-error">Error &mdash; {dashboard.error}</span>
  {:else if dashboard.lastUpdated}
    <span class:updated-flash={dashboard.updatedFlash}>Updated {dashboard.lastUpdated}</span>
  {:else}
    &nbsp;
  {/if}
  <UpstreamFilter />
</p>

{#if dashboard.loading && dashboard.stats.length === 0 && !dashboard.error}
  <div class="loading">
    <span class="loading-dot"></span>
    <span class="loading-dot"></span>
    <span class="loading-dot"></span>
  </div>
{:else if dashboard.error && dashboard.stats.length === 0}
  <p class="error-state">{dashboard.error}</p>
{:else if dashboard.periodIsEmpty && !dashboard.error}
  <p class="empty-state">
    No activity captured for this period. Start making LLM requests through the proxy and data will
    appear here.
  </p>
{/if}

<div class="row">
  <MetricCard value={costText} label={`est. AI-credit cost, ${dashboard.periodLabel}`} />
  <MetricCard value={dashboard.projectedText} label={dashboard.projectedLabel} />
  <MetricCard
    value={dashboard.totalRequests.toLocaleString()}
    label={`requests, ${dashboard.periodLabel}`}
  />
  <LiveSessionCard />
</div>

<div class="usage-head">
  <h2>Usage</h2>
  <div class="usage-toggles">
    <ToggleGroup
      options={[
        { value: 'day', label: 'Day' },
        { value: 'hour', label: 'Hour' },
      ]}
      active={dashboard.gran}
      onchange={(v) => dashboard.switchGran(v as typeof dashboard.gran)}
    />
    <ToggleGroup
      options={[
        { value: 'tokens', label: 'Tokens' },
        { value: 'requests', label: 'Requests' },
      ]}
      active={dashboard.metric}
      onchange={(v) => dashboard.switchMetric(v as typeof dashboard.metric)}
    />
  </div>
</div>

<UsageChart />

<details class="table-section models-section" open>
  <summary>
    <span class="table-section-title">Models</span>
    <span class="table-section-state" aria-hidden="true"></span>
  </summary>
  <div class="table-section-content">
    <ModelsTable />
  </div>
</details>

<AnomalyFeed />

<details class="table-section sessions-section" open>
  <summary>
    <span class="table-section-title">Recent Sessions</span>
    <span class="table-section-state" aria-hidden="true"></span>
  </summary>
  <div class="table-section-content">
    <SessionsTable />
  </div>
</details>

<RoutesPanel />
<PolicyPanel />

<footer class="footer">
  <span>Estimated AI-credit list-price cost. Not your GitHub Copilot bill.</span>
  <span><a href={dashboard.exportHref}>Export CSV</a> · Auto-refreshes every 30s</span>
</footer>
