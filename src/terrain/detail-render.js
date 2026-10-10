import {biomeColor,coarseMaterial,materialSeed} from './biome-material.js';
import {riverStyle} from './feature-style.js';
import {fieldColor,riverSystemColor} from './environment-view.js';
import {smooth} from './map-camera.js';

const clamp=(x,a,b)=>Math.max(a,Math.min(b,x));
export function makeDetailPalette(solver,layer='terrain'){
  const ctx=document.createElement('canvas').getContext('2d',{willReadFrequently:true}),cache=new Map();
  const seed=materialSeed(solver.environment?.options?.seed||0),materials={};
  for(const type of solver.config.types){const key=type.environmentType||type.id;if(!['grass','forest','desert','mountain','tundra','snow','sand','swamp'].includes(key))continue;ctx.fillStyle=type.color;ctx.fillRect(0,0,1,1);materials[key]=[...ctx.getImageData(0,0,1,1).data].slice(0,3);}
  const palette=Array.from({length:solver.N},(_,i)=>{
    if(layer==='terrain'&&!solver.pinned?.[i]&&solver.environment?.fields.waterBody[i]===0)return biomeColor(coarseMaterial(solver.environment,i),materials,{x:i%solver.W,y:Math.floor(i/solver.W),seed});
    const type=solver.config.types[solver.typeAt(i)];const css=layer==='terrain'?(type?.color||'#7c9864'):(fieldColor(solver.environment,layer,i)||'#637b83');
    if(cache.has(css))return cache.get(css);ctx.fillStyle=css;ctx.fillRect(0,0,1,1);const rgb=[...ctx.getImageData(0,0,1,1).data].slice(0,3);cache.set(css,rgb);return rgb;
  });
  palette.materials=materials;palette.seed=seed;return palette;
}

function baseColor(solver,palette,x,y,wet){
  const w=solver.W,h=solver.H;x=clamp(x,0,w-1);y=clamp(y,0,h-1);const x0=Math.floor(x),y0=Math.floor(y),a=x-x0,b=y-y0;
  const rgb=[0,0,0];let total=0;
  for(const [dx,dy,weight] of [[0,0,(1-a)*(1-b)],[1,0,a*(1-b)],[0,1,(1-a)*b],[1,1,a*b]]){
    const i=Math.min(h-1,y0+dy)*w+Math.min(w-1,x0+dx);
    if(wet!==null&&(solver.environment.fields.waterBody[i]>0)!==wet)continue;
    total+=weight;for(let k=0;k<3;k++)rgb[k]+=palette[i][k]*weight;
  }
  return total>0?rgb.map(c=>c/total):palette[Math.round(y)*w+Math.round(x)];
}

