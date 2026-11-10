// One token per explicit send, retained across manual Retry. This is an
// idempotency key, not an authentication or authorization credential.
export function newSendId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}
