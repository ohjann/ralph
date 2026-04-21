import type { SettingsValues } from '../../lib/api';
import {
  ToggleField,
  NumberField,
  TextField,
  ModelField,
  SectionShell,
} from './fields';

// Each section maps a slice of SettingsValues to a small group of fields.
// All sections share the same prop shape: the values being edited, a
// fieldErrors map keyed by TOML tag name, and an onChange that applies a
// partial patch up to the route. Splitting per-section keeps the route
// readable and lets the AC's "[advanced] collapsed by default" work via
// the SectionShell `defaultOpen` flag.

export interface SectionProps {
  values: SettingsValues;
  errors: Record<string, string>;
  onChange: (patch: SettingsValues) => void;
}

export function JudgeSection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Judge" tomlName="judge">
      <ToggleField
        name="judge_enabled"
        help="LLM-as-Judge verification after each implementer pass."
        checked={values.judge_enabled ?? true}
        onChange={(v) => onChange({ judge_enabled: v })}
      />
      <NumberField
        name="judge_max_rejections"
        help="Max judge rejections per story before auto-passing."
        value={values.judge_max_rejections ?? 2}
        min={0}
        onChange={(v) => onChange({ judge_max_rejections: v })}
        error={errors.judge_max_rejections}
      />
    </SectionShell>
  );
}

export function WorkersSection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Workers" tomlName="workers">
      <NumberField
        name="workers"
        help="Number of parallel implementer workers."
        value={values.workers ?? 1}
        min={1}
        onChange={(v) => onChange({ workers: v })}
        error={errors.workers}
      />
      <ToggleField
        name="workers_auto"
        help="Scale worker count to DAG width (capped by auto_max_workers)."
        checked={values.workers_auto ?? false}
        onChange={(v) => onChange({ workers_auto: v })}
      />
      <NumberField
        name="auto_max_workers"
        help="Cap for auto-scaled worker count."
        value={values.auto_max_workers ?? 5}
        min={1}
        onChange={(v) => onChange({ auto_max_workers: v })}
        error={errors.auto_max_workers}
      />
    </SectionShell>
  );
}

export function QualitySection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Quality" tomlName="quality">
      <ToggleField
        name="quality_review"
        help="Run final quality review pass."
        checked={values.quality_review ?? true}
        onChange={(v) => onChange({ quality_review: v })}
      />
      <NumberField
        name="quality_workers"
        help="Parallel quality reviewers."
        value={values.quality_workers ?? 3}
        min={1}
        onChange={(v) => onChange({ quality_workers: v })}
        error={errors.quality_workers}
      />
      <NumberField
        name="quality_max_iterations"
        help="Max review-fix cycles per story."
        value={values.quality_max_iterations ?? 2}
        min={1}
        onChange={(v) => onChange({ quality_max_iterations: v })}
        error={errors.quality_max_iterations}
      />
    </SectionShell>
  );
}

export function ModelsSection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Models" tomlName="models">
      <ModelField
        name="model_override"
        help="Default model for every role. Role-specific overrides take precedence."
        value={values.model_override ?? ''}
        onChange={(v) => onChange({ model_override: v })}
        error={errors.model_override}
      />
      <ModelField
        name="architect_model"
        help="Architect role only."
        value={values.architect_model ?? ''}
        onChange={(v) => onChange({ architect_model: v })}
        error={errors.architect_model}
      />
      <ModelField
        name="implementer_model"
        help="Implementer role only."
        value={values.implementer_model ?? ''}
        onChange={(v) => onChange({ implementer_model: v })}
        error={errors.implementer_model}
      />
      <ModelField
        name="utility_model"
        help="DAG analysis and other utility tasks."
        value={values.utility_model ?? ''}
        onChange={(v) => onChange({ utility_model: v })}
        error={errors.utility_model}
      />
    </SectionShell>
  );
}

export function MemorySection({ values, onChange }: SectionProps) {
  return (
    <SectionShell title="Memory" tomlName="memory">
      <ToggleField
        name="memory_disable"
        help="Skip the markdown memory injection step entirely."
        checked={values.memory_disable ?? false}
        onChange={(v) => onChange({ memory_disable: v })}
      />
    </SectionShell>
  );
}

export function FusionSection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Fusion" tomlName="fusion">
      <ToggleField
        name="no_fusion"
        help="Disable automatic fusion mode for complex stories."
        checked={values.no_fusion ?? false}
        onChange={(v) => onChange({ no_fusion: v })}
      />
      <NumberField
        name="fusion_workers"
        help="Competing implementations per complex story (min 2)."
        value={values.fusion_workers ?? 2}
        min={2}
        onChange={(v) => onChange({ fusion_workers: v })}
        error={errors.fusion_workers}
      />
    </SectionShell>
  );
}

export function AdvancedSection({ values, errors, onChange }: SectionProps) {
  return (
    <SectionShell title="Advanced" tomlName="advanced" defaultOpen={false}>
      <ToggleField
        name="no_architect"
        help="Globally skip the architect phase."
        checked={values.no_architect ?? false}
        onChange={(v) => onChange({ no_architect: v })}
      />
      <ToggleField
        name="no_simplify"
        help="Skip the per-story simplify pass."
        checked={values.no_simplify ?? false}
        onChange={(v) => onChange({ no_simplify: v })}
      />
      <ToggleField
        name="sprite_enabled"
        help="Sprite mascot overlay in the TUI."
        checked={values.sprite_enabled ?? true}
        onChange={(v) => onChange({ sprite_enabled: v })}
      />
      <TextField
        name="workspace_base"
        help="Base directory for per-story git worktrees."
        value={values.workspace_base ?? ''}
        placeholder="/tmp/ralph-workspaces"
        monospace
        onChange={(v) => onChange({ workspace_base: v })}
        error={errors.workspace_base}
      />
    </SectionShell>
  );
}
