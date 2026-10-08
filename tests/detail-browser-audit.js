// Evaluate in the shared T3 preview. Real API requests; only frame scheduling
// is substituted because the embedded host can pause native animation frames.
(async()=>{
  const $=id=>document.getElementById(id),checks=[],original={raf:window.requestAnimationFrame,caf:window.cancelAnimationFrame};
  const task=fn=>{const c=new MessageChannel();c.port1.onmessage=()=>{c.port1.close();c.port2.close();fn()};c.port2.postMessage(0);return c;};
  window.requestAnimationFrame=fn=>task(()=>fn(performance.now()));window.cancelAnimationFrame=c=>{if(c?.port1){c.port1.close();c.port2.close();}else original.caf.call(window,c);};
  const wait=async(fn,label,ms=15000)=>{const end=Date.now()+ms;while(!fn()&&Date.now()<end)await new Promise(task);if(!fn())throw new Error(label);};
  const change=(id,value)=>{const e=$(id);if(e.type==='checkbox')e.checked=value;else e.value=value;e.dispatchEvent(new Event('change',{bubbles:true}));};
  const assert=(ok,message)=>{if(!ok)throw new Error(message);};
  try{
    change('view3d',false);$('instant').checked=true;
    if(!$('environment').checked){change('environment',true);await wait(()=>$('statusLine').textContent.startsWith('Done'),'Physical mode');}
    $('width').value=160;$('height').value=100;$('generate').click();
    await wait(()=>$('statusLine').textContent.startsWith('Done')&&$('detailMap').dataset.scale,'Map generation');
    $('fitMap').click();await new Promise(task);
    const c=$('detailMap'),r=c.getBoundingClientRect(),clientX=Math.floor(r.left+r.width*.63),clientY=Math.floor(r.top+r.height*.43),px=clientX-r.left,py=clientY-r.top;
    const point=()=>[(px-Number(c.dataset.offsetX))/Number(c.dataset.scale),(py-Number(c.dataset.offsetY))/Number(c.dataset.scale)];
    const start=point();c.dispatchEvent(new WheelEvent('wheel',{clientX,clientY,deltaY:-2100,bubbles:true,cancelable:true}));
    await wait(()=>Number(c.dataset.detail)>4,'Regional detail');
    const end=point();assert(Math.hypot(start[0]-end[0],start[1]-end[1])<1e-8,'2D cursor anchor moved');checks.push('2D cursor anchor is invariant');
    await wait(()=>!$('detailStatus').textContent.includes('refining'),'Detail tiles finish loading',25000);
    assert(Number(c.dataset.tiles)>20,'No refinement tiles');checks.push('Visible detail tiles load without regenerating the parent world');
    c.dispatchEvent(new WheelEvent('wheel',{clientX,clientY,deltaY:2100,bubbles:true,cancelable:true}));await new Promise(task);
    const back=point();assert(Math.hypot(start[0]-back[0],start[1]-back[1])<1e-8,'Zoom return moved geography');checks.push('Zoom out returns to the same geographic point');
    change('view3d',true);await wait(()=>document.querySelector('canvas.view3d')?.dataset.tiles,'3D tiles');
    const view=document.querySelector('canvas.view3d'),vr=view.getBoundingClientRect();
    view.dispatchEvent(new WheelEvent('wheel',{clientX:Math.floor(vr.left+vr.width*.55),clientY:Math.floor(vr.top+vr.height*.52),deltaY:-600,bubbles:true,cancelable:true}));
    await new Promise(task);assert(Number(view.dataset.anchorError)<1e-8,'3D cursor anchor moved');checks.push('3D zoom preserves the raycast terrain point');
    assert(!$('error').textContent,$('error').textContent);
    window.__detailAudit={checks,detail:$('detailStatus').textContent,anchorError:Number(view.dataset.anchorError)};
    return window.__detailAudit;
  }finally{window.requestAnimationFrame=original.raf;window.cancelAnimationFrame=original.caf;}
})()
