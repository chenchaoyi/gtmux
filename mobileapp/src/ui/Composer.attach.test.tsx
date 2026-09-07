import React from 'react';
import {Image} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import {Composer} from './Composer';
import {ImageMarkup} from './ImageMarkup';
import {paletteFor} from './theme';

// Attaching a photo used to cost four full-screen surfaces: the sheet, the system
// picker, the markup editor, and back to the composer — for a screenshot you had already
// taken and did not want to draw on. The editor now sits ON the staged thumbnail, where
// it is one tap and optional (user report, 2026-09-07).

jest.mock('react-native-image-picker', () => ({
  launchImageLibrary: jest.fn(),
  launchCamera: jest.fn(),
}));

const picked = {uri: 'file:///tmp/IMG_0042.HEIC', fileName: 'IMG_0042.HEIC', type: 'image/heic'};

// Every tree is tracked and unmounted after the test. The send path is async, and a
// tree left running finishes its work inside the NEXT test.
const live: renderer.ReactTestRenderer[] = [];
const mount = (extra: Record<string, unknown> = {}) => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(<Composer pal={paletteFor('dark')} lang="en" onSend={() => {}} {...extra} />);
  });
  live.push(tree!);
  return tree!;
};

const flush = async () => {
  // The send path is a chain of awaits (upload → send → clear). Two microtasks is not
  // the whole chain, and a tree still finishing its work runs inside the NEXT test.
  await act(async () => {
    for (let i = 0; i < 12; i++) await Promise.resolve();
    await new Promise<void>(r => setTimeout(() => r(), 0));
  });
};

// Choosing "Photo Library" from the sheet: the sheet defers the action to its dismissal.
const chooseLibrary = async (t: renderer.ReactTestRenderer) => {
  // The composer rests as a key row; ⌨ mounts the field and the + beside it.
  act(() => t.root.findAllByProps({testID: 'composer-kbd'})[0].props.onPress());
  act(() => t.root.findAllByProps({testID: 'composer-attach'})[0].props.onPress());
  act(() => t.root.findAll(n => n.props?.accessibilityLabel === 'attach-0')[0].props.onPress());
  const sheet = t.root.findAll(n => n.props?.onDismiss != null && n.props?.animationType != null)[0];
  // BRACES, and awaited: onDismiss runs the deferred `pickPhoto`, which is async, so a
  // brace-less arrow hands its Promise back to `act` — which then treats the whole call
  // as an async act nobody awaited, and React's act queue never recovers.
  await act(async () => {
    sheet.props.onDismiss();
  });
  await flush();
};

const thumbs = (t: renderer.ReactTestRenderer) =>
  t.root.findAllByType(Image).filter(n => typeof n.props.source?.uri === 'string');

beforeEach(() => {
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: [picked]});
});

afterEach(async () => {
  // AWAITED, and that matters: an unawaited `act` alongside the async one in `flush`
  // interleaves their scopes, and React's act queue comes out of it broken — every tree
  // mounted by a later test rendered EMPTY, which reads as a mysterious cross-test
  // failure rather than as the harness bug it is.
  await act(async () => {
    while (live.length) live.pop()!.unmount();
  });
});

describe('attaching a photo', () => {
  it('stages the photo itself, without stopping in the editor', async () => {
    const t = mount();
    await chooseLibrary(t);
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual([picked.uri]);
    expect(t.root.findByType(ImageMarkup).props.visible).toBe(false);
  });

  it('uploads it under the picker’s own name, not markup.png', async () => {
    // Every photo used to arrive on the Mac called markup.png, whether or not a mark was
    // ever made — the editor was in the path, so the editor named the file.
    const seen: {name: string; type: string}[] = [];
    const t = mount({
      onUpload: async (_uri: string, name: string, type: string) => {
        seen.push({name, type});
        return '/tmp/on-the-mac.heic';
      },
    });
    await chooseLibrary(t);
    await act(async () => {
      t.root.findAllByProps({testID: 'composer-send'})[0].props.onPress();
    });
    await flush();
    expect(seen).toEqual([{name: 'IMG_0042.HEIC', type: 'image/heic'}]);
  });

  it('opens the editor when you tap the thumbnail, on that image', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findAll(n => String(n.props?.accessibilityLabel ?? '').startsWith('attach-annotate-'))[0].props.onPress());
    const m = t.root.findByType(ImageMarkup);
    expect(m.props.visible).toBe(true);
    expect(m.props.uri).toBe(picked.uri);
  });

  it('REPLACES the staged photo when the edit is done, so it is not sent twice', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findAll(n => String(n.props?.accessibilityLabel ?? '').startsWith('attach-annotate-'))[0].props.onPress());
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual(['file:///tmp/marked.png']);
  });

  it('leaves the photo alone if you back out of the editor', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findAll(n => String(n.props?.accessibilityLabel ?? '').startsWith('attach-annotate-'))[0].props.onPress());
    act(() => t.root.findByType(ImageMarkup).props.onCancel());
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual([picked.uri]);
  });
});
