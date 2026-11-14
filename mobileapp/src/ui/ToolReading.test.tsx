import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {TouchableOpacity} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {ToolCallDetails} from './ToolCallDetails';
import {CodeText} from './CodeText';
import {callRaw} from './toolPresentation';
import type {TranscriptStep} from '../api/client';

test('TSV-001 command preview is direct, single-command outline absent, original input and result copy remain exact',()=>{
 const command="printf 'hello'";const source=JSON.stringify(['/bin/bash','-c',command]);
 const step={kind:'tool',title:'exec',operation:'command',input_kind:'arguments',input:source,call:{name:'exec',loaded:true,has_arguments:true,arguments:source,has_result:true,result:JSON.stringify({content:[{type:'text',text:'\x1b[32mOK\x1b[0m'}]})}} as TranscriptStep;
 const before=JSON.stringify(step);let tree!:renderer.ReactTestRenderer;
 act(()=>{tree=renderer.create(<ToolCallDetails step={step} lang="en"/>);});
 try{
  const input=tree.root.findByProps({testID:'call-section-input'});
  expect(input.findByType(CodeText).props.source).toBe(command);
  expect(input.findAllByType(TouchableOpacity).filter(n=>n.props.testID==='call-code-outline-toggle')).toHaveLength(0);
  expect(tree.root.findAllByType(CodeText).some(n=>n.props.source==='OK')).toBe(true);
  const copy=input.findAllByType(TouchableOpacity).find(n=>n.props.accessibilityLabel==='Copy full Command arguments')!;
  act(()=>copy.props.onPress());expect(Clipboard.setString).toHaveBeenLastCalledWith(source);
  const raw=input.findAllByType(TouchableOpacity).find(n=>n.props.testID==='call-code-raw-toggle')!;
  act(()=>raw.props.onPress());expect(JSON.parse(input.findByType(CodeText).props.source)).toEqual(JSON.parse(source));
  act(()=>tree.update(<ToolCallDetails step={step} lang="en" raw/>));
  expect(tree.root.findByType(CodeText).props.source).toBe(callRaw(step));
  expect(JSON.stringify(step)).toBe(before);
 }finally{act(()=>tree.unmount());}
});
