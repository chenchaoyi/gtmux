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

test('overview preserves Markdown destinations and bookmark identity across history pages and home scopes',()=>{
 const turn:any={record_id:'record-late',session_id:'session',turn_ordinal:151,prompt:'**[Read](https://example.test/'+ 'long'.repeat(100)+')**',response:'## Result\n\n- **Complete**'};
 const before=conversationOutline([turn],'en',{home_id:'home-one'})[0];
 const after=conversationOutline([{...turn,record_id:'record-early',turn_ordinal:150,prompt:'Earlier'},turn],'en',{home_id:'home-one'})[1];
 expect(after.key).toBe(before.key);expect(after.bookmarkKey).toBe(before.bookmarkKey);expect(after.turn).toBe(151);expect(before.promptMarkdown).toBe(turn.prompt);expect(before.summaryMarkdown).toBe(turn.response);
 expect(conversationOutline([turn],'en',{home_id:'home-two'})[0].bookmarkKey).not.toBe(before.bookmarkKey);
 expect(conversationOutline([{...turn,response:'Updated progress'}],'en',{home_id:'home-one'})[0].bookmarkKey).toBe(before.bookmarkKey);
 expect(conversationOutline([{prompt:'Unidentified',response:'Reply'}])[0].bookmarkKey).toBeUndefined();
 expect(browser.conversationOutline([turn],'en',{home_id:'home-one'})).toEqual([before]);
});
