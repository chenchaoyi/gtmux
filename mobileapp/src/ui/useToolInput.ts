import { useEffect, useState } from 'react';
import { GtmuxClient, TranscriptStep } from '../api/client';

export function useToolInput(
  step?: TranscriptStep,
  client?: GtmuxClient,
  session = '',
  home = '',
) {
  const ref = step?.details_truncated ? step.input_ref : '';
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{
    key: string;
    client: GtmuxClient;
    step?: TranscriptStep;
    failed?: boolean;
  } | null>(null);
  const key = JSON.stringify([ref, session, home, attempt]);
  useEffect(() => {
    if (!ref || !client || !session) return;
    const controller = new AbortController();
    setResult(null);
    client
      .toolInput(session, home, ref, controller.signal)
      .then(value => {
        if (!controller.signal.aborted) setResult({ key, client, step: value });
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setResult({ key, client, failed: true });
      });
    return () => controller.abort();
  }, [client, ref, session, home, key]);
  const current =
    result?.key === key && result.client === client ? result : null;
  return {
    step: current?.step ?? step,
    loading: !!(ref && client && session && !current),
    failed: !!current?.failed,
    retry: () => setAttempt(n => n + 1),
  };
}
