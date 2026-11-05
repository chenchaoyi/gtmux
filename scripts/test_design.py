#!/usr/bin/env python3
"""Validate reviewed test designs, implementation mapping and source catalogue.

The seal detects accidental edit drift, not human identity or semantic correctness.
PR history validates order; a squashed main snapshot validates content only.
"""
import argparse
import csv
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
CATALOGUE = 'docs/test/catalogue.json'
MAPPING = 'docs/test/implementations.json'
LAYERS = {'L1', 'L2', 'L3', 'L4', 'G', 'M'}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def seal(plan):
    return sha(json.dumps({k: v for k, v in plan.items() if k != 'seal'}, sort_keys=True, separators=(',', ':')).encode())


def safe(path):
    return isinstance(path, str) and bool(path) and not path.startswith('/') and '..' not in PurePosixPath(path).parts and '\\' not in path


def is_test(path):
    p = PurePosixPath(path)
    return (path.endswith(('_test.go', '_test.py', '_test.rb')) or
            (p.name.startswith('test_') and p.suffix == '.py') or
            bool(re.search(r'\.(?:test|spec)\.(?:[cm]?[jt]sx?)$', p.name)) or
            ('/Tests/' in path and p.suffix == '.swift') or
            ('/tests/' in path and p.suffix in {'.swift', '.cpp'}) or
            ('/src/test/' in path and p.suffix in {'.kt', '.java'}))


def tracked(root):
    # Include newly staged/untracked source while excluding borrowed dependencies.
    raw = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=root)
    return set(raw.decode().rstrip('\0').split('\0')) - {''}


def validate(root, catalogue, mappings, plans, paths):
    errors = []
    cases = {}
    for name, plan in plans.items():
        if not safe(name) or plan.get('status') != 'reviewed' or plan.get('seal') != seal(plan):
            errors.append('Unreviewed or changed design: ' + name)
        review = plan.get('review', {})
        if not all(review.get(k) for k in ('reviewer', 'date', 'decision', 'notes')):
            errors.append('Missing review record: ' + name)
        for case in plan.get('cases', []):
            key = case.get('id')
            if not key or key in cases or case.get('layer') not in LAYERS or not all(case.get(k) for k in ('description', 'setup', 'action', 'expected', 'planned_files')) or not all(safe(p) for p in case.get('planned_files', [])):
                errors.append('Invalid or duplicate case in ' + name)
            cases[key] = (name, case)
    source = catalogue.get('files', {})
    seen = set()
    for path, file in source.items():
        if file.get('layer') not in LAYERS:
            errors.append('Invalid catalogue layer: ' + path)
        for case in file.get('cases', []):
            if not case.get('id') or case['id'] in seen or case.get('layer', file.get('layer')) not in LAYERS or not isinstance(case.get('line'), int) or case['line'] < 1 or not case.get('name'):
                errors.append('Invalid/duplicate catalogue entry: ' + path)
            seen.add(case.get('id'))
    for path in sorted(set(source) | {p for p in paths if is_test(p) and not mappings.get(p, {}).get('removed')}):
        item = source.get(path)
        if not safe(path) or not item or not (root / path).is_file() or (root / path).is_symlink():
            errors.append('Missing/unsafe catalogue source: ' + path)
        elif item.get('sha256') != sha((root / path).read_bytes()):
            errors.append('Catalogue source changed: ' + path)
    for path, item in mappings.items():
        if not safe(path):
            errors.append('Unsafe implementation path'); continue
        removed = item.get('removed', False)
        if removed:
            if (root / path).exists() or not item.get('reason'):
                errors.append('Removal lacks rationale or source still exists: ' + path)
        elif path not in source:
            errors.append('Implementation missing from catalogue: ' + path)
        for key in item.get('cases', []):
            if key not in cases or path not in cases[key][1]['planned_files']:
                errors.append('Implementation outside reviewed design: ' + path)
        if not item.get('cases'):
            errors.append('Implementation has no design cases: ' + path)
        if not removed:
            text = (root / path).read_text() if (root / path).is_file() else ''
            selectors = item.get('selectors', [])
            if not selectors or any(not isinstance(s, str) or not s or s not in text for s in selectors):
                errors.append('Missing implementation selector: ' + path)
    return errors


