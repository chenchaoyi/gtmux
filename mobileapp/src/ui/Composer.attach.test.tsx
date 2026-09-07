import React from 'react';
import {Image} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import {Composer} from './Composer';
import {ImageMarkup} from './ImageMarkup';
import {paletteFor} from './theme';

// A picked photo opens the markup editor. That is the operator's call, made after one
// release of the opposite (2026-09-07): they annotate most of what they send, so a
// second tap to reach the editor would cost them one on nearly every photo to save one
// on the rare bare screenshot.
//
// The editor is ALSO reachable from an already-staged thumbnail, which is what that
// release left behind and what makes a photo re-annotatable.

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

/** Tap the first staged thumbnail to re-open the editor on it. */
const annotate = (t: renderer.ReactTestRenderer) =>
  t.root.findAll(n => String(n.props?.accessibilityLabel ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function')[0].props.onPress();

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

// The STAGED strip only. A bare Image search also catches the markup editor's own
// preview of the photo being edited, which is not an attachment and made "nothing is
// staged yet" read as one.
// The STAGED strip only, one entry per attachment. A bare Image search also catches the
// markup editor's own preview of the photo being edited, which is not an attachment; and
// `findAll` matches every wrapper layer of the same element, so the labels are deduped.
const thumbs = (t: renderer.ReactTestRenderer) => {
  const byLabel = new Map<string, renderer.ReactTestInstance>();
  for (const n of t.root.findAll(n => String(n.props?.accessibilityLabel ?? '').startsWith('attach-annotate-'))) {
    if (!byLabel.has(n.props.accessibilityLabel)) byLabel.set(n.props.accessibilityLabel, n);
  }
  return [...byLabel.values()].map(n => n.findAllByType(Image)[0]);
};

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
  it('opens the editor on the picked photo, before anything is staged', async () => {
    const t = mount();
    await chooseLibrary(t);
    const m = t.root.findByType(ImageMarkup);
    expect(m.props.visible).toBe(true);
    expect(m.props.uri).toBe(picked.uri);
    expect(thumbs(t)).toHaveLength(0); // nothing staged until the edit is done
  });

  it('stages the edited image when the editor finishes', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual(['file:///tmp/marked.png']);
  });

  it('stages nothing if you back out of the editor', async () => {
    // Cancelling means you did not want the photo. Staging it anyway would leave an
    // attachment behind that the operator has to notice and remove.
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onCancel());
    expect(thumbs(t)).toHaveLength(0);
  });

  it('uploads what the editor produced', async () => {
    // The editor is in the path, so the editor names the file. That is a consequence of
    // the flow, recorded here so a future change to either notices the other.
    const seen: {name: string; type: string}[] = [];
    const t = mount({
      onUpload: async (_uri: string, name: string, type: string) => {
        seen.push({name, type});
        return '/tmp/on-the-mac.png';
      },
    });
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    await act(async () => {
      t.root.findAllByProps({testID: 'composer-send'})[0].props.onPress();
    });
    await flush();
    expect(seen).toEqual([{name: 'markup.png', type: 'image/png'}]);
  });

  it('re-opens the editor when you tap an already-staged thumbnail', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    act(() => annotate(t));
    const m = t.root.findByType(ImageMarkup);
    expect(m.props.visible).toBe(true);
    expect(m.props.uri).toBe('file:///tmp/marked.png');
  });

  it('REPLACES a re-edited photo rather than staging a second copy of it', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    act(() => annotate(t));
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked-again.png'));
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual(['file:///tmp/marked-again.png']);
  });

  it('leaves a staged photo alone if you back out of re-editing it', async () => {
    const t = mount();
    await chooseLibrary(t);
    act(() => t.root.findByType(ImageMarkup).props.onDone('file:///tmp/marked.png'));
    act(() => annotate(t));
    act(() => t.root.findByType(ImageMarkup).props.onCancel());
    expect(thumbs(t).map(n => n.props.source.uri)).toEqual(['file:///tmp/marked.png']);
  });
});
