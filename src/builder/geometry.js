export function allowedGeometry(layer,kind){
 if(kind==='custom-structure')return ['point','line','polygon'];
 if(kind==='factory'||kind==='landmark')return ['point'];
 if(['village','town','city','capital','custom-settlement'].includes(kind))return ['polygon'];
 if(['road','highway','bridge','tunnel','railway','port','dam','canal'].includes(kind))return ['line'];
 if(['house','castle','fortification','temple','administrative-building'].includes(kind))return ['point','polygon'];
 if(layer==='settlements')return ['polygon'];
 if(layer==='roads')return ['line'];
 if(layer==='buildings')return ['point','polygon'];
 return ['point','line','polygon'];
}

// Split straight map edges at every terrain grid and triangle boundary. Each
// resulting 3D segment lies on one actual terrain triangle, including valleys.
export function drapeSegment(a,b,step,sample,bounds){
 let lo=0,hi=1;const dx=b[0]-a[0],dy=b[1]-a[1];
 if(bounds){for(const [p,q] of [[-dx,a[0]-bounds.x],[dx,bounds.x+bounds.width-a[0]],[-dy,a[1]-bounds.y],[dy,bounds.y+bounds.height-a[1]]]){if(p===0){if(q<0)return [];}else if(p<0)lo=Math.max(lo,q/p);else hi=Math.min(hi,q/p);}if(lo>hi)return [];}
 const cuts=[lo,hi];
 for(const [start,delta] of [[a[0],dx],[a[1],dy],[a[0]+a[1],dx+dy]]){
  if(Math.abs(delta)<1e-12)continue;
  const v0=start+delta*lo,v1=start+delta*hi;
  for(let k=Math.ceil(Math.min(v0,v1)/step);k<=Math.floor(Math.max(v0,v1)/step);k++){const t=(k*step-start)/delta;if(t>lo+1e-10&&t<hi-1e-10)cuts.push(t);}
 }
 cuts.sort((a,b)=>a-b);const points=[];
 for(let i=0;i<cuts.length;i++){if(i&&cuts[i]-cuts[i-1]<1e-10)continue;const t=cuts[i],p=sample(a[0]+dx*t,a[1]+dy*t);points.push(p);}
 return points;
}
