import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import assert from 'node:assert/strict';
import ts from 'typescript';

function registeredCase(control: any, back: () => Promise<void>) {
  const file = ts.createSourceFile('run.cjs', fs.readFileSync(path.join(__dirname, 'run.cjs'), 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const matches: string[] = [];
  function visit(node: ts.Node) {
    if (ts.isCallExpression(node) && node.expression.getText(file) === 'check' && ts.isStringLiteral(node.arguments[0]) && node.arguments[0].text === '10-diff-wrap-and-back') matches.push(node.arguments[1].getText(file));
    ts.forEachChild(node, visit);
  }
  visit(file);
  expect(matches).toHaveLength(1);
  return vm.runInNewContext('(' + matches[0] + ')', {
    tools: async (name: string) => expect(name).toBe('Diff'),
    id: (name: string) => {expect(name).toBe('diff-wrap'); return control;},
    tap: async () => {}, shown: (element: any) => element.isDisplayed(), driver: {back}, assert,
  });
}

test('DTV006 registered Diff journey waits for delayed dismissal after one Back', async () => {
  let visible = true, release!: () => void, finished = false;
  const dismissed = new Promise<void>(resolve => {release = resolve;});
  const reverse = jest.fn();
  const control = {
    waitForDisplayed: async (options: any) => {
      expect(options.timeout).toBe(10000);
      if (options.reverse) {reverse(); await dismissed;}
    },
    isDisplayed: async () => visible,
  };
  const back = jest.fn(async () => {});
  const result = registeredCase(control, back)().then(() => ({ok: true}), (error: unknown) => ({error})).finally(() => {finished = true;});
  try {
    for (let i = 0; i < 20; i++) await Promise.resolve();
    expect(back).toHaveBeenCalledTimes(1);
    expect(finished).toBe(false);
    expect(reverse).toHaveBeenCalledTimes(1);
    visible = false; release();
    expect(await result).toEqual({ok: true});
  } finally {visible = false; release(); await result;}
});

test('DTV006 registered Diff journey preserves a stuck-dismissal timeout', async () => {
  const error = new Error('Diff remained visible at close deadline');
  const control = {waitForDisplayed: async (options: any) => {if (options.reverse) {expect(options.timeout).toBe(10000); throw error;}}, isDisplayed: async () => true};
  const back = jest.fn(async () => {});
  await expect(registeredCase(control, back)()).rejects.toBe(error);
  expect(back).toHaveBeenCalledTimes(1);
});

test('DTV006 registered Diff journey keeps its final false oracle', async () => {
  const control = {waitForDisplayed: async () => {}, isDisplayed: async () => true};
  await expect(registeredCase(control, async () => {})()).rejects.toThrow('true !== false');
});

test('DTV006 registered Diff journey accepts immediate close and preserves Back errors', async () => {
  const control = {waitForDisplayed: async () => {}, isDisplayed: async () => false};
  await registeredCase(control, async () => {})();
  const error = new Error('original driver Back failure');
  await expect(registeredCase(control, async () => {throw error;})()).rejects.toBe(error);
});
