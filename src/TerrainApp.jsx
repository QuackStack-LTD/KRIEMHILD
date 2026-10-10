import {useEffect,useRef,useState} from 'react';
import GeneratorView from './views/GeneratorView.jsx';
import ProjectHub from './views/ProjectHub.jsx';
import BuilderView from './views/BuilderView.jsx';
import {historyRequest} from './history.js';
import {RemoteSolver} from './terrain/api.js';
import {WorldProjectContext,parseRoute,worldRequest} from './project.js';
import './application.css';

export default function TerrainApp({onExit}){
 const root=useRef(new URLSearchParams(location.search));
 const rootWorld=()=>activeRef.current?.historyWorldId||root.current.get('world');
 const rootAge=()=>activeRef.current?.historicalMap?.ageId||root.current.get('age');
 const [path,setPath]=useState(location.pathname),[mounted,setMounted]=useState(location.pathname!=='/'),[api,setAPI]=useState(null),[active,setActive]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const [deleteTarget,setDeleteTarget]=useState(null),[libraryVersion,setLibraryVersion]=useState(0);
 const activeRef=useRef(null),routeRef=useRef(parseRoute(path)),pending=useRef(null),builder=useRef(null),newName=useRef('Unnamed World');const route=parseRoute(path);routeRef.current=route;
 function navigate(url,replace=false){if(url==='/'){onExit?.(rootWorld());return;}const query=rootWorld()?`?world=${rootWorld()}&age=${rootAge()}`:'';history[replace?'replaceState':'pushState']({},'',url+query);setPath(url);}
 useEffect(()=>{const pop=()=>setPath(location.pathname);window.addEventListener('popstate',pop);return()=>window.removeEventListener('popstate',pop);},[]);
 useEffect(()=>{
  const update=e=>{if(e.detail.solver===activeRef.current)setActive({solver:e.detail.solver,project:e.detail.solver.project});};
  const warn=e=>{if(activeRef.current?.unsaved){e.preventDefault();e.returnValue='';}};
  window.addEventListener('world-save-state',update);window.addEventListener('beforeunload',warn);
  return()=>{window.removeEventListener('world-save-state',update);window.removeEventListener('beforeunload',warn);};
 },[]);
 function received(solver){activeRef.current=solver;setActive({solver,project:solver.project});if(routeRef.current.view==='generate'&&(routeRef.current.fresh||routeRef.current.id!==solver.worldId))navigate(`/world/${solver.worldId}/generate`,true);}
 async function guard(work){setBusy(true);setError('');try{return await work();}catch(e){setError(e.message);}finally{setBusy(false);}}
 async function go(url){await guard(async()=>{await builder.current?.flush();if(url==='/'&&activeRef.current?.unsaved&&!confirm('Leave without saving your map changes?'))return;await api?.suspend();if(url!=='/')setMounted(true);navigate(url);});}
 function create(name){newName.current=name;void go('/world/new');}
 function importFiles(files,folder){void guard(async()=>{await builder.current?.flush();await api?.suspend();pending.current={files,folder};setMounted(true);navigate('/world/new');});}
 useEffect(()=>{if(!api)return;let cancelled=false;void guard(async()=>{
  if(route.view==='home'){await api.suspend();return;}
  if(route.view==='missing')return;
  let solver=activeRef.current;
  if(route.fresh){
   if(pending.current){const item=pending.current;pending.current=null;solver=await api.import(item.files,item.folder);}
   else {solver=await api.generate();if(solver){const updated=await worldRequest(solver.id,'',{kind:'metadata',revision:solver.project?.revision||0,name:newName.current,description:''});solver.project=updated.project;received(solver);}}
  }else if(!solver||solver.worldId!==route.id){solver=await api.open(route.id);}
  if(cancelled)return;
  if(!solver){const previous=activeRef.current;navigate(previous?`/world/${previous.worldId}/generate`:'/',true);return;}
  if(route.view==='build'){await api.suspend();const state=await solver.refreshWorld();solver.apply(state);}else if(!route.fresh){await api.resume();}
  received(solver);
 });return()=>{cancelled=true;};},[api,path]);
 function changed(project,dirty){const solver=activeRef.current;if(!solver)return;solver.project=project;setActive({solver,project});if(dirty)api?.invalidate(dirty);}
 async function acceptTerrain(){return guard(async()=>{
  await builder.current?.flush();let worldID=rootWorld(),ageID=rootAge();
  if(!worldID){const name=prompt('Name the World that will contain this terrain',activeRef.current.project.name);if(!name)return;const d=await historyRequest('',{name});worldID=d.world.id;ageID=d.world.currentAge;root.current.set('world',worldID);root.current.set('age',ageID);}
  const name=prompt('Terrain name',activeRef.current.project.name==='Unnamed World'?'First terrain':activeRef.current.project.name);if(!name)return;
  await historyRequest(`/${worldID}/accept`,{session:activeRef.current.id,ageId:ageID,name});activeRef.current.apply(await activeRef.current.refreshWorld());received(activeRef.current);await api.suspend();navigate(`/world/${activeRef.current.worldId}/build`);
 });}
 async function openDetail(entity,bounds){await guard(async()=>{await builder.current?.flush();await api.save();const d=await historyRequest(`/${rootWorld()}`),m=activeRef.current.historicalMap;if(!m)throw Error('Accept this terrain into a World first.');const name=prompt('Settlement map name',entity?`${entity.name} - detailed map`:'Settlement map');if(!name)return;const result=await historyRequest(`/${rootWorld()}/child`,{ageId:m.ageId,mapId:m.id,entityId:entity?.id||'',name,bounds,revision:d.world.revision});await api.suspend();navigate(`/world/${result.map.projectId}/build`);});}
 async function openParent(){await guard(async()=>{const m=activeRef.current.historicalMap;if(!m?.parentId)return;if(activeRef.current.unsaved&&!confirm('Return to parent without saving these changes?'))return;await builder.current?.flush();const d=await historyRequest(`/${rootWorld()}`),parent=d.maps.find(p=>p.ageId===m.ageId&&p.id===m.parentId);await api.suspend();navigate(`/world/${parent.projectId}/build`);});}
 const context={...active,changed,openDetail,openParent};
 async function saveWorld(){if(rootWorld()&&!activeRef.current?.historicalMap){await acceptTerrain();return;}await guard(async()=>{await builder.current?.flush();await api.save();received(activeRef.current);setLibraryVersion(v=>v+1);});}
 async function deleteWorld(){const target=deleteTarget;if(!target)return;await guard(async()=>{
  const deletingActive=activeRef.current?.worldId===target.id;
  if(deletingActive)await api?.suspend({discard:true});
  await RemoteSolver.deleteProject(target.id);
  if(deletingActive){activeRef.current=null;setActive(null);setMounted(false);setAPI(null);pending.current=null;navigate('/');}
  setDeleteTarget(null);setLibraryVersion(v=>v+1);
 });}
 async function exportWorld(){await guard(async()=>{await builder.current?.flush();let blob;if(rootWorld()&&activeRef.current?.historicalMap){await api.save();const r=await fetch(`/api/worlds/${rootWorld()}/export`);if(!r.ok)throw Error((await r.json()).error);blob=await r.blob();}else blob=await api.export();if(!blob)throw Error('Open a world first.');const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`${active?.project?.name||'KRIEMHILD'}.world.zip`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});}
 return <WorldProjectContext.Provider value={context}><header className="application-nav"><button className="brand" disabled={busy} onClick={()=>go('/')}>KRIEMHILD <span>/ {rootWorld()?'World':'Worlds'}</span></button>{active&&<><span className="active-world-name">{active.project?.name||'Unnamed World'}</span><nav aria-label="World editors"><button aria-current={route.view==='generate'?'page':undefined} disabled={busy} onClick={()=>go(`/world/${active.solver.worldId}/generate`)}>Terrain Generator</button><button aria-current={route.view==='build'?'page':undefined} disabled={busy} onClick={()=>active.solver.historicalMap?go(`/world/${active.solver.worldId}/build`):acceptTerrain()}>{active.solver.historicalMap?'Map Builder':rootWorld()?'Accept terrain & Build':'Move into a World'}</button><button className="primary" onClick={saveWorld} disabled={busy}>Save</button><button onClick={exportWorld} disabled={busy}>Export ZIP</button>{!active.solver.historicalMap&&<button className="danger" disabled={busy} onClick={()=>setDeleteTarget({id:active.solver.worldId,name:active.project?.name})}>Delete terrain</button>}</nav><span className="save-state" role="status">{active.solver.unsaved?'Unsaved changes':'Saved'}</span></>}{busy&&<span role="status">Working…</span>}</header>{error&&<div className="shell-error" role="alert">{error}<button onClick={()=>setError('')}>Dismiss</button></div>}
 {route.view==='generate'&&root.current.get('legacyImport')&&<label className="legacy-import">Open legacy terrain ZIP <input type="file" accept=".zip" onChange={e=>importFiles(e.target.files,false)}/></label>}
 {route.view==='home'&&<ProjectHub key={libraryVersion} busy={busy} active={active} onDelete={setDeleteTarget} onCreate={create} onOpen={(id,view)=>go(`/world/${id}/${view}`)} onImport={importFiles}/>}
 {mounted&&<div className="generator-host" hidden={route.view!=='generate'}><GeneratorView onReady={setAPI} onWorld={received}/></div>}
 {route.view==='build'&&active&&route.id===active.solver.worldId&&<BuilderView key={active.solver.id} ref={builder}/>}
 {route.view==='missing'&&<main className="project-hub"><h1>Page not found</h1><button onClick={()=>go('/')}>Back to Projects</button></main>}
 {deleteTarget&&<div className="project-dialog-backdrop"><section className="project-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-world-title" aria-describedby="delete-world-description"><h2 id="delete-world-title">Delete {deleteTarget.name||'this world'}?</h2><p id="delete-world-description">This permanently deletes the world, its map, placed objects and all saved explored terrain. Unsaved changes will also be discarded. Exported ZIP files are unaffected.</p><div><button autoFocus disabled={busy} onClick={()=>setDeleteTarget(null)}>Cancel</button><button className="danger" disabled={busy} onClick={deleteWorld}>Delete permanently</button></div></section></div>}
 </WorldProjectContext.Provider>;
}
