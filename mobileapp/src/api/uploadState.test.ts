import {GtmuxClient, type UploadState} from './client';

let xhr: FakeXHR;
class FakeXHR {
  status = 0;
  responseText = '';
  upload: {onprogress?: (e: {lengthComputable: boolean; loaded: number; total: number}) => void; onload?: () => void} = {};
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
const client = () => new GtmuxClient('http://fixture.invalid', 'synthetic-token');

test('real byte progress stays distinct from processing and saved completion', async () => {
  const states: UploadState[] = [];
  const fractions: number[] = [];
  let resolved = false;
  const done = client().upload('file:///photo.jpg', 'photo.jpg', 'image/jpeg', Object.assign((f: number) => fractions.push(f), {onState: (s: UploadState) => states.push(s)}), '%fixture').then(r => {resolved = true; return r;});
  expect(states).toEqual([{phase: 'uploading'}]);
  for (const loaded of [37, 62]) xhr.upload.onprogress?.({lengthComputable: true, loaded, total: 100});
  expect(fractions).toEqual([0.37, 0.62]);
  expect(states.at(-1)).toEqual({phase: 'uploading', fraction: 0.62, loaded: 62, total: 100});
  xhr.upload.onload?.();
  await Promise.resolve();
  expect(resolved).toBe(false);
  expect(states.at(-1)?.phase).toBe('processing');
  const before = states.length;
  xhr.upload.onprogress?.({lengthComputable: true, loaded: 99, total: 100});
  expect(states).toHaveLength(before);
  xhr.status = 200; xhr.responseText = JSON.stringify({path: '/saved/photo.jpg'}); xhr.onload?.();
  expect(await done).toEqual({path: '/saved/photo.jpg'});
  expect(states.at(-1)).toEqual({phase: 'complete', fraction: 1});
  xhr.upload.onload?.(); xhr.upload.onprogress?.({lengthComputable: true, loaded: 100, total: 100}); xhr.onload?.();
  expect(states).toHaveLength(before + 1);
  expect(fractions).toEqual([0.37, 0.62]);
});

test('unknown and invalid byte totals stay indeterminate rather than fabricating a percentage', async () => {
  const states: UploadState[] = []; const fractions: number[] = [];
  const done = client().upload('file:///photo.jpg', 'photo.jpg', 'image/jpeg', Object.assign((f: number) => fractions.push(f), {onState: (s: UploadState) => states.push(s)}));
  for (const event of [
    {lengthComputable: false, loaded: 20, total: 100},
    {lengthComputable: true, loaded: 20, total: 0},
    {lengthComputable: true, loaded: NaN, total: 100},
    {lengthComputable: true, loaded: 20, total: Infinity},
  ]) xhr.upload.onprogress?.(event);
  expect(fractions).toEqual([]);
  expect(states.every(s => s.fraction == null)).toBe(true);
  xhr.upload.onload?.();
  expect(states.at(-1)).toEqual({phase: 'processing'});
  xhr.status = 200; xhr.responseText = '{}'; xhr.onload?.();
  expect(await done).toEqual({error: 'failed'});
  expect(states.some(s => s.phase === 'complete')).toBe(false);
});

test('a fully transferred file refused by the server never reports a successful save', async () => {
  const states: UploadState[] = [];
  const done = client().upload('file:///photo.jpg', 'photo.jpg', 'image/jpeg', Object.assign(() => {}, {onState: (s: UploadState) => states.push(s)}));
  xhr.upload.onprogress?.({lengthComputable: true, loaded: 100, total: 100});
  expect(states.at(-1)?.phase).toBe('processing');
  xhr.status = 413; xhr.onload?.();
  expect(await done).toEqual({error: 'too-large'});
  expect(states.some(s => s.phase === 'complete')).toBe(false);
  const before = states.length; xhr.upload.onload?.();
  expect(states).toHaveLength(before);
});
