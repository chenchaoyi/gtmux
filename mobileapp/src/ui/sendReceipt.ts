import type {SendDelivery} from '../api/sendDeadline';
export type {SendDelivery} from '../api/sendDeadline';

// A receipt is an ingress observation, not proof of model execution or completion.
export function sendReceiptLabel(delivery: SendDelivery | undefined, delivered: boolean, zh: boolean): string {
  if (!delivered) return zh ? '正在发送…' : 'Sending…';
  if (delivery === 'queued') return zh ? '已排队，等待下一轮处理' : 'Queued · waiting for the next turn';
  if (delivery === 'accepted') return zh ? '已接收，等待对话确认' : 'Received · awaiting conversation confirmation';
  if (delivery === 'submitted') return zh ? '已提交，等待对话确认' : 'Submitted · awaiting conversation confirmation';
  return zh ? '已发送' : 'Sent';
}
