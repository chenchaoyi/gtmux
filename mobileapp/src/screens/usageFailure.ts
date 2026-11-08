import {ApiError, UsageNetworkError, UsageResponseError} from '../api/client';
import {ReadDeadlineError} from '../state/readTask';

// A live Usage report can take 30+ seconds on a Mac with many panes. Keep one
// request alive through that real read (and an occasional limits refresh).
export const USAGE_READ_TIMEOUT_MS = 90_000;

export type UsageFailure = {
  kind: 'timeout' | 'network' | 'auth' | 'owner' | 'server' | 'invalid' | 'unknown';
  detail: string;
};

// Keep transport detail bounded: raw network errors can contain private URLs.
export function usageFailure(error: unknown): UsageFailure {
  if (error instanceof ReadDeadlineError) return {kind: 'timeout', detail: `${error.timeoutMs / 1000}s read deadline`};
  if (error instanceof UsageNetworkError) return {kind: 'network', detail: 'Network request failed'};
  if (error instanceof UsageResponseError) return {kind: 'invalid', detail: 'Invalid usage response'};
  if (error instanceof ApiError) {
    if (error.status === 401) return {kind: 'auth', detail: 'HTTP 401'};
    if (error.status === 403) return {kind: 'owner', detail: 'HTTP 403'};
    return {kind: 'server', detail: `HTTP ${error.status}`};
  }
  return {kind: 'unknown', detail: 'Usage read failed'};
}
