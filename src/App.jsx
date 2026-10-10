import {useEffect,useRef,useState} from 'react';
import GeneratorView from './views/GeneratorView.jsx';
import ProjectHub from './views/ProjectHub.jsx';
import BuilderView from './views/BuilderView.jsx';
import {RemoteSolver} from './terrain/api.js';
import {WorldProjectContext,parseRoute,worldRequest} from './project.js';
import './application.css';

export default function App(){
 const [path,setPath]=useState(location.pathname),[mounted,setMounted]=useState(location.pathname!=='/'),[api,setAPI]=useState(null),[active,setActive]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const [deleteTarget,setDeleteTarget]=useState(null),[libraryVersion,setLibraryVersion]=useState(0);
 const activeRef=useRef(null),routeRef=useRef(parseRoute(path)),pending=useRef(null),builder=useRef(null),newName=useRef('Unnamed World');const route=parseRoute(path);routeRef.current=route;
 function navigate(url,replace=false){history[replace?'replaceState':'pushState']({},'',url);setPath(url);}
 useEffect(()=>{const pop=()=>setPath(location.pathname);window.addEventListener('popstate',pop);return()=>window.removeEventListener('popstate',pop);},[]);
 useEffect(()=>{
  const update=e=>{if(e.detail.solver===activeRef.current)setActive({solver:e.detail.solver,project:e.detail.solver.project});};
  const warn=e=>{if(activeRef.current?.unsaved){e.preventDefault();e.returnValue='';}};
  window.addEventListener('world-save-state',update);window.addEventListener('beforeunload',warn);
  return()=>{window.removeEventListener('world-save-state',update);window.removeEventListener('beforeunload',warn);};
 },[]);
 function received(solver){activeRef.current=solver;setActive({solver,project:solver.project});if(routeRef.current.view==='generate'&&(routeRef.current.fresh||routeRef.current.id!==solver.worldId))navigate(`/world/${solver.worldId}/generate`,true);}
 async function guard(work){setBusy(true);setError('');try{return await work();}catch(e){setError(e.message);}finally{setBusy(false);}}
 async function go(url){await guard(async()=>{await builder.current?.flush();await api?.suspend();if(url!=='/')setMounted(true);navigate(url);});}
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
 const context={...active,changed};
 async function saveWorld(){await guard(async()=>{await builder.current?.flush();await api.save();received(activeRef.current);setLibraryVersion(v=>v+1);});}
 async function deleteWorld(){const target=deleteTarget;if(!target)return;await guard(async()=>{
  const deletingActive=activeRef.current?.worldId===target.id;
  if(deletingActive)await api?.suspend({discard:true});
  await RemoteSolver.deleteProject(target.id);
  if(deletingActive){activeRef.current=null;setActive(null);setMounted(false);setAPI(null);pending.current=null;navigate('/');}
  setDeleteTarget(null);setLibraryVersion(v=>v+1);
 });}
 async function exportWorld(){await guard(async()=>{await builder.current?.flush();const blob=await api.export();if(!blob)throw Error('Open a world first.');const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`${active?.project?.name||'KRIEMHILD'}.world.zip`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});}
 return <WorldProjectContext.Provider value={context}><header className="application-nav"><button className="brand" disabled={busy} onClick={()=>go('/')}>KRIEMHILD <span>/ Projects</span></button>{active&&<><span className="active-world-name">{active.project?.name||'Unnamed World'}</span><nav aria-label="World editors"><button aria-current={route.view==='generate'?'page':undefined} disabled={busy} onClick={()=>go(`/world/${active.solver.worldId}/generate`)}>World Generation</button><button aria-current={route.view==='build'?'page':undefined} disabled={busy} onClick={()=>go(`/world/${active.solver.worldId}/build`)}>World Builder</button><button className="primary" onClick={saveWorld} disabled={busy}>Save</button><button onClick={exportWorld} disabled={busy}>Export ZIP</button><button className="danger" disabled={busy} onClick={()=>setDeleteTarget({id:active.solver.worldId,name:active.project?.name})}>Delete world</button></nav><span className="save-state" role="status">{active.solver.unsaved?'Unsaved changes':'Saved'}</span></>}{busy&&<span role="status">Working…</span>}</header>{error&&<div className="shell-error" role="alert">{error}<button onClick={()=>setError('')}>Dismiss</button></div>}
 {route.view==='home'&&<ProjectHub key={libraryVersion} busy={busy} active={active} onDelete={setDeleteTarget} onCreate={create} onOpen={(id,view)=>go(`/world/${id}/${view}`)} onImport={importFiles}/>}
 {mounted&&<div className="generator-host" hidden={route.view!=='generate'}><GeneratorView onReady={setAPI} onWorld={received}/></div>}
 {route.view==='build'&&active&&route.id===active.solver.worldId&&<BuilderView key={active.solver.id} ref={builder}/>}
 {route.view==='missing'&&<main className="project-hub"><h1>Page not found</h1><button onClick={()=>go('/')}>Back to Projects</button></main>}
 {deleteTarget&&<div className="project-dialog-backdrop"><section className="project-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-world-title" aria-describedby="delete-world-description"><h2 id="delete-world-title">Delete {deleteTarget.name||'this world'}?</h2><p id="delete-world-description">This permanently deletes the world, its map, placed objects and all saved explored terrain. Unsaved changes will also be discarded. Exported ZIP files are unaffected.</p><div><button autoFocus disabled={busy} onClick={()=>setDeleteTarget(null)}>Cancel</button><button className="danger" disabled={busy} onClick={deleteWorld}>Delete permanently</button></div></section></div>}
 </WorldProjectContext.Provider>;
}
