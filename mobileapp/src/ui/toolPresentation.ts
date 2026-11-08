import type { TranscriptStep } from '../api/client';

// Canonical Chat/Trace interpretation, also generated for the browser. These
// fields describe recorded evidence; never infer success from a tool name.
export function callTitle(step: TranscriptStep) {
  const call = step.call;
  if (!call) return step.title;
  const name = call.name || step.title;
  const action = name.split(/__|\./).at(-1) || name;
  const words = action.replace(/[_-]+/g, ' ');
  return [call.provider, words.charAt(0).toUpperCase() + words.slice(1)]
    .filter(Boolean)
    .join(' · ');
}
export function callFacts(step: TranscriptStep, lang: 'en' | 'zh') {
  const c = step.call;
  if (!c) return [];
  const zh = lang === 'zh';
  const statuses: Record<string, string[]> = {
    completed: ['Completed', '已完成'],
    failed: ['Failed', '失败'],
    in_progress: ['Running', '进行中'],
    cancelled: ['Cancelled', '已取消'],
  };
  const facts = [{ label: zh ? '工具' : 'Tool', value: c.name || step.title }];
  if (c.provider)
    facts.push({ label: zh ? '服务' : 'Provider', value: c.provider });
  if (c.status)
    facts.push({
      label: zh ? '状态' : 'Status',
      value: statuses[c.status]?.[zh ? 1 : 0] || c.status,
    });
  if (typeof c.duration_ms === 'number')
    facts.push({
      label: zh ? '耗时' : 'Duration',
      value:
        c.duration_ms < 1000
          ? c.duration_ms + ' ms'
          : (c.duration_ms / 1000).toFixed(2) + ' s',
    });
  return facts;
}
function hasLossyJSONNumber(value: string) {
  // Scan JSON tokens linearly, with no decoded body or regex over large strings.
  // Preserve opaque 64-bit IDs and decimal values that Number would change.
  for (let i = 0; i < value.length; i++) {
    if (value[i] === '"') {
      for (i++; i < value.length; i++) {
        if (value[i] === '\\') i++;
        else if (value[i] === '"') break;
      }
      continue;
    }
    if (value[i] !== '-' && !(value[i] >= '0' && value[i] <= '9')) continue;
    const start = i;
    while (i + 1 < value.length && '0123456789.eE+-'.includes(value[i + 1]))
      i++;
    const token = value.slice(start, i + 1),
      n = Number(token);
    if (
      Object.is(n, -0) ||
      !Number.isFinite(n) ||
      (Number.isInteger(n) && !Number.isSafeInteger(n))
    )
      return true;
    const canonical = (text: string) => {
      const [mantissa, exp = '0'] = text.toLowerCase().split('e');
      const dot = mantissa.indexOf('.');
      let power = Number(exp) - (dot < 0 ? 0 : mantissa.length - dot - 1);
      let digits = mantissa.replace('.', '').replace(/^(-?)0+/, '$1');
      const trimmed = digits.replace(/0+$/, '');
      power += digits.length - trimmed.length;
      digits = trimmed;
      return digits === '' || digits === '-' ? '0' : digits + 'e' + power;
    };
    if (canonical(token) !== canonical(String(n))) return true;
  }
  return false;
}
function json(value: string) {
  if (hasLossyJSONNumber(value)) return undefined;
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}
export type CallSection = {
  id: string;
  label: string;
  source: string;
  hint: string;
  verbatim?: boolean;
  rows?: { label: string; value: string }[];
};
export function callSections(
  step: TranscriptStep,
  lang: 'en' | 'zh',
): CallSection[] {
  const zh = lang === 'zh',
    c = step.call,
    sections: CallSection[] = [];
  const add = (
    id: string,
    label: string,
    source: string,
    hint = 'text',
    parameters = false,
  ) => {
    const value = source.length <= 16000 ? json(source) : undefined;
    const rows =
      parameters &&
      value &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      Object.keys(value).length <= 24
        ? Object.entries(value).map(([key, v]) => ({
            label: key.replace(/_/g, ' '),
            value: typeof v === 'string' ? v : JSON.stringify(v, null, 2),
          }))
        : undefined;
    sections.push({
      id,
      label,
      source,
      hint,
      rows,
      verbatim: hint === 'json' && hasLossyJSONNumber(source),
    });
  };
  const argv =
    step.input_kind === 'arguments' &&
    ['command', 'read', 'list', 'search'].includes(step.operation || '');
  if (
    step.input &&
    step.input_kind !== 'parameters' &&
    (step.input_kind !== 'arguments' || argv)
  ) {
    add(
      'input',
      argv
        ? zh
          ? '命令参数'
          : 'Command arguments'
        : step.input_kind === 'changes'
        ? zh
          ? '文件变更'
          : 'File changes'
        : step.input_kind === 'command'
        ? zh
          ? '命令'
          : 'Command'
        : step.input_kind === 'code'
        ? zh
          ? '代码'
          : 'Code'
        : zh
        ? '输入'
        : 'Input',
      step.input,
      argv
        ? 'json'
        : step.input_kind === 'code'
        ? 'javascript'
        : step.input_kind === 'command'
        ? 'shell'
        : step.input_kind === 'changes'
        ? 'json'
        : 'text',
    );
  }
  const args =
    c?.arguments ||
    (['parameters', 'arguments'].includes(step.input_kind || '')
      ? step.input || ''
      : '');
  if (
    args &&
    !argv &&
    (!step.input ||
      ['parameters', 'arguments'].includes(step.input_kind || '') ||
      step.operation === 'tool')
  )
    add('arguments', zh ? '参数' : 'Parameters', args, 'json', true);
  if (c?.has_result && c.loaded) {
    const result =
      (c.result?.length || 0) < 262144 ? json(c.result || '') : undefined;
    // Prefer structured data, otherwise preserve every MCP text block. Raw keeps
    // the entire sanitized recorded envelope, including error/type metadata.
    if (
      result === null ||
      result === '' ||
      (Array.isArray(result?.content) &&
        result.content.length === 0 &&
        result.structuredContent == null)
    )
      add(
        'empty',
        zh ? '结果' : 'Result',
        zh ? '调用已返回，没有内容。' : 'The call returned no content.',
        'notice',
      );
    else if (
      result &&
      typeof result === 'object' &&
      result.structuredContent != null
    )
      add(
        'result',
        zh ? '结果' : 'Result',
        JSON.stringify(result.structuredContent, null, 2),
        'json',
      );
    else if (result && Array.isArray(result.content)) {
      const blocks = result.content.filter(
        (v: unknown): v is { type: string; text: string } =>
          !!v &&
          typeof v === 'object' &&
          (v as { type?: string }).type === 'text' &&
          typeof (v as { text?: string }).text === 'string',
      );
      if (blocks.length)
        blocks.forEach((block: { text: string }, i: number) =>
          add(
            'result-' + i,
            zh ? '结果' : 'Result',
            block.text,
            json(block.text) !== undefined ? 'json' : 'text',
          ),
        );
      else add('result', zh ? '结果' : 'Result', c.result || 'null', 'json');
    } else
      add(
        'result',
        zh ? '结果' : 'Result',
        typeof result === 'string' ? result : c.result || 'null',
        typeof result === 'string' ? 'text' : 'json',
      );
    if (result?.isError === true)
      sections.unshift({
        id: 'error',
        label: zh ? '工具返回错误' : 'Tool returned an error',
        source: zh
          ? '以下为工具记录的错误结果。'
          : 'The tool recorded an error result.',
        hint: 'notice',
      });
  }
  if (c && !c.loaded && (c.has_arguments || c.has_result))
    add(
      'pending',
      zh ? '详情' : 'Details',
      zh
        ? '打开详情以读取已记录的参数和结果。'
        : 'Open details to load recorded parameters and results.',
      'notice',
    );
  else if (c && !c.has_result)
    add(
      'unrecorded',
      zh ? '结果' : 'Result',
      zh
        ? '此记录未包含工具结果。'
        : 'No tool result is included in this record.',
      'notice',
    );
  if (!sections.length)
    add(
      'summary',
      zh ? '已记录摘要' : 'Recorded summary',
      step.detail ||
        (zh
          ? '此记录未包含详细输入。'
          : 'No detailed input is included in this record.'),
      'notice',
    );
  if (c?.redacted)
    add(
      'redacted',
      zh ? '隐私保护' : 'Privacy',
      zh
        ? '凭据字段和二进制数据已隐藏。'
        : 'Credential fields and binary data are hidden.',
      'notice',
    );
  return sections;
}
export function callRaw(step: TranscriptStep) {
  const c = step.call;
  if (!c?.loaded)
    return (
      step.input ||
      step.detail ||
      (c
        ? JSON.stringify(
            {
              tool: c.name || step.title,
              status: c.status || '',
              details: 'not loaded',
            },
            null,
            2,
          )
        : '')
    );
  // Raw describes the sanitized recorded structure, not formatted command source.
  return (
    '{\n  "tool": ' +
    JSON.stringify(c.name || step.title) +
    ',\n  "status": ' +
    JSON.stringify(c.status || '') +
    (step.input ? ',\n  "input": ' + JSON.stringify(step.input) : '') +
    (c.has_arguments ? ',\n  "arguments": ' + (c.arguments || 'null') : '') +
    (c.has_result ? ',\n  "result": ' + (c.result || 'null') : '') +
    '\n}'
  );
}
export function toolDetailRefs(step: TranscriptStep) {
  return [
    ...new Set(
      [
        step.details_truncated ? step.input_ref : '',
        step.call && !step.call.loaded ? step.call.ref : '',
      ].filter((v): v is string => !!v),
    ),
  ];
}
export async function loadToolDetails(
  step: TranscriptStep,
  read: (ref: string) => Promise<TranscriptStep>,
) {
  const refs = toolDetailRefs(step),
    values = await Promise.all(refs.map(read));
  let merged = { ...step };
  for (let i = 0; i < refs.length; i++) {
    const value = values[i];
    if (value.input_ref !== refs[i] || value.details_truncated)
      throw new Error('Incomplete tool details');
    if (step.details_truncated && step.input_ref === refs[i])
      merged = {
        ...merged,
        input: value.input,
        workdir: value.workdir,
        details_truncated: false,
      };
    if (step.call?.ref === refs[i]) {
      if (
        !value.call?.loaded ||
        value.call.ref !== refs[i] ||
        (value.call.has_arguments &&
          typeof value.call.arguments !== 'string') ||
        (value.call.has_result && typeof value.call.result !== 'string')
      )
        throw new Error('Incomplete recorded call');
      merged = { ...merged, call: value.call };
    }
  }
  return merged;
}
// Bound rendering cost, never the stored/copyable source. Page labels disclose
// the visual window; UTF-16 surrogate pairs are kept intact across boundaries.
export function callPage(source: string, page: number) {
  const size = 16000,
    pages = Math.max(1, Math.ceil(source.length / size));
  const index = Math.max(0, Math.min(pages - 1, page));
  const boundary = (n: number) =>
    n > 0 && /[\uDC00-\uDFFF]/.test(source.charAt(n)) ? n - 1 : n;
  return {
    source: source.slice(
      boundary(index * size),
      boundary(Math.min(source.length, (index + 1) * size)),
    ),
    page: index,
    pages,
  };
}
