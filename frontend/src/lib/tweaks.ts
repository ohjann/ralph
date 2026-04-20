// Tweakable knobs that don't affect color — density of layout chrome, dot
// style for status dots. Persisted to localStorage; applied by setting
// data-attributes on <html> so the CSS rules in styles.css can take over.

import { signal } from '@preact/signals';

export type Density = 'compact' | 'default' | 'roomy';
export type DotStyle = 'pulse' | 'static' | 'glyph';

const DENSITY_KEY = 'ralph.density';
const DOTSTYLE_KEY = 'ralph.dotstyle';

function readDensity(): Density {
  const v = localStorage.getItem(DENSITY_KEY);
  return v === 'compact' || v === 'roomy' ? v : 'default';
}

function readDotStyle(): DotStyle {
  const v = localStorage.getItem(DOTSTYLE_KEY);
  return v === 'static' || v === 'glyph' ? v : 'pulse';
}

export const density = signal<Density>(readDensity());
export const dotStyle = signal<DotStyle>(readDotStyle());

function applyDensity(d: Density) {
  if (d === 'default') delete document.documentElement.dataset.density;
  else document.documentElement.dataset.density = d;
}
function applyDotStyle(s: DotStyle) {
  document.documentElement.dataset.dotstyle = s;
}

export function setDensity(d: Density) {
  density.value = d;
  localStorage.setItem(DENSITY_KEY, d);
  applyDensity(d);
}
export function setDotStyle(s: DotStyle) {
  dotStyle.value = s;
  localStorage.setItem(DOTSTYLE_KEY, s);
  applyDotStyle(s);
}

// Global toggle signal for the Tweaks panel open state.
export const tweaksOpen = signal<boolean>(false);

export function installTweaks() {
  applyDensity(density.value);
  applyDotStyle(dotStyle.value);
  // Keyboard shortcut: Shift+, (comma) — matches the design's "tweaks" intent
  // of being dev-ish. Ignored if focus is in a text input.
  window.addEventListener('keydown', (e) => {
    const target = e.target as HTMLElement | null;
    if (
      target &&
      (target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable)
    )
      return;
    if (e.shiftKey && e.key === ',') {
      e.preventDefault();
      tweaksOpen.value = !tweaksOpen.value;
    }
  });
}
