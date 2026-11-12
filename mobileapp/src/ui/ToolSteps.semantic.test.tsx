import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {StyleSheet} from 'react-native';
import {ToolSteps} from './ToolSteps';
import type {TranscriptStep} from '../api/client';

test('AOP001 AOP003 real Activity rows share semantic visible and spoken titles with touch targets',()=>{
 for(const lang of ['en','zh'] as const){
  let tree!:renderer.ReactTestRenderer;
  const steps: TranscriptStep[]=[{kind:'tool',title:'exec',operation:'read',input:'cat one.md two.md',input_kind:'command',detail:'one.md, two.md',call:{name:'exec',status:'completed'}}];
  act(()=>{tree=renderer.create(<ToolSteps steps={steps} open onToggle={()=>{}} lang={lang} fontSize={14}/>);});
  try{
   const row=tree.root.findAll(n=>n.props.testID==='tool-step-toggle-0'&&typeof n.props.onPress==='function')[0];
   expect(row.props.accessibilityLabel).toContain(lang==='zh'?'读取文件':'Read file');
   expect(StyleSheet.flatten(row.props.style).minHeight).toBeGreaterThanOrEqual(44);
   const visible=JSON.stringify(tree.toJSON());expect(visible).toContain(lang==='zh'?'读取文件':'Read file');
   expect(tree.root.findAllByProps({testID:'tool-step-row'}).length).toBeGreaterThanOrEqual(1);
   expect(row.props.accessibilityState.expanded).toBe(false);
   act(()=>row.props.onPress());expect(tree.root.findAll(n=>n.props.testID==='tool-step-toggle-0'&&n.props.onPress)[0].props.accessibilityState.expanded).toBe(true);
   expect(steps[0].call?.name).toBe('exec');
  }finally{act(()=>tree.unmount());}
 }
});
