import { useEffect, useRef, useState } from 'preact/hooks';
import {
  palette,
  resolvedTheme,
  setPalette,
} from '../../lib/theme';
import { PALETTES } from '../../lib/palettes';

export function PalettePicker() {
  const [open, setOpen] = useState(false);
  const btnRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (btnRef.current && !btnRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const currentName = PALETTES[palette.value]?.name ?? 'Palette';
  const mode = resolvedTheme.value;
  const currentVars = PALETTES[palette.value]?.[mode];
  const swatchBg = currentVars?.['--page'] ?? '#faf4ed';
  const swatchAccent = currentVars?.['--accent'] ?? '#b4637a';

  return (
    <div style={{ position: 'relative' }} ref={btnRef}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        title="Select palette"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          padding: '5px 10px 5px 8px',
          background: 'var(--sidebar-hover)',
          border: '1px solid var(--sidebar-border)',
          borderRadius: 99,
          color: 'var(--sidebar-fg)',
          fontSize: 11.5,
        }}
      >
        <span
          style={{
            width: 14,
            height: 14,
            borderRadius: 99,
            background: `linear-gradient(135deg, ${swatchBg} 50%, ${swatchAccent} 50%)`,
            border: '1px solid var(--sidebar-border)',
            flexShrink: 0,
          }}
        />
        <span>{currentName}</span>
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 9,
            color: 'var(--sidebar-fg-faint)',
            marginLeft: 2,
          }}
        >
          {open ? '▾' : '▸'}
        </span>
      </button>

      {open && (
        <div
          style={{
            position: 'absolute',
            bottom: 'calc(100% + 6px)',
            left: 0,
            minWidth: 220,
            maxHeight: 360,
            overflow: 'auto',
            background: 'var(--bg-card)',
            border: '1px solid var(--border)',
            borderRadius: 8,
            boxShadow: 'var(--shadow-md)',
            padding: 4,
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            zIndex: 50,
          }}
        >
          {Object.entries(PALETTES).map(([k, p]) => {
            const v = p[mode];
            const bg = v['--page'];
            const card = v['--bg-card'];
            const acc = v['--accent'];
            const selected = k === palette.value;
            return (
              <button
                key={k}
                type="button"
                onClick={() => {
                  setPalette(k);
                  setOpen(false);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 9,
                  padding: '6px 10px',
                  background: selected
                    ? 'var(--sidebar-active)'
                    : 'transparent',
                  color: 'var(--sidebar-fg)',
                  fontSize: 12,
                  textAlign: 'left',
                  width: '100%',
                  borderRadius: 4,
                }}
                onMouseEnter={(e) => {
                  if (!selected)
                    (e.currentTarget as HTMLElement).style.background =
                      'var(--sidebar-hover)';
                }}
                onMouseLeave={(e) => {
                  if (!selected)
                    (e.currentTarget as HTMLElement).style.background =
                      'transparent';
                }}
              >
                <span
                  style={{
                    width: 22,
                    height: 14,
                    display: 'inline-grid',
                    gridTemplateColumns: '6px 1fr 5px',
                    borderRadius: 3,
                    overflow: 'hidden',
                    border: '1px solid var(--sidebar-border)',
                    flexShrink: 0,
                  }}
                >
                  <span style={{ background: bg }} />
                  <span style={{ background: card }} />
                  <span style={{ background: acc }} />
                </span>
                <span style={{ flex: 1 }}>{p.name}</span>
                {selected && (
                  <span
                    style={{
                      color: 'var(--sidebar-fg-faint)',
                      fontSize: 10,
                    }}
                  >
                    ●
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
