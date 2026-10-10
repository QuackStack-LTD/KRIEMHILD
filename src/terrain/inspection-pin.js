// Inspection is UI state, never an authored map object or an undo command.
export function createInspectionPin(host){
 const marker=document.createElement('div');marker.className='inspection-pin';marker.hidden=true;
 marker.setAttribute('role','img');marker.setAttribute('aria-label','Pinned inspection location');host.append(marker);
 return {
  show(x,y){marker.hidden=!Number.isFinite(x)||!Number.isFinite(y);if(!marker.hidden){marker.style.left=`${x}px`;marker.style.top=`${y}px`;}},
  hide(){marker.hidden=true;},dispose(){marker.remove();}
 };
}

// Track the whole gesture: dragging away and back must not count as a click.
export function bindInspectionClick(canvas,enabled,pick,onPin,signal){
 let down=null;
 canvas.addEventListener('pointerdown',e=>{down=e.button===0&&enabled()?{id:e.pointerId,x:e.clientX,y:e.clientY,moved:false}:null;},{signal});
 canvas.addEventListener('pointermove',e=>{if(down&&e.pointerId===down.id)down.moved ||=Math.hypot(e.clientX-down.x,e.clientY-down.y)>5;},{signal});
 canvas.addEventListener('pointercancel',()=>{down=null;},{signal});
 canvas.addEventListener('pointerup',e=>{const start=down;down=null;if(!start||e.pointerId!==start.id||start.moved||!enabled()||Math.hypot(e.clientX-start.x,e.clientY-start.y)>5)return;const point=pick(e);if(point)onPin(point);},{signal});
}
