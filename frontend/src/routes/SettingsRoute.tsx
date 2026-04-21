import { useEffect } from 'preact/hooks';
import { signal } from '@preact/signals';
import { useRoute } from 'preact-iso';
import {
  apiGet,
  apiPost,
  ApiError,
  type SettingsResponse,
  type SettingsValues,
  type SettingsUpdateResponse,
  type SettingsValidationError,
} from '../lib/api';
import type { DaemonStateEvent, SettingsSnapshot } from '../lib/live';
import { pushToast } from '../lib/toast';
import {
  JudgeSection,
  WorkersSection,
  QualitySection,
  ModelsSection,
  MemorySection,
  FusionSection,
  AdvancedSection,
} from '../components/Settings/sections';

// Module-scope signals so route remounts (e.g. after a Save → re-route)
// keep the form state. Reset via load(fp) when the user navigates to a
// different repo.
const loading = signal<boolean>(false);
const saving = signal<boolean>(false);
const error = signal<string>('');
const source = signal<'daemon' | 'file'>('file');
const daemonState = signal<DaemonStateEvent | null>(null);
const original = signal<SettingsValues | null>(null);
const edited = signal<SettingsValues | null>(null);
const fieldErrors = signal<Record<string, string>>({});
const currentFP = signal<string>('');

// snapshotToValues hoists the daemon's SettingsSnapshot (live tunable
// values) into the SettingsValues shape the editor edits.
function snapshotToValues(s: SettingsSnapshot): SettingsValues {
  return {
    judge_enabled: s.judge_enabled,
    judge_max_rejections: s.judge_max_rejections,
    workers: s.workers,
    workers_auto: s.workers_auto,
    auto_max_workers: s.auto_max_workers,
    quality_review: s.quality_review,
    quality_workers: s.quality_workers,
    quality_max_iterations: s.quality_max_iterations,
    memory_disable: s.memory_disable,
    no_architect: s.no_architect,
    no_simplify: s.no_simplify,
    no_fusion: s.no_fusion,
    fusion_workers: s.fusion_workers,
    sprite_enabled: s.sprite_enabled,
    workspace_base: s.workspace_base,
    model_override: s.model_override,
    architect_model: s.architect_model,
    implementer_model: s.implementer_model,
    utility_model: s.utility_model,
  };
}

// fileMapToValues coerces the loose Record<string,unknown> the GET handler
// returns from the file source into typed SettingsValues. Skips unknown
// keys silently so the editor only ever sees the 19 tunables.
function fileMapToValues(m: Record<string, unknown>): SettingsValues {
  const out: SettingsValues = {};
  const b = (k: keyof SettingsValues) => {
    const v = m[k as string];
    if (typeof v === 'boolean') (out as Record<string, unknown>)[k] = v;
  };
  const n = (k: keyof SettingsValues) => {
    const v = m[k as string];
    if (typeof v === 'number') (out as Record<string, unknown>)[k] = v;
  };
  const s = (k: keyof SettingsValues) => {
    const v = m[k as string];
    if (typeof v === 'string') (out as Record<string, unknown>)[k] = v;
  };
  b('judge_enabled');
  n('judge_max_rejections');
  n('workers');
  b('workers_auto');
  n('auto_max_workers');
  b('quality_review');
  n('quality_workers');
  n('quality_max_iterations');
  b('memory_disable');
  b('no_architect');
  b('no_simplify');
  b('no_fusion');
  n('fusion_workers');
  b('sprite_enabled');
  s('workspace_base');
  s('model_override');
  s('architect_model');
  s('implementer_model');
  s('utility_model');
  return out;
}

function defaults(): SettingsValues {
  return {
    judge_enabled: true,
    judge_max_rejections: 2,
    workers: 1,
    workers_auto: false,
    auto_max_workers: 5,
    quality_review: true,
    quality_workers: 3,
    quality_max_iterations: 2,
    memory_disable: false,
    no_architect: false,
    no_simplify: false,
    no_fusion: false,
    fusion_workers: 2,
    sprite_enabled: true,
    workspace_base: '',
    model_override: '',
    architect_model: '',
    implementer_model: '',
    utility_model: 'haiku',
  };
}

function deepClone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}

function isDirty(): boolean {
  if (!original.value || !edited.value) return false;
  return JSON.stringify(original.value) !== JSON.stringify(edited.value);
}

