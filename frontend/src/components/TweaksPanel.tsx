import {
  density,
  dotStyle,
  setDensity,
  setDotStyle,
  tweaksOpen,
  type Density,
  type DotStyle,
} from '../lib/tweaks';

export function TweaksPanel() {
  if (!tweaksOpen.value) return null;
  return (
    <div
      style={{
        position: 'fixed',
        right: 16,
        bottom: 16,
        width: 280,
        background: 'var(--bg-elev)',
        border: '1px solid var(--border-strong)',
        borderRadius: 10,
        boxShadow: 'var(--shadow-md)',
        zIndex: 50,
        overflow: 'hidden',
        fontSize: 12.5,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 12px',
          borderBottom: '1px solid var(--border)',
          background: 'var(--bg-sunken)',
          fontWeight: 600,
          color: 'var(--fg)',
        }}
      >
        <span>Tweaks</span>
        <button
          type="button"
          onClick={() => (tweaksOpen.value = false)}
          style={{ color: 'var(--fg-faint)', fontSize: 14, lineHeight: 1 }}
          aria-label="Close"
        >
          ×
        </button>
      </div>
      <div
        style={{
          padding: '10px 12px',
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
        }}
      >
        <Field label="Density">
          <Seg<Density>
            value={density.value}
            onSet={setDensity}
            opts={[
              { v: 'compact', label: 'compact' },
              { v: 'default', label: 'default' },
              { v: 'roomy', label: 'roomy' },
            ]}
          />
        </Field>
        <Field label="Status dot">
          <Seg<DotStyle>
            value={dotStyle.value}
            onSet={setDotStyle}
            opts={[
              { v: 'pulse', label: 'pulse' },
              { v: 'static', label: 'static' },
              { v: 'glyph', label: 'glyph' },
            ]}
          />
        </Field>
        <div
          style={{
            fontSize: 10.5,
            color: 'var(--fg-ghost)',
            lineHeight: 1.4,
          }}
        >
          Palette is chosen from the sidebar footer; the light/dark toggle
          switches variant. Toggle this panel with Shift+,.
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: preact.ComponentChildren;
}) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
      <label
        style={{
          fontSize: 11,
          letterSpacing: '0.06em',
          textTransform: 'uppercase',
          color: 'var(--fg-faint)',
          fontWeight: 600,
        }}
      >
        {label}
      </label>
      {children}
    </div>
  );
}

function Seg<T extends string>({
  value,
  onSet,
  opts,
}: {
  value: T;
  onSet: (v: T) => void;
  opts: Array<{ v: T; label: string }>;
}) {
  return (
    <div
      style={{
        display: 'flex',
        border: '1px solid var(--border)',
        borderRadius: 6,
        overflow: 'hidden',
      }}
    >
      {opts.map((o, i) => {
        const on = value === o.v;
        return (
          <button
            key={o.v}
            type="button"
            onClick={() => onSet(o.v)}
            style={{
              flex: 1,
              padding: '5px 6px',
              fontSize: 11.5,
              color: on ? 'var(--accent-ink)' : 'var(--fg-muted)',
              background: on ? 'var(--accent-soft)' : 'transparent',
              borderRight:
                i === opts.length - 1 ? 'none' : '1px solid var(--border)',
            }}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
