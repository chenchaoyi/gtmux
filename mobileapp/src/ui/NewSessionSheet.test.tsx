import React from 'react';
import {Animated, Dimensions, Keyboard, KeyboardAvoidingView, Modal, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NewSessionSheet, forgetKeyboard} from './NewSessionSheet';
import {GtmuxClient, SessionCreateError} from '../api/client';
import {paletteFor} from './theme';

const result = {session: 'project', pane_id: '%8', window: '0', pane: '0', loc: 'project:0.0'};
const mounted: renderer.ReactTestRenderer[] = [];
function mount(lang: 'en' | 'zh' = 'en') {
  const createSession = jest.fn().mockResolvedValue(result);
  const onCreated = jest.fn(); const onClose = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<NewSessionSheet visible client={{createSession} as unknown as GtmuxClient}
    macName="Studio Mac" lang={lang} pal={paletteFor('dark')} onCreated={onCreated} onClose={onClose} onDismiss={() => {}} onCheckSessions={() => {}} />); });
  mounted.push(tree);
  const input = () => tree.root.findByProps({testID: 'new-session-name'});
  const submit = () => tree.root.findByProps({testID: 'new-session-create'});
  const text = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
  return {tree, createSession, onCreated, onClose, input, submit, text};
}
afterEach(() => { for (const t of mounted.splice(0)) act(() => t.unmount()); });

test('identifies the target and creates an automatically named session', async () => {
  const m = mount('zh');
  expect(m.text()).toContain('Studio Mac');
  expect(m.input().props.placeholder).toBe('自动命名');
  await act(async () => { await m.submit().props.onPress(); });
  expect(m.createSession).toHaveBeenCalledWith('', expect.any(String));
  expect(m.onCreated).toHaveBeenCalledWith(result);
});

test('previews normalized names, locks repeated taps and blocks dismissal while creating', async () => {
  const m = mount(); let resolve!: (v: typeof result) => void;
  m.createSession.mockReturnValue(new Promise(r => {resolve = r;}));
  act(() => m.input().props.onChangeText(' project.v1:review '));
  expect(m.text()).toContain('project-v1-review');
  let pending!: Promise<void>;
  act(() => { pending = m.submit().props.onPress(); m.submit().props.onPress(); });
  expect(m.createSession).toHaveBeenCalledTimes(1);
  expect(m.submit().props.accessibilityState.busy).toBe(true);
  act(() => m.tree.root.findByType(Modal).props.onRequestClose());
  expect(m.onClose).not.toHaveBeenCalled();
  await act(async () => {resolve(result); await pending;});
});

test('uncertain creation retries the same request and does not allow changing its name', async () => {
  const m = mount(); m.createSession.mockRejectedValueOnce(new SessionCreateError(0, 'uncertain'));
  act(() => m.input().props.onChangeText('work'));
  await act(async () => {await m.submit().props.onPress();});
  expect(m.text()).toContain('Check sessions');
  expect(m.input().props.editable).toBe(false);
  await act(async () => {await m.submit().props.onPress();});
  expect(m.createSession.mock.calls[1]).toEqual(m.createSession.mock.calls[0]);
});

test('a duplicate name retains the form and accepts a corrected name', async () => {
  const m = mount();m.createSession.mockRejectedValueOnce(new SessionCreateError(409, 'name_exists'));
  act(() => m.input().props.onChangeText('work'));
  await act(async () => {await m.submit().props.onPress();});
  expect(m.input().props.value).toBe('work'); expect(m.input().props.editable).toBe(true);
  expect(m.text()).toContain('already in use');
  act(() => m.input().props.onChangeText('work-2'));
  await act(async () => {await m.submit().props.onPress();});
  expect(m.createSession.mock.calls[1][1]).not.toBe(m.createSession.mock.calls[0][1]);
});

test('an old server gives an update instruction, and unmount suppresses late navigation', async () => {
  const m=mount();m.createSession.mockRejectedValueOnce(new SessionCreateError(404,'unsupported'));
  await act(async () => {await m.submit().props.onPress();});
  expect(m.text()).toContain('Update gtmux'); expect(m.submit().props.disabled).toBe(true);
  const n=mount(); let resolve!: (v: typeof result) => void;
  n.createSession.mockReturnValue(new Promise(r => {resolve=r;}));
  let pending!: Promise<void>;act(() => {pending=n.submit().props.onPress();});
  act(() => n.tree.unmount());
  await act(async () => {resolve(result);await pending;});expect(n.onCreated).not.toHaveBeenCalled();
});

