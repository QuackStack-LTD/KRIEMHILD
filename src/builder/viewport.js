import {createBuilderMap} from './map.js';

export function createBuilderViewport(host,solver,callbacks){
 const flat=document.createElement('div'),space=document.createElement('div');flat.className=space.className='builder-surface';space.hidden=true;host.append(flat,space);
 let mode=false,disposed=false,scene=null,loading=null,options={},entities=[],selected=null,camera3d=null;
 const map=createBuilderMap(flat,solver,{...callbacks,viewport:b=>{if(!mode)callbacks.viewport(b);},camera:()=>{if(!mode)callbacks.camera(state());}});
 function state(){return {...map.state(),view3d:mode,camera3d:scene?.state()||camera3d};}
 async function setMode(next){
  mode=next;
  if(mode&&!scene){
   loading??=import('./view3d.js');
   try{const {createBuilder3D}=await loading;if(disposed||!mode)return;
    if(!scene){space.hidden=false;scene=createBuilder3D(space,solver,{...callbacks,viewport:b=>{if(mode)callbacks.viewport(b);},camera:()=>{if(mode)callbacks.camera(state());}},map.texture);if(camera3d)scene.restore(camera3d);else scene.focus(map.state());scene.setEntities(entities);scene.select(selected);scene.setOptions(options);}
   }catch(e){mode=false;callbacks.error?.(e.message);}
  }
  flat.hidden=mode;space.hidden=!mode;
  if(!mode)map.refresh();callbacks.camera(state());
 }
 return {setOptions(next){options={...options,...next};map.setOptions(options);scene?.setOptions(options);if(!!next.view3d!==mode)void setMode(!!next.view3d);},setEntities(next){entities=next;map.setEntities(next);scene?.setEntities(next);},select(id){selected=id;map.select(id);scene?.select(id);},invalidate(b){map.invalidate(b);scene?.invalidate(b);},fit(){(mode?scene:map)?.fit();},finish(){(mode?scene:map)?.finish();},state,restore(saved){map.restore(saved);camera3d=saved?.camera3d;},dispose(){disposed=true;map.dispose();scene?.dispose();flat.remove();space.remove();}};
}
