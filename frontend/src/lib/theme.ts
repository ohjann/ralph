// Theme mode + palette selection. Both persist to localStorage and are
// applied to the document on every mode/palette change. Signals drive
// reactive consumers in the sidebar footer.

import { signal } from '@preact/signals';
import { applyPaletteToDom, PALETTES } from './palettes';

export type ThemeMode = 'light' | 'dark' | 'system';

const MODE_KEY = 'ralph.theme';
const PALETTE_KEY = 'ralph.palette';
const DEFAULT_PALETTE = 'rose-pine';

function preferDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolve(mode: ThemeMode): 'light' | 'dark' {
  if (mode === 'system') return preferDark() ? 'dark' : 'light';
  return mode;
}

function readStoredMode(): ThemeMode {
  const v = localStorage.getItem(MODE_KEY);
  if (v === 'light' || v === 'dark' || v === 'system') return v;
  return 'system';
}

function readStoredPalette(): string {
  const v = localStorage.getItem(PALETTE_KEY);
  return v && PALETTES[v] ? v : DEFAULT_PALETTE;
}

export const themeMode = signal<ThemeMode>(readStoredMode());
export const resolvedTheme = signal<'light' | 'dark'>(
  resolve(themeMode.value),
);
export const palette = signal<string>(readStoredPalette());

function applyAll() {
  const r = resolve(themeMode.value);
  resolvedTheme.value = r;
  document.documentElement.dataset.theme = r;
  document.documentElement.dataset.themeMode = themeMode.value;
  applyPaletteToDom(palette.value, r);
}

export function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode;
  localStorage.setItem(MODE_KEY, mode);
  applyAll();
}

export function setPalette(name: string) {
  if (!PALETTES[name]) return;
  palette.value = name;
  localStorage.setItem(PALETTE_KEY, name);
  applyAll();
}

export function installTheme() {
  applyAll();
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  mql.addEventListener('change', () => {
    if (themeMode.value === 'system') applyAll();
  });
}