export function tileImage(tile,solver,palette,layer,blend=1){
  const canvas=document.createElement('canvas');canvas.width=canvas.height=layer==='terrain'?65:tile.size;
  const ctx=canvas.getContext('2d'),image=ctx.createImageData(canvas.width,canvas.height),n=canvas.width;
  const height=i=>tile.points[i].parent+(tile.points[i].elevation-tile.points[i].parent)*blend;
  const samples=tile.points.map(p=>({parent:p.parent??0,elevation:p.elevation??0,waterDepth:p.waterDepth??0,waterLevel:p.waterLevel??0,temperature:p.temperature??15,moisture:p.moisture??.5,vegetation:p.vegetation??.5,rock:p.rock??0,sand:p.sand??0,snow:p.snow??0,wetland:p.wetland??0,floodplain:p.floodplain??0,riverDepth:p.riverDepth??0}));
  const p={},position={x:0,y:0,seed:palette.seed||0};
  const gradients=tile.points.map((v,index)=>{
    if(v.gradient)return [v.gradient[2]+(v.gradient[0]-v.gradient[2])*blend,v.gradient[3]+(v.gradient[1]-v.gradient[3])*blend];
    const i=index%tile.size,j=Math.floor(index/tile.size);
    return [(height(j*tile.size+Math.min(tile.size-1,i+1))-height(j*tile.size+Math.max(0,i-1)))/(tile.step*2),(height(Math.min(tile.size-1,j+1)*tile.size+i)-height(Math.max(0,j-1)*tile.size+i))/(tile.step*2)];
  });
  for(let j=0;j<n;j++)for(let i=0;i<n;i++){
    const at=j*n+i,u=i*(tile.size-1)/(n-1),v=j*(tile.size-1)/(n-1),a=u-Math.floor(u),b=v-Math.floor(v),x=(tile.x*32+u)*tile.step,y=(tile.y*32+v)*tile.step;
    const left=Math.floor(u),top=Math.floor(v),right=Math.min(tile.size-1,left+1),bottom=Math.min(tile.size-1,top+1);
    const indices=[top*tile.size+left,top*tile.size+right,bottom*tile.size+left,bottom*tile.size+right],weights=[(1-a)*(1-b),a*(1-b),(1-a)*b,a*b];
    const a0=samples[indices[0]],a1=samples[indices[1]],a2=samples[indices[2]],a3=samples[indices[3]],w0=weights[0],w1=weights[1],w2=weights[2],w3=weights[3];
    p.parent=a0.parent*w0+a1.parent*w1+a2.parent*w2+a3.parent*w3;
    p.elevation=a0.elevation*w0+a1.elevation*w1+a2.elevation*w2+a3.elevation*w3;
    p.waterDepth=a0.waterDepth*w0+a1.waterDepth*w1+a2.waterDepth*w2+a3.waterDepth*w3;
    p.waterLevel=a0.waterLevel*w0+a1.waterLevel*w1+a2.waterLevel*w2+a3.waterLevel*w3;
    p.temperature=a0.temperature*w0+a1.temperature*w1+a2.temperature*w2+a3.temperature*w3;
    p.moisture=a0.moisture*w0+a1.moisture*w1+a2.moisture*w2+a3.moisture*w3;
    p.vegetation=a0.vegetation*w0+a1.vegetation*w1+a2.vegetation*w2+a3.vegetation*w3;
    p.rock=a0.rock*w0+a1.rock*w1+a2.rock*w2+a3.rock*w3;
    p.sand=a0.sand*w0+a1.sand*w1+a2.sand*w2+a3.sand*w3;
    p.snow=a0.snow*w0+a1.snow*w1+a2.snow*w2+a3.snow*w3;
    p.wetland=a0.wetland*w0+a1.wetland*w1+a2.wetland*w2+a3.wetland*w3;
    p.floodplain=a0.floodplain*w0+a1.floodplain*w1+a2.floodplain*w2+a3.floodplain*w3;
    p.riverDepth=a0.riverDepth*w0+a1.riverDepth*w1+a2.riverDepth*w2+a3.riverDepth*w3;
    let wetCoverage=0,waterWeight=0;p.waterBody=0;
    for(let k=0;k<4;k++){const body=tile.points[indices[k]].waterBody;if(body>0){wetCoverage+=weights[k];if(weights[k]>waterWeight){p.waterBody=body;waterWeight=weights[k];}}}if(wetCoverage<.5)p.waterBody=0;
    position.x=x;position.y=y;
    let rgb=layer==='terrain'?null:baseColor(solver,palette,x,y,null);
    if(layer==='terrain'){
      if(p.waterBody>0){const deep=clamp(Math.log1p(p.waterDepth)/9,0,1);rgb=[45-25*deep,140-96*deep,175-91*deep];}
      else {
        const physical=biomeColor(p,palette.materials,position);
        // The same material function is used at every scale. Parent blending
        // controls geometry; there is no hard biome-dependent height reset.
        rgb=physical;
        // Authored biome paint is also categorical, with no feathered color mix.
        if(solver.pinned){const cell=Math.max(0,Math.min(solver.H-1,Math.round(y)))*solver.W+Math.max(0,Math.min(solver.W-1,Math.round(x)));if(solver.pinned[cell])rgb=[...palette[cell]];}
        if(tile.level>=3&&p.riverDepth>.3)rgb=[57,127,158];
      }
      let dx=0,dy=0;for(let k=0;k<4;k++){dx+=gradients[indices[k]][0]*weights[k];dy+=gradients[indices[k]][1]*weights[k];}
      // A bounded normal response keeps deep relief readable without clipping
      // every ridge to black/white at local scales.
      const normal=Math.sqrt(1+(dx*dx+dy*dy)/1600000);
      const shade=clamp(.55+(.65+(dx-dy)/2400)/normal,.42,1.24);
      rgb=rgb.map(c=>c*shade);
    }else if(layer==='riverClass'||layer==='riverSystem'){
      // Draw diagnostic channels as connected geometry over a quiet terrain
      // silhouette, not as enlarged colored raster cells.
      const high=clamp(p.elevation/4000,0,1)*20;
      rgb=p.waterBody>0?[19,43,58]:[34+high,49+high,43+high];
    }else if(layer==='elevation'||layer==='bathymetry'||layer==='waterDepth'){
      const value=layer==='elevation'?(p.parent+(p.elevation-p.parent)*blend+12000)/20000:p.waterDepth/12000,t=clamp(value,0,1);
      rgb=[40+200*t,80+100*(1-Math.abs(t-.5)*2),200-160*t];
    }
    const index=at*4;for(let k=0;k<3;k++)image.data[index+k]=rgb[k];image.data[index+3]=255;
  }
  ctx.putImageData(image,0,0);return canvas;
}

