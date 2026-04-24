import type { ComponentChildren } from 'preact';
import { fieldOverrides } from '../../lib/overrides';

// Shared form primitives for the Settings editor sections. Each component
// renders a label + control + optional inline error. Sections compose these
// rather than each rendering its own copy of the layout chrome.

export function FieldRow({
  children,
}: {
  children: ComponentChildren;
}) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '180px 1fr',
        gap: 16,
        alignItems: 'center',
        padding: '8px 14px',
        borderTop: '1px solid var(--border-soft)',
      }}
    >
      {children}
    </div>
  );
}

export function FieldLabel({
  name,
  help,
}: {
  name: string;
  help?: string;
}) {
  const overrides = fieldOverrides.value[name];
  const overrideCount = overrides?.length ?? 0;
  const overrideTitle =
    overrideCount > 0
      ? overrides!.map((o) => o.name || o.fp).join(', ')
      : '';
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <code
          class="mono"
          style={{
            fontSize: 12,
            color: 'var(--fg-muted)',
          }}
        >
          {name}
        </code>
        {overrideCount > 0 && (
          <span
            title={`Overridden in: ${overrideTitle}`}
            style={{
              fontSize: 9.5,
              padding: '1px 6px',
              borderRadius: 99,
              border: '1px solid var(--warn)',
              background: 'var(--warn-soft)',
              color: 'var(--warn)',
              fontWeight: 600,
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
              lineHeight: 1.3,
            }}
          >
            {overrideCount} repo{overrideCount === 1 ? '' : 's'} override
          </span>
        )}
      </div>
      {help && (
        <span style={{ fontSize: 10.5, color: 'var(--fg-faint)' }}>
          {help}
        </span>
      )}
    </div>
  );
}

export function FieldError({ msg }: { msg?: string }) {
  if (!msg) return null;
  return (
    <div style={{ color: 'var(--err)', fontSize: 11, marginTop: 4 }}>
      {msg}
    </div>
  );
}

export function ToggleField({
  name,
  help,
  checked,
  onChange,
}: {
  name: string;
  help?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <FieldRow>
      <FieldLabel name={name} help={help} />
      <label
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          cursor: 'pointer',
          width: 'fit-content',
        }}
      >
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange((e.currentTarget as HTMLInputElement).checked)}
          style={{ cursor: 'pointer' }}
        />
        <span
          style={{
            fontSize: 12,
            color: 'var(--fg)',
            fontFamily: 'var(--font-mono)',
          }}
        >
          {checked ? 'true' : 'false'}
        </span>
      </label>
    </FieldRow>
  );
}

export function NumberField({
  name,
  help,
  value,
  onChange,
  min,
  step,
  error,
}: {
  name: string;
  help?: string;
  value: number;
  onChange: (v: number) => void;
  min?: number;
  step?: number;
  error?: string;
}) {
  return (
    <FieldRow>
      <FieldLabel name={name} help={help} />
      <div>
        <input
          type="number"
          value={Number.isFinite(value) ? String(value) : ''}
          min={min}
          step={step ?? 1}
          onInput={(e) => {
            const raw = (e.currentTarget as HTMLInputElement).value;
            const n = raw === '' ? NaN : Number(raw);
            onChange(n);
          }}
          style={{
            width: 120,
            padding: '5px 8px',
            fontSize: 12.5,
            background: 'var(--bg-card)',
            color: 'var(--fg)',
            border: `1px solid ${error ? 'var(--err)' : 'var(--border)'}`,
            borderRadius: 5,
          }}
        />
        <FieldError msg={error} />
      </div>
    </FieldRow>
  );
}

