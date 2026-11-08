import { useEffect, useState } from 'react';
import { GtmuxClient, TranscriptStep } from '../api/client';
import { loadToolDetails, toolDetailRefs } from './toolPresentation';

export function useToolInput(
  step?: TranscriptStep,
  client?: GtmuxClient,
  session = '',
  home = '',
) {
  const ref = JSON.stringify(step ? toolDetailRefs(step) : []);
  const needed = ref !== '[]';
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{
    key: string;
    client: GtmuxClient;
    step?: TranscriptStep;
    failed?: boolean;
  } | null>(null);
  const key = JSON.stringify([ref, session, home, attempt]);
  useEffect(() => {
    if (!needed || !client || !session || !step) return;
    const controller = new AbortController();
    setResult(null);
    loadToolDetails(step, value =>
      client.toolInput(session, home, value, controller.signal),
    )
      .then(value => {
        if (!controller.signal.aborted) setResult({ key, client, step: value });
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setResult({ key, client, failed: true });
      });
    return () => controller.abort();
    // Source identity is the immutable ref, not an object recreated by render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, ref, needed, session, home, key]);
  const current =
    result?.key === key && result.client === client ? result : null;
  return {
    step: current?.step ?? step,
    loading: !!(needed && client && session && !current),
    failed: !!current?.failed,
    retry: () => setAttempt(n => n + 1),
  };
}