def history_errors(root, base, head, mappings, plans):
    if not all(re.fullmatch('[0-9a-f]{40}', x or '') for x in (base, head)):
        return ['PR history requires exact base and head commits']
    changed = subprocess.check_output(['git', 'diff', '--name-only', '--no-renames', '-z', base + '...' + head], cwd=root).decode().rstrip('\0').split('\0')
    old = json.loads(subprocess.check_output(['git', 'show', base + ':' + CATALOGUE], cwd=root)) if subprocess.run(['git', 'cat-file', '-e', base + ':' + CATALOGUE], cwd=root, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0 else {'files': {}}
    errors = []
    for path in changed:
        if not is_test(path) and path not in old['files'] and path not in mappings:
            continue
        mapping = mappings.get(path)
        if not mapping:
            errors.append('Changed test lacks reviewed mapping: ' + path); continue
        # Find the first edit in this PR, not just the latest update.
        commits = subprocess.check_output(['git', 'rev-list', '--reverse', base + '..' + head, '--', path], cwd=root).decode().splitlines()
        if not commits:
            errors.append('Cannot establish first implementation edit: ' + path); continue
        parent = commits[0] + '^'
        for key in mapping['cases']:
            candidates = [(n, p) for n, p in plans.items() if any(c['id'] == key for c in p['cases'])]
            if len(candidates) != 1:
                errors.append('Unknown reviewed case: ' + key); continue
            name, plan = candidates[0]
            try:
                prior = json.loads(subprocess.check_output(['git', 'show', parent + ':' + name], cwd=root, stderr=subprocess.DEVNULL))
                existed = subprocess.run(['git', 'cat-file', '-e', base + ':' + path], cwd=root, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
                if existed and subprocess.run(['git', 'cat-file', '-e', base + ':' + name], cwd=root, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                    old_plan = json.loads(subprocess.check_output(['git', 'show', base + ':' + name], cwd=root))
                    if old_plan.get('seal') == prior.get('seal'):
                        raise ValueError('Changed test needs a fresh review, not an unchanged historical seal')
                if prior.get('seal') != seal(prior) or prior.get('status') != 'reviewed' or not any(c == next(c for c in plan['cases'] if c['id'] == key) for c in prior.get('cases', [])):
                    raise ValueError('Design changed')
            except (subprocess.CalledProcessError, ValueError, StopIteration):
                errors.append('Design must be reviewed in an earlier commit: ' + path + ' / ' + key)
    return errors


def render_csv(catalogue, layer=None):
    output = io.StringIO(newline='')
    w = csv.writer(output, lineterminator='\n')
    w.writerow(['id', 'layer', 'stack', 'file', 'line', 'kind', 'description', 'dynamic', 'design'])
    for path, file in sorted(catalogue['files'].items()):
        for c in file.get('cases', []):
            if layer and c.get('layer', file['layer']) != layer:
                continue
            w.writerow([c['id'], c.get('layer', file['layer']), file['stack'], path, c['line'], c['kind'], c['name'], c.get('dynamic', False), c.get('design', 'legacy-not-retrospectively-reviewed')])
    return output.getvalue()


def render_summary(catalogue, plans, zh=False):
    from collections import Counter
    counts = Counter(c.get('layer', f['layer']) for f in catalogue['files'].values() for c in f.get('cases', []))
    names = {'L1': ('Unit / pure logic', '单元/纯逻辑'), 'L2': ('Component / contract', '组件/契约'), 'L3': ('Runtime integration', '运行时集成'), 'L4': ('Application journeys', '应用操作流程'), 'M': ('Manual / device', '人工/设备'), 'G': ('Qualification guards', '质量门禁')}
    if zh:
        text = '# 分层用例目录\n\n[English](CASES.md) · [测试体系](TEST-PLAN.zh.md) · [完整 CSV](CASES.csv)\n\n'
        text += f"目录覆盖 {len(catalogue['files'])} 份测试/辅助源码。CSV 每行列出稳定 ID、层次、技术栈、源码/行、原始测试描述、动态标记与设计来源。下表链接分别是各层完整列表，可筛选或导入表格；英文测试名保持源码原文，便于定位。\n\n"
        text += '| 层次 | 静态目录条目 | 完整用例列表 |\n| --- | --- | --- |\n'
    else:
        text = '# Case catalogue by layer\n\n[中文](CASES.zh.md) · [Test system](TEST-PLAN.md) · [Complete CSV](CASES.csv)\n\n'
        text += f"The catalogue covers {len(catalogue['files'])} test/harness source files. Each CSV row lists stable ID, layer, stack, source/line, original description, dynamic flag and design origin. Layer links contain the complete lists for filtering/spreadsheets; names remain verbatim for source lookup.\n\n"
        text += '| Layer | Static index entries | Complete case list |\n| --- | --- | --- |\n'
    for layer in ('L1', 'L2', 'L3', 'L4', 'M', 'G'):
        text += f'| {layer} · {names[layer][int(zh)]} | {counts[layer]} | [{layer}.csv](cases/{layer}.csv) |\n'
    if zh:
        text += '\n数字不是通过数。存量 6,223 个位置来自 4fad6119 盘点（声明、子测试、动态模板），另加明确的新用例、原生过程入口和评审映射入口；动态实例不展开。没有具名用例的辅助文件仍保留摘要。存量层次是结构分类，混合边界在触及时需语义复核，不补造历史评审/运行结果。`reviewed-selector` 也不冒充框架发现的声明。源码/目录/映射漂移由门禁拦截，但测试名不能自动证明断言质量。\n\n索引由 `python3 scripts/test_design.py --refresh` 同步，只接受已评审映射的变更。详见[设计工作流](DESIGN-WORKFLOW.zh.md)。\n\n## 已评审设计用例\n\n| ID | 预期行为（源码英文） |\n| --- | --- |\n'
    else:
        text += '\nNumbers are not passes. The legacy 6,223 positions came from the 4fad6119 inventory (declarations, subtests, dynamic templates); explicit new cases, native oracles and reviewed selectors are indexed separately. Dynamic instances are not expanded. Harnesses without named cases retain their hashes. Legacy layers are structural triage; mixed boundaries need semantic review when touched, not fictional historical review/results. A reviewed-selector is not a framework-discovered declaration. Gates catch source/index/mapping drift; names cannot prove assertion quality.\n\nRefresh with `python3 scripts/test_design.py --refresh`; only reviewed/mapped changes are accepted. See [design workflow](DESIGN-WORKFLOW.md).\n\n## Reviewed design cases\n\n| ID | Expected behavior |\n| --- | --- |\n'
    for _, plan in sorted(plans.items()):
        for case in plan['cases']:
            text += '| ' + case['id'] + ' | ' + case['expected'].replace('|', '\\|').replace('\n', ' ') + ' |\n'
    return text


def refresh_catalogue(root, catalogue, mappings, plans, paths):
    """Only mapped changes may update the baseline; existing records stay explicit."""
    result = json.loads(json.dumps(catalogue))
    for path in sorted(set(result['files']) | set(mappings) | {p for p in paths if is_test(p)}):
        file = root / path
        item = result['files'].get(path)
        changed = not file.is_file() or item is None or item.get('sha256') != sha(file.read_bytes())
        if not changed:
            continue
        mapping = mappings.get(path)
        if not mapping or not mapping.get('cases'):
            raise ValueError('Refresh requires a reviewed mapping: ' + path)
        if not file.is_file():
            if not mapping.get('removed') or not mapping.get('reason'):
                raise ValueError('Removal requires a reviewed rationale: ' + path)
            result['files'].pop(path, None)
            continue
        if file.is_symlink() or not safe(path):
            raise ValueError('Unsafe source: ' + path)
        matched = [c for p in plans.values() for c in p.get('cases', []) if c.get('id') in mapping.get('cases', []) and path in c.get('planned_files', [])]
        if len(matched) != len(mapping.get('cases', [])):
            raise ValueError('Unknown/unplanned refresh case: ' + path)
        item = result['files'].setdefault(path, {'layer': matched[0]['layer'], 'stack': 'reviewed implementation', 'cases': []})
        text = file.read_text()
        item['sha256'] = sha(file.read_bytes())
        for case in item['cases']:
            lines = [i + 1 for i, line in enumerate(text.splitlines()) if case['name'] in line]
            if lines:
                case['line'] = lines[0]
        # Reviewed selectors are a separate index kind; do not present them as
        # automatically discovered or expanded framework declarations.
        for selector in mapping.get('selectors', []):
            if not selector or selector not in text:
                raise ValueError('Missing refresh selector: ' + path)
            key = 'MAP-' + sha((path + '|' + selector).encode())[:12]
            if not any(c.get('name') == selector for c in item['cases']):
                item['cases'].append({'id': key, 'line': next(i + 1 for i, line in enumerate(text.splitlines()) if selector in line), 'kind': 'reviewed-selector', 'name': selector, 'dynamic': False, 'layer': matched[0]['layer'], 'design': ','.join(mapping['cases'])})
    errors = validate(root, result, mappings, plans, paths)
    if errors:
        raise ValueError('; '.join(errors))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--base')
    parser.add_argument('--head')
    parser.add_argument('--pr-history', action='store_true')
    parser.add_argument('--write-index', action='store_true')
    parser.add_argument('--refresh', action='store_true', help='Refresh only reviewed/mapped test source changes and the case CSV')
    parser.add_argument('--seal-plan', help='Seal an already reviewed design record; does not approve it')
    args = parser.parse_args()
    root = args.root
    try:
        if args.seal_plan:
            if not safe(args.seal_plan) or not args.seal_plan.startswith('docs/test/designs/'):
                raise ValueError('Unsafe design record')
            path = root / args.seal_plan
            plan = json.loads(path.read_text())
            if plan.get('status') != 'reviewed' or not all(plan.get('review', {}).get(k) for k in ('reviewer', 'date', 'decision', 'notes')):
                raise ValueError('Record the actual design review before sealing')
            plan['seal'] = seal(plan)
            path.write_text(json.dumps(plan, indent=2) + '\n')
            print('test-design: sealed reviewed record; commit it before implementation')
            return 0
        catalogue = json.loads((root / CATALOGUE).read_text())
        mappings = json.loads((root / MAPPING).read_text())
        plans = {p.relative_to(root).as_posix(): json.loads(p.read_text()) for p in (root / 'docs/test/designs').glob('*.json')}
        if args.refresh:
            catalogue = refresh_catalogue(root, catalogue, mappings, plans, tracked(root))
            (root / CATALOGUE).write_text(json.dumps(catalogue, ensure_ascii=False, indent=2) + '\n')
        errors = validate(root, catalogue, mappings, plans, tracked(root))
        csv_path = root / 'docs/test/CASES.csv'
        expected = render_csv(catalogue)
        if args.write_index or args.refresh:
            csv_path.write_text(expected)
        elif not csv_path.is_file() or csv_path.read_text() != expected:
            errors.append('Case index drift: run python3 scripts/test_design.py --write-index')
        for layer in sorted(LAYERS):
            layer_path = root / ('docs/test/cases/' + layer + '.csv')
            layer_csv = render_csv(catalogue, layer)
            if args.write_index or args.refresh:
                layer_path.parent.mkdir(exist_ok=True)
                layer_path.write_text(layer_csv)
            elif not layer_path.is_file() or layer_path.read_text() != layer_csv:
                errors.append('Layer index drift: ' + layer)
        for zh in (False, True):
            summary_path = root / ('docs/test/CASES' + ('.zh' if zh else '') + '.md')
            summary = render_summary(catalogue, plans, zh)
            if args.write_index or args.refresh:
                summary_path.write_text(summary)
            elif not summary_path.is_file() or summary_path.read_text() != summary:
                errors.append('Case summary drift: ' + summary_path.name)
        if args.pr_history:
            errors += history_errors(root, args.base, args.head, mappings, plans)
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as error:
        errors = ['Cannot validate test design: ' + (str(error) if type(error) is ValueError else type(error).__name__)]
    for error in errors:
        print('test-design: ' + error, file=sys.stderr)
    if not errors:
        print('test-design: catalogue, reviewed designs and implementation selectors agree' + ('; PR order verified' if args.pr_history else ' (snapshot only)'))
    return int(bool(errors))


if __name__ == '__main__':
    raise SystemExit(main())
