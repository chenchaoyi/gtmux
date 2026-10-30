// Synthetic, explicit workspace records for the same overview used on a paired Mac.
import type {
  HQMatter,
  HQMatterDetail,
  HQWorkspace,
  MatterState,
} from '../api/hqWorkspace';
export function demoHQWorkspace(
  zh: boolean,
  now = Math.floor(Date.now() / 1000),
  phase: 'waiting' | 'working' | 'done' | 'declined' = 'waiting',
): HQWorkspace {
  const rows: [string, string, string, MatterState, string, string][] = [
    [
      'verify-tests',
      'Approve the verification run',
      '确认验证范围',
      'needs-input',
      'The fix is ready. Run the full suite before merging?',
      '修复已完成，合入前是否运行完整回归？',
    ],
    [
      'api-refactor',
      'API refactor',
      'API 重构',
      'open',
      'Implementation is complete. Checking backward compatibility.',
      '实现已完成，正在验证旧版兼容性。',
    ],
    [
      'trace-reader',
      'Trace reading experience',
      'Trace 阅读体验',
      'open',
      'Testing folded calls and reading position during refresh.',
      '正在验证工具调用折叠和刷新时的阅读位置。',
    ],
    [
      'docs-review',
      'Onboarding guide',
      '入门指南',
      'completed',
      'The guide has been reviewed and published.',
      '指南已核对并发布。',
    ],
  ];
  if (phase !== 'waiting') {
    rows[0][3] =
      phase === 'working' ? 'open' : phase === 'done' ? 'completed' : 'closed';
    rows[0][4] =
      phase === 'working'
        ? 'Running the tests.'
        : phase === 'done'
        ? 'Tests passed.'
        : 'Tests were not run.';
    rows[0][5] =
      phase === 'working'
        ? '正在运行测试。'
        : phase === 'done'
        ? '测试已通过。'
        : '未运行测试。';
    rows[1][3] = phase === 'done' ? 'completed' : 'open';
  }
  const matters: HQMatter[] = rows.map(([id, en, cn, state], i) => ({
    id,
    title: zh ? cn : en,
    state,
    updated_at: now - (i + 1) * 120,
    revision: i + 1,
    participants: [],
  }));
  const item = (m: HQMatter) => {
    const row = rows.find(r => r[0] === m.id)!;
    return {
      matter_id: m.id,
      preview: {
        message_id: `report-${m.id}`,
        kind: 'progress',
        body: row[zh ? 5 : 4],
        at: m.updated_at,
      },
    };
  };
  return {
    schema: 1,
    revision: 4,
    matters,
    overview: {
      generated_at: now,
      since: now - 86400,
      needs_input: matters.filter(m => m.state === 'needs-input').map(item),
      following: matters.filter(m => m.state === 'open').map(item),
      recent: [
        {
          seq: 4,
          at: matters[3].updated_at,
          matter_id: 'docs-review',
          kind: 'hq-result',
          state: 'completed',
        },
      ],
      recent_total: 1,
    },
  };
}
export function demoHQMatter(
  id: string,
  zh: boolean,
  now = Math.floor(Date.now() / 1000),
  phase: 'waiting' | 'working' | 'done' | 'declined' = 'waiting',
): HQMatterDetail {
  const data = demoHQWorkspace(zh, now, phase),
    matter = data.matters.find(m => m.id === id);
  if (!matter) throw new Error('Demo matter not found');
  const preview = [
    ...data.overview.needs_input,
    ...data.overview.following,
  ].find(i => i.matter_id === id)?.preview;
  return {
    schema: 1,
    matter,
    messages: [
      {
        id: `report-${id}`,
        seq: 1,
        speaker: 'hq',
        kind: 'progress',
        body:
          preview?.body ??
          (id === 'verify-tests' && phase === 'declined'
            ? zh
              ? '未运行测试。'
              : 'Tests were not run.'
            : zh
            ? '已完成并交付。'
            : 'Completed and delivered.'),
        at: matter.updated_at,
      },
    ],
    message_earlier_available: false,
    warnings: [],
  };
}
