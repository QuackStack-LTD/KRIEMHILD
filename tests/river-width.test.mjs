import test from 'node:test';
import assert from 'node:assert/strict';
import {riverWidthPixels} from '../src/terrain/river-width.js';
import {getRiverThickness,setRiverThickness,subscribeRiverThickness,normalizeRiverThickness,riverThicknessKey} from '../src/terrain/river-display.js';

test('rivers stay thin in overview and grow continuously toward local zoom',()=>{
  const features=[['stream',1,4],['regional',100,60],['major',10000,800]].map(([rank,q,width])=>({class:rank,dischargeM3s:q,physicalWidthMetres:width,width:width/200000}));
  const symbols=features.map(f=>riverWidthPixels(f,0,5,0));
  assert.ok(symbols[1]>symbols[0]*1.5&&symbols[2]>symbols[1]*1.4);
  for(const f of features){
    assert.ok(riverWidthPixels(f,0,6,0)<1.5);
    let previous=0;
    for(let detail=0;detail<=8;detail+=.1){
      const width=riverWidthPixels(f,0,6*2**detail,detail);
      assert.ok(width>=previous,'zooming in must not shrink a river');previous=width;
    }
    assert.ok(riverWidthPixels(f,0,1536,8)>riverWidthPixels(f,0,6,0)*5);
    assert.equal(riverWidthPixels(f,0,100000,7),f.width*100000);
    for(let detail=3;detail<7;detail+=.1)assert.ok(Math.abs(riverWidthPixels(f,0,5,detail+.001)-riverWidthPixels(f,0,5,detail))<.01);
  }
  assert.equal(features[0].width,4/200000);
});

test('saved legacy and authored rivers use the same zoom policy',()=>{
  for(const id of ['authored/river','generated/river']){
    const f={id,width:.1,discharge:144};
    assert.ok(riverWidthPixels(f,0,6,0)<1.5);
    assert.ok(riverWidthPixels(f,0,768,7)>riverWidthPixels(f,0,6,0));
  }
});


test('equal-discharge geological widths remain distinct at the same zoom',()=>{
 const make=metres=>({class:'regional',dischargeM3s:300,physicalWidthMetres:metres,width:metres/200000});
 const thin=make(25),wide=make(1330);
 const a=riverWidthPixels(thin,0,384,6),b=riverWidthPixels(wide,0,384,6);
 assert.ok(a>1.8&&a<2.3,`narrow river: ${a}`);
 assert.ok(b>19&&b<22,`broad river: ${b}`);
 assert.ok(b/a>9.5);
 const variable={...wide,widths:[wide.width*.2,wide.width]};
 assert.ok(riverWidthPixels(variable,1,384,6)>riverWidthPixels(variable,0,384,6)*2);
});

test('display thickness scales generated and authored profiles without changing geography',()=>{
 const features=[{id:'generated/river',width:.001,physicalWidthMetres:200,widths:[.0005,.001]},
   {id:'authored/river',width:.001,physicalWidthMetres:200,widths:[.0005,.001]}];
 const saved=structuredClone(features);
 for(const detail of [0,3,6,8])for(const f of features){
   const base=riverWidthPixels(f,1,384,detail,1);
   assert.equal(riverWidthPixels(f,1,384,detail,4),base*4);
   assert.equal(riverWidthPixels(f,1,384,detail,.25),base*.25);
   const ratio=base/riverWidthPixels(f,0,384,detail,1);
   assert.equal(riverWidthPixels(f,1,384,detail,4)/riverWidthPixels(f,0,384,detail,4),ratio);
 }
 assert.deepEqual(features,saved);
});

test('display preference persists, notifies live renderers and cleans up subscriptions',async()=>{
 const storage=Object.getOwnPropertyDescriptor(globalThis,'localStorage'),values=new Map();
 Object.defineProperty(globalThis,'localStorage',{configurable:true,value:{getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,value)}});
 let updates=0;const abort=new AbortController();
 const unsubscribe=subscribeRiverThickness(()=>updates++,abort.signal);
 try{
   setRiverThickness(2.5);
   assert.equal(getRiverThickness(),2.5);
   assert.equal(values.get(riverThicknessKey),'2.5');
   assert.equal(updates,1);
   setRiverThickness(2.5);assert.equal(updates,1);
   const f={width:.01,physicalWidthMetres:30};
   assert.equal(riverWidthPixels(f,0,10,4),riverWidthPixels(f,0,10,4,1)*2.5);
   const restored=await import('../src/terrain/river-display.js?restore-test');
   assert.equal(restored.getRiverThickness(),2.5);
   abort.abort();setRiverThickness(3);assert.equal(updates,1);
   assert.equal(normalizeRiverThickness(null),1);
   assert.equal(normalizeRiverThickness('bad'),1);
   assert.equal(normalizeRiverThickness(-1),.25);
   assert.equal(normalizeRiverThickness(100),4);
 }finally{
   unsubscribe();setRiverThickness(1);
   if(storage)Object.defineProperty(globalThis,'localStorage',storage);else delete globalThis.localStorage;
 }
});
