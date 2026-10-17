import type { AccessFilter, AccessGroup } from '../api/accessHistory';
export function mergeAccessGroups(
  prior: AccessGroup[],
  incoming: AccessGroup[],
  sum: boolean,
): AccessGroup[] {
  const result = new Map(prior.map(g => [g.key, g]));
  for (const g of incoming) {
    const old = result.get(g.key);
    const newest =
      !old || new Date(g.last_at) > new Date(old.last_at) ? g : old;
    result.set(
      g.key,
      old && sum
        ? {
            ...newest,
            count: old.count + g.count,
            accepted: old.accepted + g.accepted,
            refused: old.refused + g.refused,
            failed: old.failed + g.failed,
            first_at:
              new Date(old.first_at) < new Date(g.first_at)
                ? old.first_at
                : g.first_at,
          }
        : newest,
    );
  }
  return [...result.values()];
}
export function accessGroupFilter(
  filter: AccessFilter,
  group: AccessGroup,
): AccessFilter {
  return {
    ...filter,
    group_by: undefined,
    actor: group.device_id ? undefined : group.actor,
    device_id: group.device_id,
  };
}
export function accessIdentity(filter: AccessFilter) {
  return filter.device_id
    ? `device:${filter.device_id}`
    : filter.actor
    ? `actor:${filter.actor}`
    : '';
}
