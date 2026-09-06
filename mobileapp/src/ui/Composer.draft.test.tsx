import React from 'react';
import {TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Composer} from './Composer';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

// The store's rules are pinned in state/drafts.test.ts. THIS pins the wiring,
// which is the half that was actually missing: a draft that is stored correctly
// and never read back is still a lost draft.

const flush = async () => {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
};

function mount(draftKey?: string, demo = false) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <Composer pal={paletteFor('dark')} lang="en" demo={demo} draftKey={draftKey} onSend={() => {}} />,
    );
  });
  return tree!;
}

// The composer rests as a key row; the field mounts when you tap ⌨. Opening it is
// part of the scenario — a draft is restored into a box you are about to type in.
function openBox(t: renderer.ReactTestRenderer) {
  // Idempotent: the ⌨ key TOGGLES, so a second call would close the field again.
  if (t.root.findAllByType(TextInput).length === 0) {
    act(() => {
      t.root.findByProps({testID: TestIds.composer.keyboard}).props.onPress();
    });
  }
  return t.root.findAllByType(TextInput);
}

beforeEach(() => {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  (AsyncStorage.setItem as jest.Mock).mockClear();
});

test('a stored draft comes back in the pane it was written for', async () => {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(
    JSON.stringify({'%7': {text: 'the migration should target', at: Date.now()}}),
  );
  const t = mount('%7');
  await flush();
  expect(openBox(t)[0].props.value).toBe('the migration should target');
  act(() => t.unmount());
});

test('another pane gets its own box, not this one', async () => {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(
    JSON.stringify({'%7': {text: 'half a thought', at: Date.now()}}),
  );
  const t = mount('%11');
  await flush();
  expect(openBox(t)[0].props.value).toBe('');
  act(() => t.unmount());
});

test('typing before the load lands is not overwritten by it', async () => {
  // The read is async; if it applied unconditionally it would wipe the first
  // characters of whatever you had already started typing.
  let release: (v: string | null) => void = () => {};
  (AsyncStorage.getItem as jest.Mock).mockReturnValue(
    new Promise<string | null>(res => {
      release = res;
    }),
  );
  const t = mount('%7');
  const box = openBox(t)[0];
  act(() => {
    box.props.onChangeText('typed first');
  });
  await act(async () => {
    release(JSON.stringify({'%7': {text: 'the stored one', at: Date.now()}}));
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(openBox(t)[0].props.value).toBe('typed first');
  act(() => t.unmount());
});

// The save is debounced, so a test that only flushes microtasks proves nothing
// about it — this waits the debounce out.
const settleDebounce = async () => {
  await act(async () => {
    await new Promise<void>(r => setTimeout(() => r(), 500));
  });
};

const wroteDrafts = () =>
  (AsyncStorage.setItem as jest.Mock).mock.calls.some(c => c[0] === 'gtmux.drafts');

test('what you type is persisted under this pane once you pause', async () => {
  const t = mount('%7');
  const box = openBox(t)[0];
  act(() => {
    box.props.onChangeText('the migration should target');
  });
  await settleDebounce();
  const call = (AsyncStorage.setItem as jest.Mock).mock.calls.find(c => c[0] === 'gtmux.drafts');
  expect(call).toBeDefined();
  expect(JSON.parse(call![1])['%7'].text).toBe('the migration should target');
  act(() => t.unmount());
});

test('Demo neither reads nor writes real drafts', async () => {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(
    JSON.stringify({'%7': {text: 'a real message to your Mac', at: Date.now()}}),
  );
  const t = mount('%7', true);
  await flush();
  const box = openBox(t)[0];
  expect(box.props.value).toBe('');
  act(() => {
    box.props.onChangeText('demo typing');
  });
  await settleDebounce();
  expect(wroteDrafts()).toBe(false);
  act(() => t.unmount());
});

// History is scoped, and the WIRING is the half that goes wrong: a store that
// files correctly but is read with the wrong scope shows you someone else's list.
test('the history picker shows this scope first, topped up from elsewhere', async () => {
  (AsyncStorage.getItem as jest.Mock).mockImplementation((k: string) =>
    Promise.resolve(
      k === 'gtmux.inputHistory'
        ? JSON.stringify({
            scopes: {gtmux: {list: ['cut release'], at: Date.now()}},
            recent: ['from elsewhere'],
          })
        : null,
    ),
  );
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <Composer
        pal={paletteFor('dark')}
        lang="en"
        historyScope="gtmux"
        onSend={() => {}}
      />,
    );
  });
  await flush();
  act(() => {
    tree!.root.findByProps({testID: TestIds.composer.history}).props.onPress();
  });
  const shown = tree!.root
    .findAll(n => typeof n.props?.children === 'string')
    .map(n => n.props.children as string);
  expect(shown).toContain('cut release');
  expect(shown.indexOf('cut release')).toBeLessThan(shown.indexOf('from elsewhere'));
  act(() => tree!.unmount());
});

test('a different scope does not show the first one’s entries at the top', async () => {
  (AsyncStorage.getItem as jest.Mock).mockImplementation((k: string) =>
    Promise.resolve(
      k === 'gtmux.inputHistory'
        ? JSON.stringify({
            scopes: {gtmux: {list: ['a gtmux-only phrase'], at: Date.now()}},
            recent: [],
          })
        : null,
    ),
  );
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <Composer pal={paletteFor('dark')} lang="en" historyScope="diting-mobile" onSend={() => {}} />,
    );
  });
  await flush();
  act(() => {
    tree!.root.findByProps({testID: TestIds.composer.history}).props.onPress();
  });
  const shown = tree!.root
    .findAll(n => typeof n.props?.children === 'string')
    .map(n => n.props.children as string);
  // The tail is empty, so there is nothing to top up with — and the other
  // project's list is NOT it.
  expect(shown).not.toContain('a gtmux-only phrase');
  act(() => tree!.unmount());
});