// validateClient mirrors config.TomlConfig.Validate so Save can disable in
// real time without round-tripping. Server validation still runs and is
// authoritative.
function validateClient(v: SettingsValues): Record<string, string> {
  const errs: Record<string, string> = {};
  if (v.workers !== undefined && (!Number.isFinite(v.workers) || v.workers < 1)) {
    errs.workers = 'must be >= 1';
  }
  if (
    v.auto_max_workers !== undefined &&
    v.workers !== undefined &&
    Number.isFinite(v.auto_max_workers) &&
    Number.isFinite(v.workers) &&
    v.auto_max_workers < v.workers
  ) {
    errs.auto_max_workers = 'must be >= workers';
  }
  if (
    v.quality_workers !== undefined &&
    (!Number.isFinite(v.quality_workers) || v.quality_workers < 1)
  ) {
    errs.quality_workers = 'must be >= 1';
  }
  if (
    v.quality_max_iterations !== undefined &&
    (!Number.isFinite(v.quality_max_iterations) || v.quality_max_iterations < 1)
  ) {
    errs.quality_max_iterations = 'must be >= 1';
  }
  if (
    v.fusion_workers !== undefined &&
    (!Number.isFinite(v.fusion_workers) || v.fusion_workers < 2)
  ) {
    errs.fusion_workers = 'must be >= 2';
  }
  if (
    v.judge_max_rejections !== undefined &&
    (!Number.isFinite(v.judge_max_rejections) || v.judge_max_rejections < 0)
  ) {
    errs.judge_max_rejections = 'must be >= 0';
  }
  // Model-name presence: empty string is fine (= "use default"), but a
  // value that is whitespace-only is rejected — the AC requires "model
  // names non-empty when set".
  const checkModel = (key: keyof SettingsValues) => {
    const val = v[key];
    if (typeof val === 'string' && val !== '' && val.trim() === '') {
      errs[key as string] = 'model name cannot be blank';
    }
  };
  checkModel('model_override');
  checkModel('architect_model');
  checkModel('implementer_model');
  checkModel('utility_model');
  return errs;
}

async function load(fp: string) {
  if (currentFP.value === fp && edited.value) return;
  currentFP.value = fp;
  loading.value = true;
  error.value = '';
  fieldErrors.value = {};
  daemonState.value = null;
  try {
    const r = await apiGet<SettingsResponse>(
      `/api/live/${encodeURIComponent(fp)}/settings`,
    );
    if (currentFP.value !== fp) return;
    source.value = r.source;
    let values: SettingsValues;
    if (r.source === 'daemon' && r.state) {
      const state = r.state as DaemonStateEvent;
      daemonState.value = state;
      values = state.settings
        ? snapshotToValues(state.settings)
        : defaults();
    } else {
      values = { ...defaults(), ...fileMapToValues(r.config ?? {}) };
    }
    original.value = values;
    edited.value = deepClone(values);
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

// diffPatch returns only the fields that differ from original — keeps the
// POST body minimal so the daemon's "applied" list reflects the user's
// actual intent rather than every field in the form.
function diffPatch(o: SettingsValues, e: SettingsValues): SettingsValues {
  const out: SettingsValues = {};
  (Object.keys(e) as Array<keyof SettingsValues>).forEach((k) => {
    if (JSON.stringify(o[k]) !== JSON.stringify(e[k])) {
      (out as Record<string, unknown>)[k as string] = e[k];
    }
  });
  return out;
}

async function save(fp: string) {
  if (!edited.value || !original.value) return;
  saving.value = true;
  fieldErrors.value = {};
  try {
    const patch = diffPatch(original.value, edited.value);
    const resp = await apiPost<SettingsUpdateResponse>(
      `/api/live/${encodeURIComponent(fp)}/settings`,
      patch,
    );
    // Mark current edited as the new baseline.
    original.value = deepClone(edited.value);
    source.value = resp.source;
    pushToast(
      'success',
      resp.source === 'daemon'
        ? `Settings applied live (${resp.applied.length} fields).`
        : `Settings saved to config.toml (${resp.applied.length} fields).`,
    );
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) {
      const body = e.body as SettingsValidationError | undefined;
      if (body?.error === 'validation_failed' && body.fields) {
        fieldErrors.value = body.fields;
        const firstKey = Object.keys(body.fields)[0];
        pushToast(
          'error',
          `Validation failed: ${firstKey} — ${body.fields[firstKey]}`,
        );
        return;
      }
    }
    pushToast('error', e instanceof Error ? e.message : String(e));
  } finally {
    saving.value = false;
  }
}

function discard() {
  if (!original.value) return;
  edited.value = deepClone(original.value);
  fieldErrors.value = {};
}

export function SettingsRoute() {
  const { params } = useRoute();
  const fp = params.fp;
  useEffect(() => {
    if (fp) void load(fp);
  }, [fp]);

  if (!fp) return null;
  if (loading.value && !edited.value)
    return (
      <div style={{ padding: 32, color: 'var(--fg-faint)' }}>
        Loading settings…
      </div>
    );
  if (error.value)
    return (
      <div style={{ padding: 32, color: 'var(--err)' }}>
        Failed to load: {error.value}
      </div>
    );
  const cur = edited.value;
  if (!cur) return null;

  const dirty = isDirty();
  const clientErrors = validateClient(cur);
  const allErrors: Record<string, string> = {
    ...clientErrors,
    ...fieldErrors.value,
  };
  const hasErrors = Object.keys(allErrors).length > 0;
  const saveDisabled = !dirty || saving.value || hasErrors;

  const onChange = (patch: SettingsValues) => {
    edited.value = { ...cur, ...patch };
    if (Object.keys(fieldErrors.value).length > 0) fieldErrors.value = {};
  };

  return (
    <div style={{ padding: '22px 28px 80px', minHeight: '100%' }}>
      <div style={{ maxWidth: 920, margin: '0 auto' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            fontSize: 12,
            color: 'var(--fg-faint)',
            fontFamily: 'var(--font-mono)',
            marginBottom: 12,
          }}
        >
          <span>system</span>
          <span style={{ color: 'var(--fg-ghost)' }}>/</span>
          <span style={{ color: 'var(--fg)' }}>settings</span>
        </div>

        <div
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'space-between',
            gap: 16,
            marginBottom: 14,
            flexWrap: 'wrap',
          }}
        >
          <div>
            <h2
              style={{
                fontSize: 18,
                fontWeight: 600,
                letterSpacing: '-0.01em',
                margin: '0 0 4px',
                color: 'var(--fg)',
              }}
            >
              Daemon configuration
            </h2>
            <p style={{ fontSize: 12, color: 'var(--fg-muted)', margin: 0 }}>
              Edit every tunable from the viewer. Saves persist to{' '}
              <code class="mono">.ralph/config.toml</code>; when the daemon
              is up, changes apply live.
            </p>
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            {dirty && (
              <span
                style={{
                  fontSize: 11,
                  color: 'var(--warn)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  fontWeight: 600,
                }}
              >
                unsaved
              </span>
            )}
            <button
              type="button"
              onClick={discard}
              disabled={!dirty || saving.value}
              style={{
                padding: '6px 12px',
                fontSize: 12,
                border: '1px solid var(--border)',
                borderRadius: 5,
                background: 'transparent',
                color: 'var(--fg)',
                cursor: !dirty || saving.value ? 'not-allowed' : 'pointer',
                opacity: !dirty || saving.value ? 0.5 : 1,
              }}
            >
              Discard
            </button>
            <button
              type="button"
              onClick={() => void save(fp)}
              disabled={saveDisabled}
              style={{
                padding: '6px 14px',
                fontSize: 12,
                border: '1px solid var(--ok)',
                borderRadius: 5,
                background: saveDisabled ? 'var(--bg-elev)' : 'var(--ok)',
                color: saveDisabled ? 'var(--fg-muted)' : 'white',
                cursor: saveDisabled ? 'not-allowed' : 'pointer',
                fontWeight: 600,
              }}
            >
              {saving.value ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>

        <SourceBanner src={source.value} />

        <JudgeSection values={cur} errors={allErrors} onChange={onChange} />
        <WorkersSection values={cur} errors={allErrors} onChange={onChange} />
        <QualitySection values={cur} errors={allErrors} onChange={onChange} />
        <ModelsSection values={cur} errors={allErrors} onChange={onChange} />
        <MemorySection values={cur} errors={allErrors} onChange={onChange} />
        <FusionSection values={cur} errors={allErrors} onChange={onChange} />
        <AdvancedSection values={cur} errors={allErrors} onChange={onChange} />

        {daemonState.value && (
          <DaemonStateSummary state={daemonState.value} />
        )}
      </div>
    </div>
  );
}

