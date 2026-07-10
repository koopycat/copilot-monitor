// API response types

export type Endpoint = string;
export type ModelId = string;

export interface ModelStats {
  model: ModelId;
  endpoint: Endpoint;
  upstream_host: string;
  requests: number;
  prompt_tokens: number;
  cached_input_tokens: number;
  cache_write_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  avg_latency_ms: number;
}

export interface CostRow {
  model: ModelId;
  endpoint: Endpoint;
  upstream_host: string;
  total_usd: number;
  fallback: boolean;
  not_billed: boolean;
}

export interface CostResponse {
  total_usd: number;
  rows: CostRow[];
}

export type Granularity = 'day' | 'hour';

export interface TimelineEntry {
  date: string;
  hour?: number;
  model: ModelId;
  upstream_host?: string;
  requests: number;
  total_tokens: number;
}

export interface Session {
  id: number;
  started_at: string;
  ended_at: string;
  project: string | null;
  request_count: number;
  token_count: number;
  cost: number;
}

export interface CurrentSession {
  session: {
    id: number;
    started_at: string;
    last_request_at: string;
    active: boolean;
    request_count: number;
    token_count: number;
    cost: number;
  } | null;
  models: Array<{ model: ModelId; requests: number }>;
}

export type PeriodKey = 'today' | 'yesterday' | '7d' | '30d' | '90d' | '365d';

export interface Period {
  key: PeriodKey;
  label: string;
}


