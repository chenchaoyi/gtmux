import {kindsList} from './AppContext';

describe('kindsList', () => {
  it('lists the enabled kinds', () => {
    expect(kindsList({waiting: true, done: true})).toEqual(['waiting', 'done']);
    expect(kindsList({waiting: true, done: false})).toEqual(['waiting']);
    expect(kindsList({waiting: false, done: true})).toEqual(['done']);
  });
  it('sends a no-match sentinel when BOTH are off (so empty != "all")', () => {
    // an empty Kinds list means "all" on the server, so both-off must NOT be [];
    // ['none'] matches no real kind → the device gets no pushes.
    expect(kindsList({waiting: false, done: false})).toEqual(['none']);
  });
});
