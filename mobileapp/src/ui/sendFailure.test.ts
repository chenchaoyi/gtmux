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
    expect(c.title).toMatch(/someone is typing/i);
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

describe('a Mac that refuses this phone', () => {
  // 401/403 is the Mac refusing THIS PHONE: every retry is refused the same way, so the
  // bar sends the reader to pair again and says the text is back in the box.
  it('is its own kind, read off the status, whatever the body says', () => {
    expect(classifySendFailure('unauthorized', 401)).toBe('refused');
    expect(classifySendFailure('send failed: pane not found', 401)).toBe('refused');
    // 403 is a good token turned away from this pane: not a refusal of the phone.
    expect(classifySendFailure('input not shared for this pane', 403)).toBe('not-shared');
    expect(classifySendFailure('unauthorized')).toBe('unknown'); // no status: as before
    expect(classifySendFailure('send failed: pane not found', 500)).toBe('gone');
  });
  it('offers pairing again, never a retry, in both languages', () => {
    for (const zh of [false, true]) {
      const c = failureCopy('refused', zh);
      expect(c.action).toBe('pair-again');
      expect(c.show).toBe(true);
      expect(c.title).toMatch(zh ? /重新配对/ : /pair again/);
      expect(c.title).toMatch(zh ? /输入框/ : /back in the box/);
    }
  });
});

describe('a pane not open to this connection for typing (403)', () => {
  it('says so, offers neither a retry nor pairing again, and keeps the text', () => {
    for (const zh of [false, true]) {
      const c = failureCopy('not-shared', zh);
      expect(c.action).toBe('none');
      expect(c.show).toBe(true);
      expect(c.title).toMatch(zh ? /没有对这个连接开放输入/ : /not shared with this connection/);
      expect(c.title).not.toMatch(zh ? /配对/ : /pair again/);
    }
  });
});