// The keyboard rises with the form, not after it: the name field is focused on the frame
// after mount, before the Modal's fade ends (onShow), so the form moves once, with the
// keyboard, instead of settling and then being pushed up again (2026-10-05).
test('focuses the name on the next frame, not when the fade ends', () => {
  jest.useFakeTimers();
  try {
    const m = mount();
    const focus = (m.input().instance as {focus: jest.Mock}).focus;
    // The mock is shared by every TextInput the file mounted; count from here.
    focus.mockClear();
    expect(focus).not.toHaveBeenCalled(); // the frame has not run yet
    act(() => { jest.advanceTimersByTime(20); }); // one frame
    expect(focus).toHaveBeenCalledTimes(1);
    // And not again when the fade ends: nothing listens to the Modal's onShow any more.
    expect(m.tree.root.findByType(Modal).props.onShow).toBeUndefined();
  } finally {
    jest.useRealTimers();
  }
});

test('says what Create and open will do', () => {
  expect(mount('en').text()).toContain('Starts a new tmux session on this Mac and opens its terminal here, so you can start an agent in it.');
  expect(mount('zh').text()).toContain('在这台 Mac 上新开一个 tmux 会话，并在这里打开它的终端，你可以在里面启动 agent。');
});

// One motion, with the keyboard (%6's recordings of the first fix, 2026-10-05: the form
// still faded in at the bottom, then jumped up in one frame, then the keyboard slid in; on
// the first open after launch it waited at the bottom for 0.6s). On a phone the form is
// lifted from its first frame to where the keyboard will put it, and corrected on the
// keyboard's own timing when the keyboard says its height.
describe('the form moves with the keyboard, once', () => {
  let shown: ((e: {endCoordinates: {height: number}; duration: number}) => void) | undefined;
  beforeEach(() => {
    forgetKeyboard();
    shown = undefined;
    jest.spyOn(Keyboard, 'addListener').mockImplementation(((name: string, fn: any) => {
      if (name === 'keyboardWillShow') shown = fn;
      return {remove: () => {}};
    }) as any);
    jest.spyOn(Animated, 'timing').mockImplementation((() => ({start: () => {}})) as any);
  });
  afterEach(() => jest.restoreAllMocks());
  const lifted = (m: ReturnType<typeof mount>) =>
    (m.tree.root.findByProps({testID: 'new-session-lift'}).props.style as any[]).flat()
      .find((x: any) => x?.transform)?.transform[0].translateY.__getValue();

  test('the first open is already lifted, by a guess, and moves only on the keyboard\'s timing', () => {
    const m = mount();
    expect(lifted(m)).toBe(-Math.round(Dimensions.get('window').height * 0.4));
    act(() => shown!({endCoordinates: {height: 336}, duration: 250}));
    const [, cfg] = (Animated.timing as unknown as jest.Mock).mock.calls.at(-1);
    expect(cfg).toMatchObject({toValue: -336, duration: 250, useNativeDriver: true});
    expect(cfg.easing).toBeDefined();
  });

  test('a later open starts at the height the keyboard said, so nothing moves', () => {
    const m1 = mount();
    act(() => shown!({endCoordinates: {height: 336}, duration: 250}));
    act(() => m1.tree.unmount());
    mounted.splice(mounted.indexOf(m1.tree), 1);
    const m2 = mount();
    expect(lifted(m2)).toBe(-336);
  });

  test('the iPad form keeps the keyboard avoider', () => {
    let tree!: renderer.ReactTestRenderer;
    act(() => { tree = renderer.create(<NewSessionSheet visible layout="regular" client={{} as unknown as GtmuxClient}
      macName="Studio Mac" lang="en" pal={paletteFor('dark')} onCreated={() => {}} onClose={() => {}} onDismiss={() => {}} onCheckSessions={() => {}} />); });
    mounted.push(tree);
    expect(tree.root.findAllByType(KeyboardAvoidingView)).toHaveLength(1);
    expect(tree.root.findAllByProps({testID: 'new-session-lift'})).toHaveLength(0);
  });
});

