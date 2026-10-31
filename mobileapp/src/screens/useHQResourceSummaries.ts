import { useEffect, useRef, useState } from 'react';
import {
  GtmuxClient,
  HQBoard,
  KnowledgeIndex,
  UsageReport,
  isAuthError,
} from '../api/client';
import { createReadTask, startReadPolling } from '../state/readTask';

export type ResourceReading<T> = { value?: T; failed?: boolean };
export type HQResourceReadings = {
  board: ResourceReading<HQBoard>;
  knowledge: ResourceReading<KnowledgeIndex>;
  usage: ResourceReading<UsageReport | null>;
};
const empty = (): HQResourceReadings => ({
  board: {},
  knowledge: {},
  usage: {},
});

// Independent, owner-only read lanes: a slow usage request never holds up the
// other summaries. Never poll behind the console or publish a retired identity.
export function useHQResourceSummaries(
  client: GtmuxClient,
  active: boolean,
  onUnauthorized: () => void,
) {
  const [record, setRecord] = useState<{
    client: GtmuxClient;
    value: HQResourceReadings;
  } | null>(null);
  const refresh = useRef(() => {});
  useEffect(() => {
    refresh.current = () => {};
    if (!active) {
      setRecord(null);
      return;
    }
    let retired = false;
    const update = <K extends keyof HQResourceReadings>(
      key: K,
      value: HQResourceReadings[K],
    ) => {
      if (!retired)
        setRecord(old => ({
          client,
          value: {
            ...(old?.client === client ? old.value : empty()),
            [key]: value,
          },
        }));
    };
    const failed = (key: keyof HQResourceReadings, error: unknown) => {
      if (retired) return;
      if (isAuthError(error)) {
        retired = true;
        tasks.forEach(task => task.dispose());
        setRecord(null);
        onUnauthorized();
      } else
        setRecord(old => {
          const value = old?.client === client ? old.value : empty();
          return {
            client,
            value: { ...value, [key]: { ...value[key], failed: true } },
          };
        });
    };
    const tasks = [
      createReadTask(
        signal => client.hqBoard(signal),
        value => update('board', { value }),
        { onError: error => failed('board', error) },
      ),
      createReadTask(
        signal => client.hqKnowledge(signal),
        value => update('knowledge', { value }),
        { onError: error => failed('knowledge', error) },
      ),
      createReadTask(
        signal => client.usage(signal),
        value => update('usage', { value }),
        { onError: error => failed('usage', error) },
      ),
    ];
    refresh.current = () => tasks.forEach(task => task.run());
    const stops = tasks.map(task => startReadPolling(task, 30_000));
    return () => {
      retired = true;
      refresh.current = () => {};
      stops.forEach(stop => stop());
    };
  }, [client, active, onUnauthorized]);
  return {
    readings: active && record?.client === client ? record.value : empty(),
    refresh,
  };
}