export function drawDetailFeatures(ctx,tiles,camera,detail,layer){
  if(!['terrain','river','accumulation','watershed','riverClass','riverSystem'].includes(layer))return;
  const debug=['riverClass','riverSystem'].includes(layer);
  const seen=new Set();ctx.save();
  for(const {tile,arrival} of tiles)for(const f of tile.features){
    if(seen.has(f.id))continue;seen.add(f.id);
    const alpha=(debug?1:smooth(detail-f.level+1))*arrival;if(alpha<=0)continue;
    if(!['river','canal'].includes(f.kind)||f.path.length<2)continue;
    ctx.globalAlpha=alpha;const path=f.path.map(p=>[camera.x+p[0]*camera.scale,camera.y+p[1]*camera.scale]);
    // Variable-width ribbons meet at shared hydraulic junction widths. Banks
    // become readable at regional scale; tiny streams remain surface lines.
    const ribbon=(factor,color,minWidth)=>{
      const left=[],right=[];
      for(let i=0;i<path.length;i++){
        const a=path[Math.max(0,i-1)],b=path[Math.min(path.length-1,i+1)],len=Math.hypot(b[0]-a[0],b[1]-a[1])||1;
        const radius=Math.max(minWidth,(f.widths?.[i]??f.width)*camera.scale*factor)/2;
        const nx=-(b[1]-a[1])/len*radius,ny=(b[0]-a[0])/len*radius;
        left.push([path[i][0]+nx,path[i][1]+ny]);right.push([path[i][0]-nx,path[i][1]-ny]);
      }
      ctx.fillStyle=color;ctx.beginPath();[...left,...right.reverse()].forEach((p,i)=>i?ctx.lineTo(...p):ctx.moveTo(...p));ctx.closePath();ctx.fill();
    };
    if(debug){
      let color=f.class==='major'?'#ffcd78':f.class==='regional'?'#64d4ad':'#609bc1';
      if(layer==='riverSystem')color=riverSystemColor(f.riverId||f.id);
      ribbon(1,color,f.class==='major'?2.4:1.2);
      const a=path[0],b=path[path.length-1],length=Math.hypot(b[0]-a[0],b[1]-a[1]);
      if(length>14){const k=Math.floor(path.length*.65),p=path[k],q=path[Math.max(0,k-2)],angle=Math.atan2(p[1]-q[1],p[0]-q[0]);ctx.fillStyle=color;ctx.beginPath();ctx.moveTo(...p);ctx.lineTo(p[0]-Math.cos(angle-.5)*5,p[1]-Math.sin(angle-.5)*5);ctx.lineTo(p[0]-Math.cos(angle+.5)*5,p[1]-Math.sin(angle+.5)*5);ctx.closePath();ctx.fill();}
      continue;
    }
    if(detail>2&&f.discharge>=2){ctx.globalAlpha=alpha*smooth(detail-2)*.28;ribbon(1.35,riverStyle.bank,0);}
    ctx.globalAlpha=alpha*(f.regime==='seasonal'?.75:1);ribbon(1,f.regime==='ephemeral'?'#978569':riverStyle.water,Math.min(1.6,.8+Math.sqrt(f.discharge||0)*.03));
    if(detail>4){ctx.globalAlpha=alpha*.2;ribbon(.55,riverStyle.highlight,0);}
  }
  ctx.restore();
}

export function featureTexture(tile,solver,palette,layer,blend,detail){
  const base=tileImage(tile,solver,palette,layer,blend),canvas=document.createElement('canvas');canvas.width=canvas.height=256;
  const ctx=canvas.getContext('2d');ctx.imageSmoothingEnabled=false;ctx.drawImage(base,.5,.5,base.width-1,base.height-1,0,0,256,256);
  const span=32*tile.step;
  drawDetailFeatures(ctx,[{tile,arrival:1}],{x:-tile.x*256,y:-tile.y*256,scale:256/span},detail,layer);
  return canvas;
}

export function drawDetailOverlay(ctx,solver,camera,layer,width,height){
  const f=solver.environment.fields;
  ctx.save();
  if(layer==='wind'){
    const step=Math.max(1,Math.ceil(32/camera.scale));ctx.strokeStyle='#e7f1f5';ctx.lineWidth=1;
    const minX=Math.max(0,Math.floor(-camera.x/camera.scale/step)*step),minY=Math.max(0,Math.floor(-camera.y/camera.scale/step)*step);
    for(let y=minY;y<solver.H&&camera.y+y*camera.scale<height;y+=step)for(let x=minX;x<solver.W&&camera.x+x*camera.scale<width;x+=step){
      const i=y*solver.W+x,px=camera.x+x*camera.scale,py=camera.y+y*camera.scale,a=Math.atan2(f.windY[i],f.windX[i]),length=Math.min(22,camera.scale*step*.7),ex=px+Math.cos(a)*length,ey=py+Math.sin(a)*length;
      ctx.beginPath();ctx.moveTo(px,py);ctx.lineTo(ex,ey);ctx.lineTo(ex-Math.cos(a-.6)*4,ey-Math.sin(a-.6)*4);ctx.moveTo(ex,ey);ctx.lineTo(ex-Math.cos(a+.6)*4,ey-Math.sin(a+.6)*4);ctx.stroke();
    }
  }
  if(document.getElementById('contShow')?.checked)for(const p of solver.contPoints||[]){const x=camera.x+p.x*camera.scale,y=camera.y+p.y*camera.scale;ctx.fillStyle=p.kind?.color||'#e3b978';ctx.beginPath();ctx.arc(x,y,5,0,Math.PI*2);ctx.fill();}
  ctx.restore();
}
