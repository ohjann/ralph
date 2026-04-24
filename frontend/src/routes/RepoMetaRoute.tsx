import { useEffect } from 'preact/hooks';
import { signal } from '@preact/signals';
import { useRoute } from 'preact-iso';
import {
  apiGet,
  apiPost,
  ApiError,
  type RepoMetaResponse,
  type SpawnRetroResponse,
} from '../lib/api';
import { pushToast } from '../lib/toast';
import { refreshRepoRuns } from '../components/Sidebar/Sidebar';

const loading = signal<boolean>(false);
const error = signal<string>('');
const resp = signal<RepoMetaResponse | null>(null);
const currentFP = signal<string>('');
const retroBusy = signal<boolean>(false);

async function triggerRetro(fp: string): Promise<void> {
  if (retroBusy.value) return;
  retroBusy.value = true;
  try {
    const out = await apiPost<SpawnRetroResponse>(
      `/api/spawn/retro/${encodeURIComponent(fp)}`,
    );
    const tail = out.runId
      ? `run ${out.runId.slice(0, 12)}`
      : `pid ${out.pid}`;
    pushToast('success', `Retro started — ${tail}`);
    refreshRepoRuns(fp);
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      const body = e.body as { runId?: string } | undefined;
      const suffix = body?.runId ? ` (${body.runId.slice(0, 8)}…)` : '';
      pushToast('warn', `A retro is already running for this repo${suffix}.`);
      return;
    }
    const msg = e instanceof Error ? e.message : String(e);
    pushToast('error', `Retro failed to start: ${msg}`);
  } finally {
    retroBusy.value = false;
  }
}

