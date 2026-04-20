import { useRoute } from 'preact-iso';
import { RunSummary } from '../components/RunSummary/RunSummary';

export function RunRoute() {
  const { params } = useRoute();
  const fp = params.fp;
  const runId = params.runId;
  if (!fp || !runId) return null;
  return <RunSummary fp={fp} runId={runId} />;
}
