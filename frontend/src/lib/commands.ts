import { apiPost, ApiError } from './api';
import { pushToast } from './toast';

// sendCommand POSTs to /api/live/:fp/:cmd and surfaces success/error via the
// toast channel. Returns the parsed response on success, throws on failure
// (callers can catch if they need custom handling; otherwise the toast has
// already fired).
export async function sendCommand<TResp = unknown>(
  fp: string,
  cmd: 'pause' | 'resume' | 'hint' | 'clarify' | 'quit',
  body?: unknown,
  labels?: { success?: string; errorPrefix?: string },
): Promise<TResp> {
  try {
    const res = await apiPost<TResp>(`/api/live/${encodeURIComponent(fp)}/${cmd}`, body);
    if (labels?.success) pushToast('success', labels.success);
    return res;
  } catch (e) {
    const prefix = labels?.errorPrefix ?? `${cmd} failed`;
    let detail = '';
    if (e instanceof ApiError) detail = ` (${e.message})`;
    else if (e instanceof Error) detail = `: ${e.message}`;
    pushToast('error', `${prefix}${detail}`);
    throw e;
  }
}
