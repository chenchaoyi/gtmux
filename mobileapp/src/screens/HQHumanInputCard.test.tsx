import React from 'react';
import { act, create } from 'react-test-renderer';
import { TouchableOpacity } from 'react-native';
import { HQHumanInputCard } from './HQHumanInputCard';
const matter: any = {
  id: 'goal-id',
  title: 'Sample',
  state: 'needs-input',
  participants: [],
  user_input: {
    id: 'question-id',
    kind: 'decision',
    prompt: 'Keep the sample?',
    why: 'Two options remain.',
    impact: '1 MiB of synthetic files.',
    choices: ['Keep', 'Remove'],
    at: 100,
  },
};
const pal: any = { surface: '#fff', divider: '#ccc', fg: '#111', fg2: '#555' };
test('current question is visible, choices only prepare a reply and impact is disclosed separately', () => {
  const reply = jest.fn();
  let tree: any;
  act(() => {
    tree = create(
      <HQHumanInputCard matter={matter} pal={pal} zh={false} onReply={reply} />,
    );
  });
  expect(JSON.stringify(tree.toJSON())).toContain('Keep the sample?');
  expect(reply).not.toHaveBeenCalled();
  const buttons = tree.root.findAllByType(TouchableOpacity);
  act(() => buttons[1].props.onPress());
  expect(reply).toHaveBeenCalledWith('Keep the sample?\nKeep');
  expect(JSON.stringify(tree.toJSON())).not.toContain('1 MiB');
  act(() => buttons[0].props.onPress());
  expect(JSON.stringify(tree.toJSON())).toContain('1 MiB');
});
test('a saved reply is not requested again and old unstructured records ask HQ to clarify', () => {
  let tree: any;
  act(() => {
    tree = create(
      <HQHumanInputCard
        matter={{
          ...matter,
          user_input: { ...matter.user_input, answer_id: 'answer-id' },
        }}
        pal={pal}
        zh={false}
        onReply={jest.fn()}
      />,
    );
  });
  expect(tree.root.findAllByType(TouchableOpacity)).toHaveLength(0);
  expect(JSON.stringify(tree.toJSON())).toContain('Reply saved');
  act(() =>
    tree.update(
      <HQHumanInputCard
        matter={{ ...matter, user_input: undefined }}
        pal={pal}
        zh={false}
        onReply={jest.fn()}
      />,
    ),
  );
  expect(JSON.stringify(tree.toJSON())).toContain('HQ needs to clarify');
});
