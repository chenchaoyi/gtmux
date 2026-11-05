import { build } from './model';
import {
  traceEventLabel,
  traceEventPreview,
  traceEventIcon,
  conversationOutline,
  traceKindLabels,
  traceDetailLabels,
  traceCounts,
  traceWindowLabel,
  traceTurnSummary,
  traceDetailMetadata,
} from './presentation';
const browser = require('../../../internal/server/web/trace-model.js');
test('loaded counts and message/call/context summaries agree across renderers at original turn 253', () => {
  const turns = Array.from({ length: 4 }, (_, i) => ({
    session_id: 'synthetic-private-session',
    turn_ordinal: 253 + i,
    prompt: 'Coordination message',
    response: 'Reply',
  }));
  const index = build(turns);
  expect(index.turns).toBe(4);
  expect(browser.build(turns).turns).toBe(4);
  expect(traceWindowLabel(index.events, 'en')).toBe('4 turns · 8 records');
  expect(traceCounts(index.events)).toEqual({
    turns: 4,
    calls: 0,
    messages: 8,
    contexts: 0,
    records: 8,
  });
  const group = index.events.slice(0, 2);
  expect(traceTurnSummary(group, 'en')).toBe('0 calls · 2 messages');
  const mixed = build([
    {
      ...turns[0],
      segments: [
        {
          context: { text: 'Workspace rule', title: 'Workspace instructions' },
        },
        {
          steps: [
            {
              title: 'Read',
              operation: 'read',
              input: '{"path":"example.ts"}',
            },
          ],
        },
        { text: 'Reply' },
      ],
    },
  ]).events;
  expect(traceTurnSummary(mixed, 'en')).toBe(
    '1 call · 2 messages · 1 context record',
  );
  expect(traceTurnSummary(mixed, 'zh')).toBe(
    '1 次调用 · 2 条消息 · 1 条上下文',
  );
  for (const lang of ['en', 'zh'] as const) {
    expect(browser.traceCounts(mixed)).toEqual(traceCounts(mixed));
    expect(browser.traceWindowLabel(index.events, lang)).toBe(
      traceWindowLabel(index.events, lang),
    );
    expect(browser.traceTurnSummary(mixed, lang)).toBe(
      traceTurnSummary(mixed, lang),
    );
    expect(browser.traceDetailMetadata(group[1], lang)).toBe(
      traceDetailMetadata(group[1], lang),
    );
  }
  expect(traceEventLabel(group[1], 'en')).toBe('Agent reply');
  expect(traceDetailMetadata(group[1], 'en')).toBe('Turn 253');
  expect(traceDetailMetadata(group[1], 'en')).not.toContain(
    'synthetic-private-session',
  );
});
test('native and browser share trace labels, summaries, icons, categories and original ordinals', () => {
  const turns: any = [
    {
      session_id: 'synthetic',
      turn_ordinal: 151,
      prompt: 'Question',
      segments: [
        {
          steps: [
            {
              title: 'functions.exec',
              input: 'echo one',
              input_kind: 'command',
            },
            {
              title: 'wait',
              input: '{"duration_ms":1000}',
              input_kind: 'parameters',
            },
          ],
        },
        { text: 'Progress text' },
      ],
    },
  ];
  for (const lang of ['en', 'zh'] as const)
    for (const event of build(turns).events) {
      expect(browser.traceEventLabel(event, lang)).toBe(
        traceEventLabel(event, lang),
      );
      expect(browser.traceEventPreview(event, lang)).toBe(
        traceEventPreview(event, lang),
      );
      expect(browser.traceEventIcon(event)).toBe(traceEventIcon(event));
    }
  expect(browser.traceKindLabels).toEqual(traceKindLabels);
  expect(browser.traceDetailLabels).toEqual(traceDetailLabels);
  expect(browser.conversationOutline(turns)).toEqual(
    conversationOutline(turns),
  );
  expect(conversationOutline(turns)[0].turn).toBe(151);
});

test('outline summaries use the requested language for image-only prompts', () => {
  const turns: any = [
    {
      turn_ordinal: 151,
      prompt: '<image name="sample" path="/tmp/synthetic.png"></image>',
      response: 'Done',
    },
  ];
  expect(conversationOutline(turns, 'zh')[0].prompt).toBe('图片消息');
  expect(browser.conversationOutline(turns, 'zh')).toEqual(
    conversationOutline(turns, 'zh'),
  );
});

test('overview preserves Markdown destinations and bookmark identity across history pages and home scopes', () => {
  const turn: any = {
    record_id: 'record-late',
    session_id: 'session',
    turn_ordinal: 151,
    prompt: '**[Read](https://example.test/' + 'long'.repeat(100) + ')**',
    response: '## Result\n\n- **Complete**',
  };
  const before = conversationOutline([turn], 'en', { home_id: 'home-one' })[0];
  const after = conversationOutline(
    [
      {
        ...turn,
        record_id: 'record-early',
        turn_ordinal: 150,
        prompt: 'Earlier',
      },
      turn,
    ],
    'en',
    { home_id: 'home-one' },
  )[1];
  expect(after.key).toBe(before.key);
  expect(after.bookmarkKey).toBe(before.bookmarkKey);
  expect(after.turn).toBe(151);
  expect(before.promptMarkdown).toBe(turn.prompt);
  expect(before.summaryMarkdown).toBe(turn.response);
  expect(
    conversationOutline([turn], 'en', { home_id: 'home-two' })[0].bookmarkKey,
  ).not.toBe(before.bookmarkKey);
  expect(
    conversationOutline([{ ...turn, response: 'Updated progress' }], 'en', {
      home_id: 'home-one',
    })[0].bookmarkKey,
  ).toBe(before.bookmarkKey);
  expect(
    conversationOutline([{ prompt: 'Unidentified', response: 'Reply' }])[0]
      .bookmarkKey,
  ).toBeUndefined();
  expect(
    browser.conversationOutline([turn], 'en', { home_id: 'home-one' }),
  ).toEqual([before]);
});
