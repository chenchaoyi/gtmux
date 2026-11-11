import {abortAtDeadline} from './requestAbort';

export type SendDelivery = 'queued' | 'accepted' | 'submitted';

export const SEND_TIMEOUT_MS = 20_000;

export class SendDeadlineError extends Error {
  constructor() {
    super('Send confirmation timed out. Check the conversation before retrying; the original send ID is retained.');
    this.name = 'SendDeadlineError';
  }
}

// Bound the entire mutation response, including body parsing. Never retry here:
// cancellation cannot prove whether the server accepted this exact message.
export async function withSendDeadline<T>(send: (signal: AbortSignal) => Promise<T>): Promise<T> {
  const controller = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<never>((_, reject) => {
    timer = setTimeout(() => {
      abortAtDeadline(controller, SEND_TIMEOUT_MS);
      reject(new SendDeadlineError());
    }, SEND_TIMEOUT_MS);
  });
  try {
    return await Promise.race([Promise.resolve().then(() => send(controller.signal)), deadline]);
  } finally {
    clearTimeout(timer);
  }
}
