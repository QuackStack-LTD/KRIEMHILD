import {test} from 'node:test';
import assert from 'node:assert/strict';
import {AutosaveQueue} from '../src/terrain/autosave.js';

test('Autosave serializes requests and keeps the newest gesture during an in-flight write',async()=>{
  const writes=[];let release;
  const queue=new AutosaveQueue(async value=>{writes.push(value);if(value===1)await new Promise(r=>release=r);},()=>{},60000);
  try {queue.schedule(1);const saving=queue.flush();queue.schedule(2);queue.schedule(3);assert.deepEqual(writes,[1]);release();await saving;assert.deepEqual(writes,[1,3]);assert.equal(queue.pending,null);}
  finally {queue.dispose();}
});
test('Autosave retains failed changes and retries instead of marking them saved',async()=>{
  const states=[];let fail=true;const writes=[];
  const queue=new AutosaveQueue(async value=>{if(fail)throw Error('offline');writes.push(value);},state=>states.push(state),60000);
  try {queue.schedule('camera');await assert.rejects(queue.flush(),/offline/);assert.equal(queue.pending,'camera');assert.equal(states.at(-1),'error');fail=false;await queue.flush();assert.deepEqual(writes,['camera']);assert.equal(states.at(-1),'saved');}
  finally {queue.dispose();}
});
