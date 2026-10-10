import {entityStyle,entityColor} from '../terrain/feature-style.js';
import {LineSegments2} from 'three/addons/lines/LineSegments2.js';
import {LineSegmentsGeometry} from 'three/addons/lines/LineSegmentsGeometry.js';
import {LineMaterial} from 'three/addons/lines/LineMaterial.js';
import * as THREE from 'three';
import {createView} from '../terrain/view3d.js';
import {drapeSegment} from './geometry.js';
import {detailName} from '../terrain/map-camera.js';

export function createBuilder3D(host,solver,callbacks,texture){
 let view=null,entities=[],selected=null,options={tool:'select',layer:'terrain',layers:[]},revision=0,lastStamp='',lastQuery='',queryTimer=0,down=null,draft=[];
 const objects=new THREE.Group(),abort=new AbortController();
 const dotCanvas=document.createElement('canvas');dotCanvas.width=dotCanvas.height=32;
 const dot=dotCanvas.getContext('2d');dot.fillStyle='#ffffff';dot.beginPath();dot.arc(16,16,15,0,Math.PI*2);dot.fill();
 const dotTexture=new THREE.CanvasTexture(dotCanvas);
 function clear(){for(const child of [...objects.children]){objects.remove(child);child.geometry.dispose();child.material.dispose();}}
 function lifted(x,y){const p=view.surfaceAt(x,y);if(!p)return null;const v=new THREE.Vector3(...p);if(solver.wrapX)v.addScaledVector(v.clone().normalize(),.001);else v.y+=.001;return v;}
 function frame(){
  if(!view)return;
  const viewport=view.viewport(),box=viewport.bounds,detail=viewport.detail;
  const queryKey=[Math.round(box.x*16),Math.round(box.y*16),Math.round(box.width*16),Math.round(box.height*16),Math.floor(detail*4)].join('/');
  if(queryKey!==lastQuery){lastQuery=queryKey;clearTimeout(queryTimer);queryTimer=setTimeout(()=>callbacks.viewport({...box,...(solver.wrapX?{x:0,width:solver.W-1}:{}),detail}),120);}
  const stamp=[view.surfaceVersion(),revision,queryKey].join('/');if(stamp===lastStamp)return;lastStamp=stamp;clear();
  // Globe viewports crossing the meridian include both sides in the query.
  const bounds=solver.wrapX?{x:0,y:Math.max(0,box.y),width:solver.W-1,height:Math.min(solver.H-1,box.y+box.height)-Math.max(0,box.y)}:{x:Math.max(0,box.x),y:Math.max(0,box.y),width:Math.min(solver.W-1,box.x+box.width)-Math.max(0,box.x),height:Math.min(solver.H-1,box.y+box.height)-Math.max(0,box.y)};
  const step=view.surfaceStep();let total=0;
  const draw=entities.concat(draft.length?[{id:'draft',geometry:'line',points:draft,color:'#000000',layer:'',minDetail:0}]:[]);
  for(const e of draw){
   if(e.minDetail>detail||options.layers.find(l=>l.id===e.layer)?.visible===false)continue;
   const positions=[];
   if(e.geometry==='point'){const p=lifted(...e.points[0]);if(p)positions.push(...p.toArray());}
   else {const path=e.geometry==='polygon'?[...e.points,e.points[0]]:e.points;for(let i=1;i<path.length;i++){const samples=drapeSegment(path[i-1],path[i],step,lifted,bounds);for(let j=1;j<samples.length;j++)if(samples[j-1]&&samples[j])positions.push(...samples[j-1].toArray(),...samples[j].toArray());}}
   if(!positions.length)continue;total+=positions.length/3;
   const geometry=e.geometry==='point'?new THREE.BufferGeometry():new LineSegmentsGeometry();if(e.geometry==='point')geometry.setAttribute('position',new THREE.Float32BufferAttribute(positions,3));else geometry.setPositions(positions);
   const color=entityColor(e,e.id===selected);
   const material=e.geometry==='point'?new THREE.PointsMaterial({color,size:entityStyle.pointSize,sizeAttenuation:false,depthWrite:false,map:dotTexture,transparent:true,alphaTest:.1}):new LineMaterial({color,linewidth:entityStyle.lineWidth,depthWrite:false,resolution:new THREE.Vector2(host.clientWidth,host.clientHeight)});
   const object=e.geometry==='point'?new THREE.Points(geometry,material):new LineSegments2(geometry,material);object.userData.entity=e;object.renderOrder=10;objects.add(object);
  }
  view.renderer.domElement.dataset.entityVertices=String(total);
  view.renderer.domElement.dataset.entityCount=String(entities.length);
  callbacks.detail(`${detailName(detail)} · 3D`);
 }
 view=createView(host,()=>callbacks.camera(view?.cameraState()),frame);view.scene.add(objects);
 view.setMap(solver.W,solver.H,texture,{sphere:solver.wrapX,solver,waterFields:solver.environment?.fields});
 const heights=solver.environment?solver.environment.fields.elevation.map(z=>z/2000):Array.from(solver.dom,(_,i)=>solver.config.types[solver.typeAt(i)]?.height||0);
 view.setHeights(heights,1);
 const canvas=view.renderer.domElement;canvas.tabIndex=0;canvas.setAttribute('aria-label','World Builder 3D terrain');
 function listen(name,fn){canvas.addEventListener(name,fn,{signal:abort.signal});}
 listen('pointerdown',e=>{canvas.focus();down={x:e.clientX,y:e.clientY,button:e.button};});
 listen('pointerup',e=>{
  const start=down;down=null;if(!start||start.button!==0||Math.hypot(e.clientX-start.x,e.clientY-start.y)>5)return;
  const p=view.pick(e);if(!p)return;
  if(options.tool==='select'){
   const rect=canvas.getBoundingClientRect(),ray=new THREE.Raycaster();ray.params.Line.threshold=Math.max(.005,8/view.viewport().pixels);ray.params.Points.threshold=ray.params.Line.threshold;
   ray.setFromCamera(new THREE.Vector2((e.clientX-rect.left)/rect.width*2-1,1-(e.clientY-rect.top)/rect.height*2),view.camera);
   const object=ray.intersectObjects(objects.children,false)[0]?.object.userData.entity;
   selected=object?.id||null;callbacks.select(object||null);revision++;
  }else if(options.tool==='point')callbacks.place([[p.x,p.y]],'point');
  else if(['line','polygon','river'].includes(options.tool)){if(!draft.length||Math.hypot(draft.at(-1)[0]-p.x,draft.at(-1)[1]-p.y)>1e-5)draft.push([p.x,p.y]);callbacks.draft(draft.length);revision++;}
  else if(options.tool!=='pan')callbacks.modify(options.tool,p,[],{elevation:(solver.wrapX?p.point.length()-solver.W/(2*Math.PI):p.point.y)*2000});
 });
 function finish(){if(draft.length<(options.tool==='polygon'?3:2))return;const points=draft;draft=[];callbacks.draft(0);revision++;if(options.tool==='river')callbacks.modify('river',{x:points[0][0],y:points[0][1]},points,null);else callbacks.place(points,options.tool);}
 listen('dblclick',finish);listen('keydown',e=>{if(e.key==='Enter'){e.preventDefault();finish();}if(e.key==='Escape'){draft=[];callbacks.draft(0);revision++;}});
 return {
  setOptions(next){if(next.tool!==options.tool){draft=[];callbacks.draft(0);}options={...options,...next};view.controls.enableRotate=['select','pan'].includes(options.tool);view.controls.mouseButtons.LEFT=options.tool==='pan'?THREE.MOUSE.PAN:THREE.MOUSE.ROTATE;view.setDetailLayer(options.layer);revision++;},
  setEntities(next){entities=next;revision++;},select(id){selected=id;revision++;},finish,
  invalidate(bounds){view.invalidate(bounds);revision++;},state:()=>view.cameraState(),restore:s=>view.restoreCamera(s),fit:()=>view.resetCamera(),
  focus(state){if(!state?.center||solver.wrapX)return;const x=(state.center.x/(solver.W-1)-.5)*solver.W,z=(state.center.y/(solver.H-1)-.5)*solver.H,d=host.clientHeight/(2*state.scale*Math.tan(Math.PI/9));view.restoreCamera({position:[x,d*.8,z+d*.8],target:[x,0,z],near:.001});},
  dispose(){clearTimeout(queryTimer);abort.abort();clear();dotTexture.dispose();view.scene.remove(objects);view.dispose();}
 };
}
