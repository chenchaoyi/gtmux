import type {ParticipantProfile} from '../api/types';
type Process = NonNullable<ParticipantProfile['execution_chain']>[number];
export function executionGroups(chain: Process[]): (Process & {count: number})[] {
 const groups: (Process & {count: number})[] = [];
 for (const process of chain) {
  const previous = groups[groups.length - 1];
  if (previous && previous.name === process.name && previous.path === process.path && JSON.stringify(previous.arguments ?? []) === JSON.stringify(process.arguments ?? [])) previous.count++;
  else groups.push({...process, count:1});
 }
 return groups;
}
