import React from 'react';
import {Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SendFailedBar} from './SendFailedBar';
import {paletteFor} from './theme';

// One bar, one place on screen, a different next move per refusal. Before this the three
// refusals all read "the input box didn't confirm", which sends the reader to the Mac for
// a session that is gone and offers a retry for a pane someone else is typing in.
const render = (reason: string, handlers: Record<string, () => void> = {}) => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <SendFailedBar
        text="继续"
        reason={reason}
        pal={paletteFor('dark')}
        lang="en"
        onRetry={handlers.onRetry ?? (() => {})}
        onBackToRadar={handlers.onBackToRadar}
        onDismiss={() => {}}
      />,
    );
  });
  return tree!;
};

const words = (t: renderer.ReactTestRenderer): string =>
  t.root
    .findAllByType(Text)
    .flatMap(n => ([] as unknown[]).concat(n.props.children as unknown[]))
    .filter(c => typeof c === 'string')
    .join(' ');

test('a pane someone is typing in says so, and does not pretend an override exists', () => {
  const t = render('send failed: not sent: that pane has unsent text in its input box');
  expect(words(t)).toContain('someone is typing');
  expect(t.root.findAllByProps({testID: 'send-failed-send-anyway'})).toHaveLength(0);
});

test('a session that is gone offers the way out rather than a retry that cannot work', () => {
  let back = 0;
  const t = render('send failed: pane not found', {onBackToRadar: () => (back += 1)});
  expect(words(t)).toContain('that session is gone');
  const btn = t.root.findByProps({testID: 'send-failed-back-to-radar'});
  act(() => {
    btn.props.onPress();
  });
  expect(back).toBe(1);
});

test('a key the app chose and the server refuses shows nothing at all', () => {
  // The reader can neither cause it nor fix it. It belongs in the log.
  const t = render('send failed: key not allowed');
  expect(t.root.findAllByType(TouchableOpacity)).toHaveLength(0);
  expect(t.toJSON()).toBeNull();
});

test('an unrecognised refusal still keeps the text and the retry', () => {
  let retried = 0;
  const t = render('send failed: something new', {onRetry: () => (retried += 1)});
  expect(words(t)).toContain('继续'); // what is being held, so retry is checkable
  act(() => {
    t.root.findByProps({testID: 'send-failed-retry'}).props.onPress();
  });
  expect(retried).toBe(1);
});