function SourceBanner({ src }: { src: 'daemon' | 'file' }) {
  if (src === 'daemon') {
    return (
      <div
        style={{
          padding: '10px 14px',
          background: 'var(--ok-soft)',
          color: 'var(--ok)',
          border: '1px solid var(--ok)',
          borderRadius: 6,
          fontSize: 13,
          marginBottom: 16,
        }}
      >
        <strong style={{ fontWeight: 600 }}>Daemon reachable.</strong>{' '}
        Changes apply live.
      </div>
    );
  }
  return (
    <div
      style={{
        padding: '10px 14px',
        background: 'var(--warn-soft)',
        color: 'var(--warn)',
        border: '1px solid var(--warn)',
        borderRadius: 6,
        fontSize: 13,
        marginBottom: 16,
      }}
    >
      <strong style={{ fontWeight: 600 }}>Daemon offline.</strong> Changes
      written to <code class="mono">config.toml</code> and apply on next
      start.
    </div>
  );
}

function DaemonStateSummary({ state }: { state: DaemonStateEvent }) {
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
    <div style={{ marginTop: 22 }}>
      <h3
        style={{
          fontSize: 12,
          fontWeight: 600,
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          color: 'var(--fg-muted)',
          margin: '0 0 8px',
        }}
      >
        Live runtime state
      </h3>
      <dl
        style={{
          border: '1px solid var(--border)',
          borderRadius: 8,
          overflow: 'hidden',
          background: 'var(--bg-elev)',
          margin: 0,
        }}
      >
        {rows.map(([k, v], i) => (
          <div
            key={k}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 16,
              padding: '8px 14px',
              borderTop: i === 0 ? 'none' : '1px solid var(--border-soft)',
            }}
          >
            <dt
              class="mono"
              style={{
                fontSize: 12,
                color: 'var(--fg-muted)',
                width: 140,
                flexShrink: 0,
              }}
            >
              {k}
            </dt>
            <dd
              class="mono"
              style={{ fontSize: 12, color: 'var(--fg)', margin: 0 }}
            >
              {v}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
