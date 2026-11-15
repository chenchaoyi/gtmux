import fs from 'fs';
import path from 'path';
import ts from 'typescript';

function files(root: string): string[] {
  return fs.readdirSync(root, {withFileTypes: true}).flatMap(entry => entry.isDirectory() ? files(path.join(root, entry.name)) : /\.tsx?$/.test(entry.name) && !entry.name.includes('.test.') ? [path.join(root, entry.name)] : []);
}
test('BLP-004 App-owned loading views share the brand engine instead of native ActivityIndicator', () => {
  const root = path.resolve(__dirname, '..');
  const violations: string[] = [];
  for (const file of [...files(root), path.join(root, '../App.tsx')]) {
    const source = ts.createSourceFile(file, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    const namespaces = new Set<string>();
    for (const statement of source.statements) {
      if (!ts.isImportDeclaration(statement) || !ts.isStringLiteral(statement.moduleSpecifier) || statement.moduleSpecifier.text !== 'react-native' || statement.importClause?.isTypeOnly) continue;
      const clause = statement.importClause;
      if (clause?.name) namespaces.add(clause.name.text);
      const bindings = clause?.namedBindings;
      if (bindings && ts.isNamespaceImport(bindings)) namespaces.add(bindings.name.text);
      if (bindings && ts.isNamedImports(bindings) && bindings.elements.some(item => !item.isTypeOnly && (item.propertyName ?? item.name).text === 'ActivityIndicator')) violations.push(path.relative(root, file));
    }
    const inspect = (node: ts.Node) => {
      if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.expression) && namespaces.has(node.expression.text) && node.name.text === 'ActivityIndicator') violations.push(path.relative(root, file));
      ts.forEachChild(node, inspect);
    };
    inspect(source);
  }
  expect(violations).toEqual([]);
});
