// Shared signal for the global settings editor. Each setting-field looks
// itself up here to render a small badge when one or more repos locally
// override it via .ralph/config.toml. Kept module-global so the existing
// Settings field components can subscribe without any prop-drilling.

import { signal } from '@preact/signals';

export interface OverrideRepoInfo {
  fp: string;
  name: string;
}

export const fieldOverrides = signal<Record<string, OverrideRepoInfo[]>>({});

export function clearOverrides(): void {
  fieldOverrides.value = {};
}

export function setOverrides(map: Record<string, OverrideRepoInfo[]>): void {
  fieldOverrides.value = map || {};
}
