/**
 * @format
 */

import React from 'react';
import ReactTestRenderer from 'react-test-renderer';
import App from '../App';

test('renders correctly', async () => {
  let tree!: ReactTestRenderer.ReactTestRenderer;
  await ReactTestRenderer.act(async () => {
    tree = ReactTestRenderer.create(<App />);
  });
  try {
    expect(tree.toJSON()).not.toBeNull();
  } finally {
    // Root loads asynchronously and may mount the server list's refresh timer.
    // Tear down the whole tree so the smoke test does not keep Jest alive.
    await ReactTestRenderer.act(async () => { tree.unmount(); });
  }
});
