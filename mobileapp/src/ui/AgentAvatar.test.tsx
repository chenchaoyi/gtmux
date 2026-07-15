import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentAvatar} from './AgentAvatar';
import {Agent} from '../api/types';

// Regression: AgentAvatar must render OUTSIDE an AgentsProvider — the pre-pairing
// Demo screen (the App Store reviewer's fallback) reuses the radar row, which
// renders AgentAvatar with no provider. Before the useAgentsOptional fix this
// threw "useAgents must be used within AgentsProvider" and red-screened the Demo.
test('renders without an AgentsProvider (Demo screen path)', () => {
  const agent = {agent: 'Claude Code'} as Agent;
  let tree: renderer.ReactTestRenderer | undefined;
  // The bug was a thrown "useAgents must be used within AgentsProvider".
  expect(() => {
    act(() => {
      tree = renderer.create(
        <AgentAvatar agent={agent} size={40} radius={10} bg="#fff" fg="#000" />,
      );
    });
  }).not.toThrow();
  expect(tree!.toJSON()).toBeTruthy();
});
