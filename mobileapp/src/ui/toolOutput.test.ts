import {readableToolOutput,displayWorkdir} from './toolPresentation';
test('TSV-002 terminal preview removes controls while preserving literal and raw evidence',()=>{
 const source='\x1b]8;;https://synthetic.invalid\x07link\x1b]8;;\x07 \x1b[32mOK\x1b[0m\n';
 expect(readableToolOutput(source,false)).toBe('link OK\n');
 expect(readableToolOutput('[32mOK[0m',true)).toBe('OK');
 expect(readableToolOutput('[32m reference',true)).toBe('[32m reference');
 expect(readableToolOutput('[32mOK[0m',false)).toBe('[32mOK[0m');
 expect(source).toContain('\x1b[32m');
});
test('TSV-002 local file URI presentation does not reinterpret foreign or malformed paths',()=>{
 expect(displayWorkdir('file:///tmp/a%20b')).toBe('/tmp/a b');
 for(const value of ['file://remote/private/a','file:///bad%XX','https://synthetic.invalid/a','/tmp/already plain'])expect(displayWorkdir(value)).toBe(value);
});
