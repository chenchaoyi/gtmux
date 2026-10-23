export {};
// eslint-disable-next-line @typescript-eslint/no-var-requires
const fs = require('fs') as {readFileSync: (p: string, encoding: string) => string};
const req = require as unknown as {resolve: (m: string) => string};

test.each(['DetailScreen.tsx', 'HQScreen.tsx'])('%s scopes and remounts the live input target', file => {
  const source = fs.readFileSync(req.resolve('./' + file), 'utf8');
  expect(source).toMatch(/key=\{draftKeyFor\(client.base,/);
  expect(source).toMatch(/draftKey=\{draftKeyFor\(client.base,/);
  expect(source).toMatch(/paneId=\{/);
});
