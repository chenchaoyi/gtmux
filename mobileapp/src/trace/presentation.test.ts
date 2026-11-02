import { build } from './model';
import {
  traceEventLabel,
  traceEventPreview,
  traceEventIcon,
  conversationOutline,
  traceKindLabels,
  traceDetailLabels,
} from './presentation';
const browser = require('../../../internal/server/web/trace-model.js');
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
