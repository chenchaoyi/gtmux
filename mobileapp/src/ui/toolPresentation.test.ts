import {
  callSections,
  callRaw,
  callTitle,
  callFacts,
  callPage,
  loadToolDetails,
} from './toolPresentation';
import type { TranscriptStep } from '../api/client';
const query: TranscriptStep = {
  kind: 'tool',
  title: 'github.fetch_commit_workflow_runs',
  operation: 'tool',
  call: {
    provider: 'GitHub',
    name: 'github.fetch_commit_workflow_runs',
    status: 'completed',
    duration_ms: 1250,
    has_arguments: true,
    has_result: true,
    loaded: true,
    arguments: '{"repo_full_name":"example/project","commit_sha":"abc123"}',
    result:
      '{"structuredContent":{"workflow_runs":[{"status":"success"}]},"isError":false}',
  },
};
test('query details show named parameters, structured results, provenance and complete raw', () => {
  expect(callTitle(query)).toBe('GitHub · Fetch commit workflow runs');
  const sections = callSections(query, 'en');
  expect(sections.find(s => s.id === 'arguments')?.rows).toEqual([
    { label: 'repo full name', value: 'example/project' },
    { label: 'commit sha', value: 'abc123' },
  ]);
  expect(sections.find(s => s.id === 'result')?.source).toContain('success');
  expect(callFacts(query, 'zh')).toContainEqual({
    label: '耗时',
    value: '1.25 s',
  });
  expect(JSON.parse(callRaw(query))).toMatchObject({
    tool: query.title,
    arguments: { commit_sha: 'abc123' },
    result: { structuredContent: { workflow_runs: [{ status: 'success' }] } },
  });
});
test('command, file changes, waits, text/errors and missing/empty results stay distinct', () => {
  const command = '  printf "hello"\n  # exact';
  const step = {
    ...query,
    input: command,
    input_kind: 'command',
    operation: 'command',
    call: { ...query.call, name: 'exec_command', result: '"hello"' },
  };
  expect(callSections(step, 'en')[0]).toMatchObject({
    label: 'Command',
    source: command,
    hint: 'shell',
  });
  const argv = '["bash","-lc","echo hello"]';
  expect(
    callSections({ ...step, input: argv, input_kind: 'arguments' }, 'en')[0],
  ).toMatchObject({ label: 'Command arguments', source: argv, hint: 'json' });
  expect(
    callSections(
      {
        kind: 'tool',
        title: 'apply_patch',
        input: '{"changes":{"sample.go":{"unified_diff":"-old\\n+new"}}}',
        input_kind: 'changes',
      },
      'en',
    )[0].label,
  ).toBe('File changes');
  expect(
    callSections(
      {
        kind: 'tool',
        title: 'wait',
        input: '{"duration_ms":1000}',
        input_kind: 'parameters',
      },
      'zh',
    )[0].rows?.[0].value,
  ).toBe('1000');
  expect(
    callSections(
      {
        ...query,
        call: {
          ...query.call,
          result:
            '{"content":[{"type":"text","text":"failure detail"}],"isError":true}',
        },
      },
      'en',
    )
      .map(s => s.source)
      .join(' '),
  ).toContain('failure detail');
  expect(
    callSections(
      { ...query, call: { ...query.call, result: 'null' } },
      'en',
    ).find(s => s.id === 'empty')?.source,
  ).toContain('no content');
  expect(
    callSections(
      { ...query, call: { ...query.call, has_result: false } },
      'en',
    ).find(s => s.id === 'unrecorded'),
  ).toBeTruthy();
  expect(
    callSections(
      { ...query, call: { ...query.call, loaded: false } },
      'en',
    ).find(s => s.id === 'pending'),
  ).toBeTruthy();
  expect(
    callSections({ kind: 'tool', title: 'unknown' }, 'en')[0].source,
  ).toContain('No detailed input');
});
test('full reads preserve original command and distinct native result references', async () => {
  const original: TranscriptStep = {
    kind: 'tool',
    title: 'exec',
    input: 'preview',
    details_truncated: true,
    input_ref: 'original',
    call: { ref: 'native', has_result: true },
  };
  const read = jest.fn(
    async (ref: string): Promise<TranscriptStep> =>
      ref === 'original'
        ? { ...original, input: '  exact\n', details_truncated: false }
        : {
            kind: 'tool',
            title: 'exec',
            input: 'different native command',
            input_ref: 'native',
            call: {
              ref: 'native',
              has_result: true,
              loaded: true,
              result: '"ok"',
            },
          },
  );
  const loaded = await loadToolDetails(original, read);
  expect(read).toHaveBeenCalledTimes(2);
  expect(loaded.input).toBe('  exact\n');
  expect(loaded.call?.result).toBe('"ok"');
  await expect(
    loadToolDetails(original, async () => ({
      kind: 'tool',
      title: 'exec',
      input_ref: 'wrong',
    })),
  ).rejects.toThrow('Incomplete');
});
test('large rendering pages disclose bounds and preserve all characters', () => {
  const source = 'x'.repeat(15999) + '🙂' + '尾'.repeat(35000);
  let joined = '';
  const pages = callPage(source, 0).pages;
  for (let p = 0; p < pages; p++) {
    const view = callPage(source, p);
    expect(view.source.length).toBeLessThanOrEqual(16001);
    joined += view.source;
  }
  expect(joined).toBe(source);
  expect(
    callRaw({
      ...query,
      call: { ...query.call, result: JSON.stringify(source) },
    }),
  ).toContain(source);
});
