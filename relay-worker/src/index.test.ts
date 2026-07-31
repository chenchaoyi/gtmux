// Golden-payload test for the LIVE push relay (the Cloudflare Worker — the artifact
// that actually ships; the Go relay/ is a never-deployed reference). Pins the APNs
// payload + headers SHAPE that must stay in sync with relay/apns.go and the iOS app's
// expectations (aps.category quick-reply, mutable-content NSE badge, silent
// content-available badge sync, apns-collapse-id, and the top-level server routing
// field). A shape edit that diverges fails here instead of silently in production.
//
// Zero deps: runs on node's built-in test runner with type-stripping —
//   node --experimental-strip-types --test src/index.test.ts
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {buildApnsRequest, buildLiveActivityAps} from './index.ts';

const JWT = 'jwt-abc';
const TOPIC = 'com.gtmux.app';

test('waiting alert → quick-reply category + mutable-content + routing fields', () => {
  const {payload, headers} = buildApnsRequest(
    {token: 't', kind: 'waiting', title: 'Claude Code', body: 'needs you', subtitle: 'ccy MBP', pane: '%4'},
    JWT,
    TOPIC,
  );
  assert.deepEqual(JSON.parse(payload), {
    aps: {
      alert: {title: 'Claude Code', body: 'needs you', subtitle: 'ccy MBP'},
      sound: 'default',
      'mutable-content': 1,
      category: 'AGENT_WAITING', // drives the 1/2/3 quick-reply actions
    },
    pane: '%4',
    kind: 'waiting',
    server: 'ccy MBP', // top-level (tap-routable) copy of the subtitle
  });
  assert.equal(headers['apns-push-type'], 'alert');
  assert.equal(headers['apns-topic'], TOPIC);
  assert.equal(headers.authorization, 'bearer ' + JWT);
  assert.equal(headers['apns-priority'], undefined); // alerts don't force a priority
  assert.equal(headers['apns-collapse-id'], undefined);
});

test('non-waiting alert (done) → NO quick-reply category', () => {
  const {payload} = buildApnsRequest({token: 't', kind: 'done', title: 'x', body: 'y'}, JWT, TOPIC);
  const aps = JSON.parse(payload).aps as Record<string, unknown>;
  assert.equal(aps.category, undefined);
  assert.equal(aps['mutable-content'], 1); // still wakes the NSE for the status badge
});

test('silent badge-sync → content-available only, background priority, no alert', () => {
  const {payload, headers} = buildApnsRequest(
    {token: 't', silent: true, badge: 3, collapseId: '%4'},
    JWT,
    TOPIC,
  );
  const body = JSON.parse(payload);
  assert.deepEqual(body.aps, {'content-available': 1, badge: 3});
  assert.equal(body.aps.alert, undefined);
  assert.equal(body.aps.sound, undefined);
  assert.equal(headers['apns-push-type'], 'background');
  assert.equal(headers['apns-priority'], '5'); // silent must be low-priority or APNs throttles
  assert.equal(headers['apns-collapse-id'], '%4');
});

test('badge rides along on an alert push too', () => {
  const {payload} = buildApnsRequest({token: 't', kind: 'waiting', badge: 2}, JWT, TOPIC);
  assert.equal(JSON.parse(payload).aps.badge, 2);
});

test('empty optionals default cleanly (no undefined leaks in the wire body)', () => {
  const {payload} = buildApnsRequest({token: 't'}, JWT, TOPIC);
  assert.deepEqual(JSON.parse(payload), {
    aps: {alert: {title: '', body: ''}, sound: 'default', 'mutable-content': 1},
    pane: '',
    kind: '',
    server: '',
  });
});

test('live activity → timestamp + event + content-state + stale-date', () => {
  const aps = buildLiveActivityAps(
    {token: 'act', liveActivity: true, event: 'update', contentState: {waiting: 2}, staleDate: 1_700_002_400},
    1_700_000_000,
  );
  assert.deepEqual(aps, {
    timestamp: 1_700_000_000,
    event: 'update',
    'content-state': {waiting: 2},
    'stale-date': 1_700_002_400,
  });
});

test('live activity without a stale-date omits the key (no undefined leak)', () => {
  const aps = buildLiveActivityAps({token: 'act', liveActivity: true, contentState: {}}, 1_700_000_000);
  assert.deepEqual(aps, {timestamp: 1_700_000_000, event: 'update', 'content-state': {}});
  assert.equal('stale-date' in aps, false);
});
