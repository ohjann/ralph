import { useEffect } from 'preact/hooks';
import { signal } from '@preact/signals';
import {
  apiGet,
  apiPost,
  ApiError,
  type SettingsValues,
  type SettingsValidationError,
} from '../lib/api';
import { pushToast } from '../lib/toast';
import { setOverrides, fieldOverrides } from '../lib/overrides';
import {
  JudgeSection,
  WorkersSection,
  QualitySection,
  ModelsSection,
  MemorySection,
  FusionSection,
  AdvancedSection,
} from '../components/Settings/sections';

interface GlobalSettingsResponse {
  config: Record<string, unknown>;
  overrides: Record<string, { fp: string; name: string }[]>;
}

const loading = signal<boolean>(false);
const saving = signal<boolean>(false);
const loadError = signal<string>('');
const original = signal<SettingsValues | null>(null);
const edited = signal<SettingsValues | null>(null);
const fieldErrors = signal<Record<string, string>>({});

function coerce(m: Record<string, unknown>): SettingsValues {
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

function isDirty(): boolean {
  if (!original.value || !edited.value) return false;
  return JSON.stringify(original.value) !== JSON.stringify(edited.value);
}

// diffPatch returns only the fields that changed between the loaded
// baseline and the edited values, so the POST payload stays minimal and
// leaves untouched fields as TomlConfig nil-pointers on the server.
function diffPatch(): SettingsValues {
  const out: SettingsValues = {};
  if (!original.value || !edited.value) return out;
  for (const [k, v] of Object.entries(edited.value)) {
    const o = (original.value as Record<string, unknown>)[k];
    if (o !== v) (out as Record<string, unknown>)[k] = v;
  }
  return out;
}

async function load() {
  loading.value = true;
  loadError.value = '';
  try {
    const r = await apiGet<GlobalSettingsResponse>('/api/settings/global');
    const v = coerce(r.config);
    original.value = v;
    edited.value = { ...v };
    setOverrides(r.overrides ?? {});
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!edited.value) return;
  const patch = diffPatch();
  if (Object.keys(patch).length === 0) return;
  saving.value = true;
  try {
    const r = await apiPost<GlobalSettingsResponse>(
      '/api/settings/global',
      patch,
    );
    const v = coerce(r.config);
    original.value = v;
    edited.value = { ...v };
    setOverrides(r.overrides ?? {});
    fieldErrors.value = {};
    pushToast('success', 'Global settings saved');
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) {
      const body = e.body as SettingsValidationError | undefined;
      if (body?.error === 'validation_failed' && body.fields) {
        fieldErrors.value = body.fields;
        const first = Object.keys(body.fields)[0];
        const msg = first ? body.fields[first] : 'invalid settings';
        pushToast('error', `Validation failed: ${first} — ${msg}`);
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
  edited.value = { ...original.value };
  fieldErrors.value = {};
}

export function GlobalSettingsRoute() {
  useEffect(() => {
    void load();
  }, []);

  if (loading.value && !edited.value) {
    return (
      <div style={{ padding: 32, fontSize: 13, color: 'var(--fg-faint)' }}>
        Loading settings…
      </div>
    );
  }
  if (loadError.value) {
    return (
      <div style={{ padding: 32, fontSize: 13, color: 'var(--err)' }}>
        Failed to load: {loadError.value}
      </div>
    );
  }
  const values = edited.value;
  if (!values) return null;

  const dirty = isDirty();
  const overrideEntries = Object.entries(fieldOverrides.value);

  return (
    <div style={{ padding: '22px 28px 80px', minHeight: '100%' }}>
      <div style={{ maxWidth: 920, margin: '0 auto' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: 16,
            marginBottom: 18,
            flexWrap: 'wrap',
          }}
        >
          <div>
            <h1
              style={{
                fontSize: 22,
                fontWeight: 600,
                letterSpacing: '-0.015em',
                margin: 0,
                color: 'var(--fg)',
              }}
            >
              Global settings
            </h1>
            <p
              style={{
                fontSize: 13,
                color: 'var(--fg-muted)',
                margin: '4px 0 0',
                lineHeight: 1.5,
              }}
            >
              Applied as defaults to every run. A repo's
              <code
                class="mono"
                style={{
                  padding: '1px 5px',
                  margin: '0 4px',
                  background: 'var(--bg-sunken)',
                  border: '1px solid var(--border-soft)',
                  borderRadius: 3,
                  fontSize: 11,
                }}
              >
                .ralph/config.toml
              </code>
              can still override any field; the badges below flag fields
              that are overridden somewhere.
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
              onClick={() => void save()}
              disabled={!dirty || saving.value}
              style={{
                padding: '6px 14px',
                fontSize: 12,
                border: '1px solid var(--accent-border)',
                borderRadius: 5,
                background: dirty && !saving.value
                  ? 'var(--accent-soft)'
                  : 'var(--bg-elev)',
                color: dirty && !saving.value
                  ? 'var(--accent-ink)'
                  : 'var(--fg-muted)',
                cursor: !dirty || saving.value ? 'not-allowed' : 'pointer',
                fontWeight: 600,
              }}
            >
              {saving.value ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>

        {overrideEntries.length > 0 && (
          <OverrideBanner entries={overrideEntries} />
        )}

        <JudgeSection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <WorkersSection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <QualitySection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <ModelsSection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <MemorySection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <FusionSection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
        <AdvancedSection
          values={values}
          errors={fieldErrors.value}
          onChange={(patch) => (edited.value = { ...values, ...patch })}
        />
      </div>
    </div>
  );
}

function OverrideBanner({
  entries,
}: {
  entries: [string, { fp: string; name: string }[]][];
}) {
  // Collect unique repos — the banner says "these repos pin some settings"
  // so users know where to look if something is being ignored.
  const repos = new Map<string, string>();
  for (const [, infos] of entries) {
    for (const r of infos) repos.set(r.fp, r.name || r.fp);
  }
  if (repos.size === 0) return null;
  return (
    <div
      style={{
        padding: '10px 14px',
        marginBottom: 18,
        background: 'var(--warn-soft)',
        border: '1px solid var(--warn)',
        borderRadius: 6,
        fontSize: 12.5,
        color: 'var(--warn)',
      }}
    >
      <strong style={{ fontWeight: 600 }}>Per-repo overrides active.</strong>{' '}
      {repos.size === 1 ? 'One repo' : `${repos.size} repos`} override{' '}
      {entries.length === 1 ? 'one field' : `${entries.length} fields`} via{' '}
      <code class="mono">.ralph/config.toml</code>. Overridden fields are
      flagged below. Affected repos:{' '}
      {Array.from(repos.values()).join(', ')}.
    </div>
  );
}
