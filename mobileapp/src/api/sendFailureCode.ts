const codes = [
  'input-awaiting-confirmation', 'input-state-unavailable', 'draft-present',
  'conversation-unavailable', 'target-changed', 'session-not-loaded',
  'input-spool-full', 'outcome-unconfirmed', 'input-unconfirmed',
] as const;

export type SendFailureCode = typeof codes[number];

export function sendFailureCode(value: unknown): SendFailureCode | undefined {
  return typeof value === 'string' && (codes as readonly string[]).includes(value)
    ? value as SendFailureCode : undefined;
}
