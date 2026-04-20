// Theme mode management — 'light' | 'dark' | 'system'. Persists to
// localStorage under 'ralph.theme'. Resolves 'system' against
// prefers-color-scheme at call time. Keeps data-theme + data-theme-mode on
// <html> in sync so the CSS token block under html[data-theme="..."] matches
// the active mode.

import { signal } from '@preact/signals';

export type ThemeMode = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'ralph.theme';

function preferDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolve(mode: ThemeMode): 'light' | 'dark' {
  if (mode === 'system') return preferDark() ? 'dark' : 'light';
  return mode;
}

function readStored(): ThemeMode {
  const v = localStorage.getItem(STORAGE_KEY);
  if (v === 'light' || v === 'dark' || v === 'system') return v;
  return 'system';
}

export const themeMode = signal<ThemeMode>(readStored());
export const resolvedTheme = signal<'light' | 'dark'>(resolve(themeMode.value));

function applyToDom() {
  const r = resolve(themeMode.value);
  resolvedTheme.value = r;
  document.documentElement.dataset.theme = r;
  document.documentElement.dataset.themeMode = themeMode.value;
}

export function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode;
  localStorage.setItem(STORAGE_KEY, mode);
  applyToDom();
}

// Install must be called once on app boot.
export function installTheme() {
  applyToDom();
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (themeMode.value === 'system') applyToDom();
  };
  mql.addEventListener('change', onChange);
}
