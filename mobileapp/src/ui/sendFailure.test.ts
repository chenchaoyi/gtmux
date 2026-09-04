import {busyNote, classifySendFailure, failureCopy} from './sendFailure';

// The strings are the ones serve returns (internal/app/serve.go), wrapped by the handler
// as "send failed: <err>".
describe('classifying what the server said', () => {
  test.each([
    ['send failed: not sent: that pane has unsent text in its input box — clear it or send from the Mac', 'draft'],
    ['send failed: pane not found', 'gone'],
    ['send failed: key not allowed', 'key'],
    ["send failed: not confirmed: the pane's input box did not settle on the full message", 'unconfirmed'],
    ['send failed: something nobody has written yet', 'unknown'],
    ['', 'unknown'],
  ])('%s', (reason, want) => {
    expect(classifySendFailure(reason)).toBe(want);
  });
});

describe('what the reader is offered', () => {
  test('the draft case names the cause and offers no override, because there is none', () => {
    // The candidate proposed a "send anyway". POST /api/send has no field to carry one —
    // the core's ClobberDraft is reachable only from the CLI — and adding one is a
    // contract change rather than a display change. So the copy says what is true and
    // what the reader can actually do.
    const c = failureCopy('draft', false);
    expect(c.show).toBe(true);
    expect(c.action).not.toBe('send-anyway');
    expect(c.title).toContain('someone is typing');
    expect(c.title).toMatch(/Mac|again/);
  });

  test('a gone session offers the way out, not a retry that cannot work', () => {
    expect(failureCopy('gone', false).action).toBe('back-to-radar');
  });

  test('a key the server will never run is not the reader’s problem', () => {
    // The app chose that key; a reader can neither cause nor fix it.
    expect(failureCopy('key', false).show).toBe(false);
  });

  test('everything else keeps the retry it always had', () => {
    expect(failureCopy('unconfirmed', false).action).toBe('retry');
    expect(failureCopy('unknown', false).action).toBe('retry');
  });

  test('both languages are written, not fallen back to English', () => {
    for (const kind of ['draft', 'gone', 'unconfirmed', 'unknown'] as const) {
      expect(failureCopy(kind, true).title).toMatch(/[一-龥]/);
      expect(failureCopy(kind, false).title).not.toMatch(/[一-龥]/);
    }
  });
});

describe('a send into a session that is mid-turn', () => {
  test('says what will happen, and only for a working target', () => {
    expect(busyNote('working', false)).toContain('after the current turn');
    expect(busyNote('idle', false)).toBe('');
    expect(busyNote(undefined, false)).toBe('');
  });

  test('it is worded as an expectation, because that is what it is', () => {
    // The server does not report queueing on this path (the phone's send never runs the
    // queued detection). This is read off the target's status, so it must not claim to
    // have observed anything.
    const en = busyNote('working', false);
    expect(en).toContain('will be handled');
    expect(en).not.toMatch(/queued|observed/i);
  });
});
