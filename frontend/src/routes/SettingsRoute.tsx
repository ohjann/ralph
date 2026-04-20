import { useEffect } from 'preact/hooks';
import { signal } from '@preact/signals';
import { useRoute } from 'preact-iso';
import { apiGet, type SettingsResponse } from '../lib/api';
import type { DaemonStateEvent } from '../lib/live';

const loading = signal<boolean>(false);
const error = signal<string>('');
const resp = signal<SettingsResponse | null>(null);
const currentFP = signal<string>('');

async function load(fp: string) {
  if (currentFP.value === fp && resp.value) return;
  currentFP.value = fp;
  loading.value = true;
  error.value = '';
  resp.value = null;
  try {
    const r = await apiGet<SettingsResponse>(
      `/api/live/${encodeURIComponent(fp)}/settings`,
    );
    if (currentFP.value !== fp) return;
    resp.value = r;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

export function SettingsRoute() {
  const { params } = useRoute();
  const fp = params.fp;

  useEffect(() => {
    if (fp) void load(fp);
  }, [fp]);

  if (!fp) return null;
  if (loading.value && !resp.value) {
    return <div class="p-8 text-sm text-neutral-500">Loading settings…</div>;
  }
  if (error.value) {
    return (
      <div class="p-8 text-sm text-red-400">
        Failed to load settings: {error.value}
      </div>
    );
  }
  if (!resp.value) return null;

  return (
    <div class="p-6 max-w-3xl">
      <header class="mb-4">
        <h1 class="text-xl font-semibold mb-1">Settings</h1>
        <div class="text-xs text-neutral-500 font-mono">{fp}</div>
        <div class="text-[10px] uppercase tracking-wider text-amber-400 mt-2">
          Read-only
        </div>
      </header>

      {resp.value.source === 'file' && <FileBanner />}
      {resp.value.source === 'daemon' && <DaemonBanner />}

      {resp.value.source === 'file' ? (
        <FileConfig config={resp.value.config ?? {}} />
      ) : (
        <DaemonStateSummary state={resp.value.state as DaemonStateEvent} />
      )}
    </div>
  );
}

function FileBanner() {
  return (
    <div class="mb-4 p-3 rounded border border-amber-500/40 bg-amber-500/5 text-sm text-amber-200">
      <strong class="font-medium">Daemon offline.</strong>{' '}
      Showing persisted config from{' '}
      <code class="font-mono text-xs">.ralph/config.toml</code>.
    </div>
  );
}

function DaemonBanner() {
  return (
    <div class="mb-4 p-3 rounded border border-emerald-500/40 bg-emerald-500/5 text-sm text-emerald-200">
      <strong class="font-medium">Daemon reachable.</strong>{' '}
      Showing live runtime state. Persisted configuration lives in{' '}
      <code class="font-mono text-xs">.ralph/config.toml</code>.
    </div>
  );
}

function FileConfig({ config }: { config: Record<string, unknown> }) {
  const keys = Object.keys(config).sort();
  if (keys.length === 0) {
    return (
      <div class="text-sm text-neutral-500 italic">
        No persisted configuration (config.toml is missing or empty).
      </div>
    );
  }
  return (
    <section>
      <h2 class="text-xs uppercase tracking-wider text-neutral-500 mb-2">
        Configuration
      </h2>
      <dl class="border border-neutral-800 rounded divide-y divide-neutral-800">
        {keys.map((k) => (
          <div key={k} class="flex items-center gap-4 px-3 py-2">
            <dt class="text-xs text-neutral-400 font-mono w-56 shrink-0">
              {k}
            </dt>
            <dd class="text-xs text-neutral-200 font-mono break-all">
              {fmtValue(config[k])}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function DaemonStateSummary({ state }: { state: DaemonStateEvent }) {
  if (!state) {
    return (
      <div class="text-sm text-neutral-500 italic">
        Daemon returned no state snapshot.
      </div>
    );
  }
  const rows: Array<[string, string]> = [
    ['phase', state.phase || 'idle'],
    ['paused', state.paused ? 'true' : 'false'],
    ['uptime', state.uptime],
    ['workers', String(Object.keys(state.workers ?? {}).length)],
    [
      'progress',
      `${state.completed_count}/${state.total_stories}` +
        (state.failed_count > 0 ? ` · ${state.failed_count} failed` : ''),
    ],
    ['iterations', String(state.iteration_count)],
    ['total_cost', `$${state.cost_totals.total_cost.toFixed(2)}`],
    [
      'tokens',
      `in ${state.cost_totals.total_input_tokens.toLocaleString()} · out ${state.cost_totals.total_output_tokens.toLocaleString()}`,
    ],
  ];
  return (
    <section>
      <h2 class="text-xs uppercase tracking-wider text-neutral-500 mb-2">
        Live daemon state
      </h2>
      <dl class="border border-neutral-800 rounded divide-y divide-neutral-800">
        {rows.map(([k, v]) => (
          <div key={k} class="flex items-center gap-4 px-3 py-2">
            <dt class="text-xs text-neutral-400 font-mono w-32 shrink-0">
              {k}
            </dt>
            <dd class="text-xs text-neutral-200 font-mono">{v}</dd>
          </div>
        ))}
      </dl>
      <p class="text-[11px] text-neutral-500 mt-2">
        The daemon currently exposes runtime state, not its tunable config.
        To see persisted configuration, stop the daemon and reload this page.
      </p>
    </section>
  );
}

function fmtValue(v: unknown): string {
  if (v === null) return 'null';
  if (typeof v === 'string') return v;
  if (typeof v === 'boolean' || typeof v === 'number') return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}
