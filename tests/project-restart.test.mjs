import {test} from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {spawn,spawnSync} from 'node:child_process';
import {once} from 'node:events';
import {setTimeout as delay} from 'node:timers/promises';

test('World ZIP survives deletion of the original session and a Go server process restart',async()=>{
  const root=path.resolve(import.meta.dirname,'..'),binary=path.join(root,'bin',process.platform==='win32'?'kriemhild-project-test.exe':'kriemhild-project-test');
  const local=path.join(root,'.tools/go/bin',process.platform==='win32'?'go.exe':'go');
  const build=spawnSync(fs.existsSync(local)?local:'go',['build','-o',binary,'./cmd/kriemhild'],{cwd:root,encoding:'utf8',windowsHide:true});
  assert.equal(build.status,0,build.stderr);
  const origin='http://127.0.0.1:18126';let server;
  const launch=async()=>{server=spawn(binary,['-addr','127.0.0.1:18126'],{cwd:root,stdio:'pipe',windowsHide:true});for(let n=0;n<100;n++){try{if((await fetch(origin+'/api/health')).ok)return;}catch{}await delay(50);}throw new Error('Test server did not start');};
  const stop=async()=>{if(server&&server.exitCode===null){const closed=once(server,'exit');server.kill();await closed;}};
  const json=async(route,body)=>{const r=await fetch(origin+'/api/'+route,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const value=await r.json();assert.ok(r.ok,JSON.stringify(value));return value;};
  try{
    await launch();
    const config=JSON.parse(fs.readFileSync(path.join(root,'tests/reference/tiles.json'),'utf8'));
    const initial=await json('sessions',{config,environment:{columns:48,rows:32,seed:'cold-process-project',realism:true}});
    let state=initial;while(state.status==='running')state=(await json(`sessions/${initial.id}/step`,{count:10000})).state;
    assert.equal(state.status,'done');
    const tilePath='/detail/5/12/10';const tile=await fetch(origin+`/api/sessions/${initial.id}`+tilePath).then(r=>r.text());
    const cell=state.dom.findIndex(mask=>mask!==0),type=31-Math.clz32(state.dom[cell]);
    const painted=await json(`sessions/${initial.id}/paint`,{cells:[cell],type});assert.equal(painted.painted,true);
    state=painted.state;while(state.status==='running')state=(await json(`sessions/${initial.id}/step`,{count:10000})).state;
    const saved=await fetch(origin+`/api/sessions/${initial.id}/project`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ui:{seedUsed:123,camera2d:{center:{x:12,y:10},scale:200}}})});
    assert.equal(saved.status,200);const archive=Buffer.from(await saved.arrayBuffer());
    assert.ok(archive.includes(Buffer.from('detail/tiles/5/12/10.json')),'ZIP omitted the explored tile');
    await fetch(origin+`/api/sessions/${initial.id}`,{method:'DELETE'});await stop();await launch();
    const reopened=await fetch(origin+'/api/projects/import',{method:'POST',headers:{'Content-Type':'application/zip'},body:archive});
    const restored=await reopened.json();assert.equal(reopened.status,201,JSON.stringify(restored));
    assert.deepEqual(restored.environment,initial.environment);assert.deepEqual(restored.dom,state.dom);assert.deepEqual(restored.pinned,state.pinned);
    assert.equal(restored.projectUI.camera2d.scale,200);assert.ok(restored.storedDetailCount>=6);
    const restoredTile=await fetch(origin+`/api/sessions/${restored.id}`+tilePath).then(r=>r.text());assert.equal(restoredTile,tile);
    const missing=await fetch(origin+`/api/sessions/${restored.id}/detail/4/2/2`);assert.equal(missing.status,200);
    await fetch(origin+`/api/sessions/${restored.id}`,{method:'DELETE'});
  }finally{await stop();}
});
