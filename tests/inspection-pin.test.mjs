import {test} from 'node:test';
import assert from 'node:assert/strict';
import {bindInspectionClick} from '../src/terrain/inspection-pin.js';
test('Inspection ignores drags, cancelled gestures, right clicks and active editing tools',()=>{
 const handlers={},canvas={addEventListener:(name,fn)=>handlers[name]=fn};let enabled=true;const pins=[];
 bindInspectionClick(canvas,()=>enabled,e=>({x:e.clientX,y:e.clientY}),p=>pins.push(p),new AbortController().signal);
 const emit=(type,x=20,y=20,button=0)=>handlers[type]({clientX:x,clientY:y,button,pointerId:1});
 emit('pointerdown');emit('pointerup');assert.deepEqual(pins,[{x:20,y:20}]);
 emit('pointerdown');emit('pointermove',40);emit('pointermove',20);emit('pointerup');
 emit('pointerdown');emit('pointercancel');emit('pointerup');
 emit('pointerdown',20,20,2);emit('pointerup',20,20,2);
 enabled=false;emit('pointerdown');emit('pointerup');enabled=true;
 emit('pointerdown');emit('pointerup',70);
 assert.equal(pins.length,1);
});