export function TextField({
  name,
  help,
  value,
  onChange,
  placeholder,
  error,
  monospace,
}: {
  name: string;
  help?: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  error?: string;
  monospace?: boolean;
}) {
  return (
    <FieldRow>
      <FieldLabel name={name} help={help} />
      <div>
        <input
          type="text"
          value={value}
          placeholder={placeholder}
          onInput={(e) =>
            onChange((e.currentTarget as HTMLInputElement).value)
          }
          style={{
            width: '100%',
            maxWidth: 380,
            padding: '5px 8px',
            fontSize: 12.5,
            background: 'var(--bg-card)',
            color: 'var(--fg)',
            border: `1px solid ${error ? 'var(--err)' : 'var(--border)'}`,
            borderRadius: 5,
            fontFamily: monospace ? 'var(--font-mono)' : undefined,
            boxSizing: 'border-box',
          }}
        />
        <FieldError msg={error} />
      </div>
    </FieldRow>
  );
}

// Common Claude model presets — duplicates the StartRunModal preset list
// so the user picks from the same names. "(custom)" means the field is
// free-text below the select; empty string means "fall back to default".
export const MODEL_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: '(default)' },
  { value: 'opus', label: 'opus' },
  { value: 'sonnet', label: 'sonnet' },
  { value: 'haiku', label: 'haiku' },
  { value: 'claude-opus-4-7', label: 'claude-opus-4-7' },
  { value: 'claude-sonnet-4-6', label: 'claude-sonnet-4-6' },
  { value: 'claude-haiku-4-5-20251001', label: 'claude-haiku-4-5' },
];

export function ModelField({
  name,
  help,
  value,
  onChange,
  error,
}: {
  name: string;
  help?: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
}) {
  // If the current value is not in the preset list, drop into free-text mode
  // so the user can tweak it without losing the existing string.
  const isCustom = value !== '' && !MODEL_OPTIONS.some((o) => o.value === value);
  return (
    <FieldRow>
      <FieldLabel name={name} help={help} />
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
        <select
          value={isCustom ? '__custom__' : value}
          onChange={(e) => {
            const v = (e.currentTarget as HTMLSelectElement).value;
            if (v === '__custom__') {
              if (!isCustom) onChange(value || 'custom-model-name');
            } else {
              onChange(v);
            }
          }}
          style={{
            padding: '5px 8px',
            fontSize: 12.5,
            background: 'var(--bg-card)',
            color: 'var(--fg)',
            border: `1px solid ${error ? 'var(--err)' : 'var(--border)'}`,
            borderRadius: 5,
            fontFamily: 'var(--font-mono)',
          }}
        >
          {MODEL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
          <option value="__custom__">custom…</option>
        </select>
        {isCustom && (
          <input
            type="text"
            value={value}
            onInput={(e) =>
              onChange((e.currentTarget as HTMLInputElement).value)
            }
            placeholder="model name"
            style={{
              padding: '5px 8px',
              fontSize: 12.5,
              minWidth: 220,
              background: 'var(--bg-card)',
              color: 'var(--fg)',
              border: `1px solid ${error ? 'var(--err)' : 'var(--border)'}`,
              borderRadius: 5,
              fontFamily: 'var(--font-mono)',
            }}
          />
        )}
      </div>
      <div style={{ gridColumn: '2 / 3' }}>
        <FieldError msg={error} />
      </div>
    </FieldRow>
  );
}

export function SectionShell({
  title,
  tomlName,
  defaultOpen = true,
  children,
}: {
  title: string;
  tomlName: string;
  defaultOpen?: boolean;
  children: ComponentChildren;
}) {
  return (
    <details
      open={defaultOpen}
      style={{
        border: '1px solid var(--border)',
        borderRadius: 8,
        background: 'var(--bg-elev)',
        marginBottom: 14,
      }}
    >
      <summary
        style={{
          padding: '10px 14px',
          cursor: 'pointer',
          listStyle: 'none',
          display: 'flex',
          alignItems: 'baseline',
          gap: 10,
          userSelect: 'none',
        }}
      >
        <span
          class="mono"
          style={{
            fontSize: 12,
            color: 'var(--fg-muted)',
          }}
        >
          [{tomlName}]
        </span>
        <span style={{ fontSize: 13, color: 'var(--fg)', fontWeight: 600 }}>
          {title}
        </span>
      </summary>
      <div style={{ borderTop: '1px solid var(--border)' }}>{children}</div>
    </details>
  );
}