async function load(fp: string) {
  if (currentFP.value === fp && resp.value) return;
  currentFP.value = fp;
  loading.value = true;
  error.value = '';
  resp.value = null;
  try {
    const r = await apiGet<RepoMetaResponse>(
      `/api/repos/${encodeURIComponent(fp)}/meta`,
    );
    if (currentFP.value !== fp) return;
    resp.value = r;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export function RepoMetaRoute() {
  const { params } = useRoute();
  const fp = params.fp;

  useEffect(() => {
    if (fp) void load(fp);
  }, [fp]);

  if (!fp) return null;
  if (loading.value && !resp.value) {
    return (
      <div style={{ padding: 32, fontSize: 13, color: 'var(--fg-faint)' }}>
        Loading repo meta…
      </div>
    );
  }
  if (error.value) {
    return (
      <div style={{ padding: 32, fontSize: 13, color: 'var(--err)' }}>
        Failed to load: {error.value}
      </div>
    );
  }
  if (!resp.value) return null;

  const { meta, aggCosts, runCountsByKind } = resp.value;
  const kinds = Object.entries(runCountsByKind).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  const totalByKind = kinds.reduce((s, [, n]) => s + n, 0);

  return (
    <div style={{ padding: 24, maxWidth: 960 }}>
      <header
        style={{
          marginBottom: 24,
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ minWidth: 0 }}>
          <h1
            style={{
              fontSize: 22,
              fontWeight: 600,
              letterSpacing: '-0.015em',
              margin: 0,
              color: 'var(--fg)',
            }}
          >
            {meta.name || meta.path}
          </h1>
          <div
            class="mono"
            style={{
              fontSize: 11.5,
              color: 'var(--fg-faint)',
              marginTop: 4,
              wordBreak: 'break-all',
            }}
          >
            {meta.path}
          </div>
        </div>
        <RetroButton busy={retroBusy.value} onClick={() => void triggerRetro(fp)} />
      </header>

      <SectionHeader>Identity</SectionHeader>
      <dl
        style={{
          margin: '0 0 24px',
          border: '1px solid var(--border)',
          borderRadius: 8,
          background: 'var(--bg-elev)',
          overflow: 'hidden',
        }}
      >
        <Row icon="link" label="Fingerprint" value={fp} mono first />
        <Row
          icon="code"
          label="First git SHA"
          value={meta.git_first_sha ? meta.git_first_sha.slice(0, 16) : '—'}
          mono
        />
        <Row icon="clock" label="First seen" value={fmtTime(meta.first_seen)} />
        <Row icon="clock" label="Last seen" value={fmtTime(meta.last_seen)} />
        <Row
          icon="link"
          label="Last run"
          value={meta.last_run_id ? meta.last_run_id.slice(0, 16) : '—'}
          mono
        />
        <Row label="Total invocations" value={String(meta.run_count)} mono />
      </dl>

      <SectionHeader>Aggregate stats</SectionHeader>
      <div
        style={{
          marginBottom: 24,
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
          gap: 8,
        }}
      >
        <Card label="Stored runs" value={String(aggCosts.runs)} />
        <Card
          label="Total cost"
          value={`$${aggCosts.totalCost.toFixed(2)}`}
          accent
        />
        <Card label="Duration" value={fmtDuration(aggCosts.durationMinutes)} />
        <Card label="Iterations" value={String(aggCosts.totalIterations)} />
        <Card label="Stories total" value={String(aggCosts.storiesTotal)} />
        <Card label="Stories done" value={String(aggCosts.storiesCompleted)} />
        <Card
          label="Stories failed"
          value={String(aggCosts.storiesFailed)}
          tone={aggCosts.storiesFailed > 0 ? 'warn' : undefined}
        />
        {aggCosts.storiesTotal > 0 && (
          <Card
            label="Completion rate"
            value={`${Math.round(
              (aggCosts.storiesCompleted / aggCosts.storiesTotal) * 100,
            )}%`}
          />
        )}
      </div>

      <SectionHeader>Runs by kind</SectionHeader>
      {kinds.length === 0 ? (
        <div
          style={{
            fontSize: 13,
            color: 'var(--fg-faint)',
            fontStyle: 'italic',
          }}
        >
          No stored manifests.
        </div>
      ) : (
        <table
          style={{
            width: '100%',
            borderCollapse: 'collapse',
            fontSize: 12,
            border: '1px solid var(--border)',
            borderRadius: 8,
            overflow: 'hidden',
            background: 'var(--bg-elev)',
          }}
        >
          <thead>
            <tr>
              <TH>Kind</TH>
              <TH>Count</TH>
              <TH>Share</TH>
            </tr>
          </thead>
          <tbody>
            {kinds.map(([k, n], i) => (
              <tr
                key={k}
                style={{
                  borderTop: i === 0 ? 'none' : '1px solid var(--border-soft)',
                }}
              >
                <TD mono>{k}</TD>
                <TD mono>{n}</TD>
                <TD mono muted>
                  {totalByKind > 0
                    ? `${Math.round((n / totalByKind) * 100)}%`
                    : '—'}
                </TD>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function SectionHeader({ children }: { children: preact.ComponentChildren }) {
  return (
    <h2
      style={{
        fontSize: 12.5,
        fontWeight: 600,
        margin: '0 0 10px',
        color: 'var(--fg-muted)',
        textTransform: 'uppercase',
        letterSpacing: '0.07em',
      }}
    >
      {children}
    </h2>
  );
}

function RetroButton({ busy, onClick }: { busy: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      style={{
        flexShrink: 0,
        display: 'inline-flex',
        alignItems: 'center',
        gap: 7,
        padding: '7px 12px',
        fontSize: 12.5,
        fontWeight: 600,
        borderRadius: 6,
        border: '1px solid var(--accent-border)',
        background: 'var(--accent-soft)',
        color: 'var(--accent-ink)',
        cursor: busy ? 'not-allowed' : 'pointer',
        opacity: busy ? 0.6 : 1,
      }}
    >
      <RetroIcon />
      {busy ? 'Starting…' : 'Run retrospective'}
    </button>
  );
}

function RetroIcon() {
  return (
    <svg
      width="13"
      height="13"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2.2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <polyline points="3 4 3 10 9 10" />
    </svg>
  );
}

type RowIcon = 'link' | 'code' | 'clock';

function Row({
  icon,
  label,
  value,
  mono,
  first,
}: {
  icon?: RowIcon;
  label: string;
  value: string;
  mono?: boolean;
  first?: boolean;
}) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '8px 14px',
        borderTop: first ? 'none' : '1px solid var(--border-soft)',
      }}
    >
      <dt
        style={{
          fontSize: 11.5,
          color: 'var(--fg-faint)',
          width: 160,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
        }}
      >
        {icon && <RowIconGlyph name={icon} />}
        <span>{label}</span>
      </dt>
      <dd
        class={mono ? 'mono' : ''}
        style={{
          fontSize: mono ? 12 : 13,
          color: 'var(--fg)',
          margin: 0,
          wordBreak: 'break-all',
        }}
      >
        {value}
      </dd>
    </div>
  );
}

function RowIconGlyph({ name }: { name: RowIcon }) {
  const props = {
    width: 12,
    height: 12,
    viewBox: '0 0 16 16',
    fill: 'none',
    stroke: 'currentColor',
    'stroke-width': 1.6,
    'stroke-linecap': 'round' as const,
    'stroke-linejoin': 'round' as const,
    style: { color: 'var(--fg-faint)', flexShrink: 0 },
    'aria-hidden': true,
  };
  switch (name) {
    case 'link':
      return (
        <svg {...props}>
          <path d="M7 9a3 3 0 0 0 4 0l2-2a3 3 0 0 0-4-4l-1 1M9 7a3 3 0 0 0-4 0l-2 2a3 3 0 0 0 4 4l1-1" />
        </svg>
      );
    case 'code':
      return (
        <svg {...props}>
          <path d="m5 4-3 4 3 4M11 4l3 4-3 4M9 3 7 13" />
        </svg>
      );
    case 'clock':
      return (
        <svg {...props}>
          <circle cx="8" cy="8" r="5.5" />
          <path d="M8 5v3l2 1" />
        </svg>
      );
  }
}

function Card({
  label,
  value,
  tone,
  accent,
}: {
  label: string;
  value: string;
  tone?: 'warn';
  accent?: boolean;
}) {
  const valueColor =
    tone === 'warn'
      ? 'var(--warn)'
      : accent
        ? 'var(--accent-ink)'
        : 'var(--fg)';
  return (
    <div
      style={{
        border: '1px solid var(--border)',
        borderRadius: 8,
        padding: '10px 12px',
        background: 'var(--bg-elev)',
        minWidth: 0,
      }}
    >
      <div
        style={{
          fontSize: 10.5,
          color: 'var(--fg-faint)',
          textTransform: 'uppercase',
          letterSpacing: '0.07em',
          fontWeight: 600,
        }}
      >
        {label}
      </div>
      <div
        class="mono"
        style={{
          fontSize: 18,
          fontWeight: 500,
          marginTop: 2,
          color: valueColor,
          letterSpacing: '-0.01em',
        }}
      >
        {value}
      </div>
    </div>
  );
}

function TH({ children }: { children: preact.ComponentChildren }) {
  return (
    <th
      style={{
        textAlign: 'left',
        fontWeight: 500,
        padding: '8px 12px',
        fontSize: 11,
        color: 'var(--fg-faint)',
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        borderBottom: '1px solid var(--border-soft)',
        background: 'var(--bg-sunken)',
      }}
    >
      {children}
    </th>
  );
}

function TD({
  children,
  mono,
  muted,
}: {
  children: preact.ComponentChildren;
  mono?: boolean;
  muted?: boolean;
}) {
  return (
    <td
      class={mono ? 'mono' : ''}
      style={{
        padding: '8px 12px',
        fontSize: mono ? 12 : 13,
        color: muted ? 'var(--fg-faint)' : 'var(--fg)',
      }}
    >
      {children}
    </td>
  );
}

function fmtDuration(minutes: number): string {
  if (minutes < 1) return `${Math.round(minutes * 60)}s`;
  if (minutes < 60) return `${minutes.toFixed(1)}m`;
  const h = Math.floor(minutes / 60);
  const m = Math.round(minutes % 60);
  return `${h}h ${m}m`;
}
