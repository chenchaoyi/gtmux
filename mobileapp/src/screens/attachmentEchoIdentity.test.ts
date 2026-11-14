import {beginChatEcho, chatEchoRecorded} from './pendingChatEcho';
import {promptIdentity} from './promptIdentity';
const turn = (prompt: string, time: string) => ({prompt, time, response: '', session_id: 'synthetic-thread'});
const text = 'Look at this\n/tmp/echo-fixture.png';
const recorded = text + '\n![Image #1](/tmp/echo-fixture.png)\n';

test('AEC-001 dual-representation identity preserves ordered attachment multiplicity', () => {
  const repeated = '/tmp/a.png\n/tmp/a.png';
  const rows = [
    [text, recorded, true],
    ['/tmp/a photo.png', '/tmp/a photo.png\n![Image #1](/tmp/a%20photo.png)', true],
    ['/tmp/a.png\n/tmp/b.png', '/tmp/a.png\n/tmp/b.png\n![Image #1](/tmp/a.png)\n![Image #2](/tmp/b.png)', true],
    [repeated, repeated + '\n![Image #1](/tmp/a.png)\n![Image #2](/tmp/a.png)', true],
    ['/tmp/a.png', '/tmp/a.png\n<image name=[Image #1] path="/tmp/a.png">\n</image>', true],
    ['/tmp/a.png', '/tmp/a.png\n![Image #1](/tmp/b.png)', false],
    ['/tmp/a.png\n/tmp/b.png', '/tmp/a.png\n/tmp/b.png\n![Image #1](/tmp/b.png)\n![Image #2](/tmp/a.png)', false],
    [repeated, '/tmp/a.png\n![Image #1](/tmp/a.png)', false],
    [repeated, repeated + '\n![Image #1](/tmp/a.png)', false],
    [text, recorded.replace('Look at this', 'Different instruction'), false],
  ] as const;
  for (const [sent, wire, same] of rows) {
    expect(promptIdentity(sent) === promptIdentity(wire)).toBe(same);
    const echo = beginChatEcho('new-send', sent, [turn(wire, '2026-11-14T00:00:00Z')]);
    expect(chatEchoRecorded(echo, [turn(wire, '2026-11-14T00:00:00Z')])).toBe(false);
    expect(chatEchoRecorded(echo, [turn(wire, '2026-11-14T00:01:00Z')])).toBe(same);
  }
});
