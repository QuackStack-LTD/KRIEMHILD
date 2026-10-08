// Elevation-based hillshading makes continuous ridges legible in the 2D map,
// including snow/forest-covered mountains whose biome is not bare rock.
export function reliefShades(environment,w,h){
  if(!environment)return null;
  const z=environment.fields.elevation,out=new Float32Array(w*h);
  for(let i=0;i<out.length;i++){
    if(environment.fields.waterDepth?.[i]>0){out[i]=Math.max(.48,1.22-Math.log1p(environment.fields.waterDepth[i])*.075);continue}
    const x=i%w,y=Math.floor(i/w);
    const dx=(z[y*w+Math.min(w-1,x+1)]-z[y*w+Math.max(0,x-1)])/1000;
    const dy=(z[Math.min(h-1,y+1)*w+x]-z[Math.max(0,y-1)*w+x])/1000;
    const light=(dx*.5+dy*.5+.7071)/Math.hypot(dx,dy,1);
    const shade=Math.max(.48,Math.min(1.16,.55+.64*light));
    out[i]=shade;
  }
  return out;
}
export function shadePixel(pixel,shade){
  if(shade===1)return pixel;
  const r=Math.min(255,Math.round((pixel&255)*shade));
  const g=Math.min(255,Math.round(((pixel>>>8)&255)*shade));
  const b=Math.min(255,Math.round(((pixel>>>16)&255)*shade));
  return (0xff000000|(b<<16)|(g<<8)|r)>>>0;
}
