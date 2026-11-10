import {sendReceiptLabel} from './sendReceipt';

test('MSD-005 receipt wording distinguishes queueing and acceptance without claiming execution', () => {
  expect(sendReceiptLabel('queued', true, false)).toBe('Queued · waiting for the next turn');
  expect(sendReceiptLabel('queued', true, true)).toBe('已排队，等待下一轮处理');
  expect(sendReceiptLabel('accepted', true, false)).toBe('Received · awaiting conversation confirmation');
  expect(sendReceiptLabel('submitted', true, true)).toBe('已提交，等待对话确认');
  expect(sendReceiptLabel(undefined, true, false)).toBe('Sent');
  expect(sendReceiptLabel('queued', false, true)).toBe('正在发送…');
});
