import {GtmuxClient, type UploadState} from './client';

let xhr: FakeXHR;
class FakeXHR {
  status = 200;
  responseText = '';
  upload: {onload?: () => void; onprogress?: (e: unknown) => void} = {};
  onload?: () => void;
  onerror?: () => void;
  onabort?: () => void;
  ontimeout?: () => void;
  constructor() {xhr = this;}
  open() {}
  setRequestHeader() {}
  send() {}
  abort() {this.onabort?.();}
}
let original: typeof XMLHttpRequest;
beforeEach(() => {original = globalThis.XMLHttpRequest; (globalThis as any).XMLHttpRequest = FakeXHR;});
afterEach(() => {xhr?.abort(); globalThis.XMLHttpRequest = original;});
const upload = (states: UploadState[]) => new GtmuxClient('https://fixture.invalid', 'synthetic-token').upload(
  'file:///photo.jpg', 'photo.jpg', 'image/jpeg', Object.assign(() => {}, {onState: (state: UploadState) => states.push(state)}),
);

test.each(['', ' \t\r\n'])('HTTP success with a blank upload path fails without a completion mark: %j', async path => {
  const states: UploadState[] = [];
  const done = upload(states);
  xhr.upload.onload?.();
  xhr.responseText = JSON.stringify({path}); xhr.onload?.();
  expect(await done).toEqual({error: 'failed'});
  expect(states.some(s => s.phase === 'complete')).toBe(false);
  const count = states.length;
  xhr.responseText = JSON.stringify({path: '/late/path.jpg'}); xhr.onload?.();
  expect(states).toHaveLength(count);
});

test('a nonblank saved upload path retains ordinary spaces exactly', async () => {
  const states: UploadState[] = [];
  const done = upload(states);
  const path = '/saved/a photo.jpg';
  xhr.responseText = JSON.stringify({path}); xhr.onload?.();
  expect(await done).toEqual({path});
  expect(states.at(-1)).toEqual({phase: 'complete', fraction: 1});
});
