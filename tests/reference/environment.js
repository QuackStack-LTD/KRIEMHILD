// Environmental terrain pipeline v1. Pure, seeded JavaScript; shared with KRIEMHILD.
// Procedural Earth-like relationships, not a numerical Earth-system simulation.
(function (global) {
  'use strict';
  const VERSION = 'terraingen-environment-v1';
  const clamp = (v, lo = 0, hi = 1) => Math.max(lo, Math.min(hi, v));
  const smooth = v => { v = clamp(v); return v * v * (3 - 2 * v); };
  const FIELD_NAMES = ['elevation','bathymetry','slope','oceanDistance','latitude','temperature','summer','winter','current','windX','windY','windStrength','precipitation','moisture','aridity','plate','boundary','tectonicStress','volcano','flow','accumulation','groundwater','sediment','fertility','snow','glacier','dune','duneOrientation','reef','salinity','clarity','mesa','wetland','oasis','farmland','settlement','lava','ash','lake'];
  function neighbors(i, w, h) {
    const a = [], x = i % w, y = Math.floor(i / w);
    if (x) a.push(i - 1); if (x < w - 1) a.push(i + 1);
    if (y) a.push(i - w); if (y < h - 1) a.push(i + w);
    return a;
  }
  function noise(seed) {
    const hash = (x, y) => {
      let a = Math.imul(x + 17, 374761393) ^ Math.imul(y + 31, 668265263) ^ seed;
      a = Math.imul(a ^ (a >>> 13), 1274126177); return ((a ^ (a >>> 16)) >>> 0) / 4294967295;
    };
    return (x, y) => {
      const ix = Math.floor(x), iy = Math.floor(y), dx = smooth(x - ix), dy = smooth(y - iy);
      return (hash(ix, iy) * (1-dx) + hash(ix+1, iy)*dx)*(1-dy) + (hash(ix, iy+1)*(1-dx)+hash(ix+1,iy+1)*dx)*dy;
    };
  }
  function distance(mask, w, h, value) {
    const d = new Float32Array(w*h).fill(w+h), q = new Int32Array(w*h); let tail = 0;
    for (let i=0;i<d.length;i++) if (mask[i] === value) { d[i]=0; q[tail++]=i; }
    for (let head=0;head<tail;head++) { const i=q[head]; for(const j of neighbors(i,w,h)) if(d[j]>d[i]+1){d[j]=d[i]+1;q[tail++]=j;} }
    return d;
  }
  function settings(input) {
    const o = Object.assign({columns:96,rows:64,seed:'Kriemhild',landPercent:42,ruggedness:100,plateCount:12,continentCount:12,temperatureOffset:0,rainfall:1,latitudeNorth:90,latitudeSouth:-90,windDirection:0,stability:3,selection:'entropy',cleanup:2}, input);
    for (const [k,lo,hi] of [['columns',16,256],['rows',16,256],['landPercent',1,85],['ruggedness',1,200],['plateCount',3,32],['continentCount',2,40],['temperatureOffset',-25,25],['rainfall',0.1,3],['latitudeNorth',-90,90],['latitudeSouth',-90,90],['windDirection',-1,1],['stability',0,10],['cleanup',0,10]]) {
      if (!Number.isFinite(o[k]) || o[k]<lo || o[k]>hi) throw new Error('Invalid environmental setting: '+k);
    }
    if (!Number.isInteger(o.columns)||!Number.isInteger(o.rows)||o.columns*o.rows>24576) throw new Error('Choose at most 24,576 terrain cells.');
    if(o.latitudeNorth<=o.latitudeSouth) throw new Error('North latitude must exceed south latitude.');
    if(String(o.seed).length>300) throw new Error('Seed is too long.');
    return o;
  }
  function build(input, config) {
    const o=settings(input), w=o.columns,h=o.rows,n=w*h, seed=global.TerrainWFC.seedFromString(o.seed);
    const rng=global.TerrainWFC.makeRng(seed^0x738ac52), sample=noise(seed), f={};
    FIELD_NAMES.forEach(k=>f[k]=new Float32Array(n)); f.flow.fill(-1);
    const margin=Math.max(2,Math.ceil(Math.min(w,h)*0.045)), mask=new Uint8Array(n), heights=new Int32Array(n);
    // Reuse TerrainGenOnSteroids' inverse-square continental influence, restricted
    // to land/water. Climate and volcanoes are no longer continental labels.
    const macroRules=global.TerrainWFC.compileRules({continents:[{id:'water',odds:0.5},{id:'land',odds:0.5}],types:[{id:'water',neighbors:['water','land'],continent:'water'},{id:'land',neighbors:['water','land'],continent:'land'}]});
    const macro={rules:macroRules,W:w,H:h,N:n,Z:0,wrapX:false};
    global.TerrainWFC.Solver.prototype._buildContinents.call(macro,{count:o.continentCount,strength:5},seed);
    const scores=new Float32Array(n), sorted=[];
    for(let i=0;i<n;i++) {
      const x=i%w,y=Math.floor(i/w),edge=Math.min(x,y,w-1-x,h-1-y);
      // Rounded, noisy falloff closes coasts naturally instead of making an
      // inset rectangular continent that looks like a cropped mainland.
      const radial=Math.hypot((x-(w-1)/2)/(w/2-margin),(y-(h-1)/2)/(h/2-margin));
      const envelope=smooth((1.08-radial+(sample(x/w*9+13,y/h*9)-.5)*.14)/.26);
      scores[i]=macro.contShare[i*2+1]*0.65+sample(x/w*7,y/h*7)*0.25+sample(x/w*20,y/h*20)*.1-(1-envelope)*1.2;
      if(edge>margin) sorted.push(scores[i]);
    }
    sorted.sort((a,b)=>a-b);
    const landCount=Math.min(sorted.length-1,Math.round(n*o.landPercent/100));
    const cutoff=sorted[Math.max(0,sorted.length-landCount-1)];
    for(let i=0;i<n;i++)mask[i]=scores[i]>cutoff?1:0;
    const plates=Array.from({length:o.plateCount},()=>({x:rng()*w,y:rng()*h,vx:rng()*2-1,vy:rng()*2-1,continental:rng()>0.45}));
    const hotspots=Array.from({length:3},()=>({x:(0.15+rng()*0.7)*w,y:(0.15+rng()*0.7)*h}));
    const landDist=distance(mask,w,h,1), oceanDist=distance(mask,w,h,0);
    for(let i=0;i<n;i++) {
      const x=i%w,y=Math.floor(i/w); let a=0,b=1,da=Infinity,db=Infinity;
      for(let p=0;p<plates.length;p++){const d=(x-plates[p].x)**2+(y-plates[p].y)**2;if(d<da){b=a;db=da;a=p;da=d;}else if(d<db){b=p;db=d;}}
      const pa=plates[a],pb=plates[b],len=Math.hypot(pb.x-pa.x,pb.y-pa.y)||1;
      const convergence=((pa.vx-pb.vx)*(pb.x-pa.x)+(pa.vy-pb.vy)*(pb.y-pa.y))/len;
      const proximity=Math.exp(-(((Math.sqrt(db)-Math.sqrt(da))/2.8)**2));
      f.plate[i]=a; f.boundary[i]=proximity>0.3?(convergence>0.25?1:convergence<-.25?2:3):0;
      f.tectonicStress[i]=clamp(convergence)*proximity;
      // Collision mountains, subduction arcs, oceanic divergence and hotspots.
      const subduction=convergence>0.25 && (!pa.continental||!pb.continental);
      const divergent=convergence<-.25;
      let hot=0;for(const p of hotspots)hot=Math.max(hot,Math.exp(-((x-p.x)**2+(y-p.y)**2)/8));
      f.volcano[i]=clamp(Math.max(hot,proximity*(subduction?0.8:divergent?0.5:0)));
      const rough=sample(x/w*24,y/h*24),relief=(0.25+rough*.75)*o.ruggedness/100;
      const inland=smooth(oceanDist[i]/4);
      let elevation=mask[i] ? 20+inland*(200+Math.max(0,scores[i]-cutoff)*1700+f.tectonicStress[i]*5200*relief+hot*3500) : -(12+Math.max(0,landDist[i]-1)**2*115+sample(x/w*5,y/h*5)*landDist[i]*90);
      if(!mask[i]) elevation+=Math.min(-elevation-8,proximity*(divergent?900:subduction?-1400:0));
      if(input.heights) elevation=Number(input.heights[i])*4-Number(input.sea||0)*4;
      if(Math.min(x,y,w-1-x,h-1-y)<=margin) elevation=Math.min(-100,elevation);
      heights[i]=Math.round(clamp(elevation/4,-2000,2000));
      if(mask[i] && !input.heights && Math.min(x,y,w-1-x,h-1-y)>margin) heights[i]=Math.max(1,heights[i]);
      f.elevation[i]=heights[i]*4;mask[i]=heights[i]>0?1:0;
      f.bathymetry[i]=Math.max(0,-f.elevation[i]);
    }
    // All distance/slope/climate fields follow the accepted continuous geometry.
    const ocean=new Uint8Array(n).fill(1),queue=new Int32Array(n);let tail=0;
    for(let i=0;i<n;i++)if(!mask[i]&&(i<w||i>=n-w||i%w===0||i%w===w-1)){ocean[i]=0;queue[tail++]=i;}
    for(let head=0;head<tail;head++)for(const j of neighbors(queue[head],w,h))if(!mask[j]&&ocean[j]){ocean[j]=0;queue[tail++]=j;}
    f.oceanDistance=distance(ocean,w,h,0);
    for(let i=0;i<n;i++){
      let slope=0;for(const j of neighbors(i,w,h))slope=Math.max(slope,Math.abs(f.elevation[i]-f.elevation[j])/2000);
      f.slope[i]=clamp(slope);const y=Math.floor(i/w),x=i%w;
      const lat=o.latitudeNorth+(o.latitudeSouth-o.latitudeNorth)*y/(h-1),abs=Math.abs(lat),r=lat*Math.PI/180;
      f.latitude[i]=lat;
      // Broad coastal current proxy. No claim of solved ocean circulation.
      f.current[i]=Math.sin(x/w*Math.PI*4)*Math.sin(r*2)*3*Math.exp(-f.oceanDistance[i]/3);
      const base=31-57*Math.pow(Math.sin(Math.abs(r)),1.4)+o.temperatureOffset+f.current[i];
      f.temperature[i]=base-Math.max(0,f.elevation[i])*0.0065;
      const amplitude=(3+abs*.1+Math.min(18,f.oceanDistance[i]*1.3))*(mask[i]?1:.35);
      f.summer[i]=f.temperature[i]+amplitude;f.winter[i]=f.temperature[i]-amplitude;
      const east=abs>=30&&abs<60?1:-1;
      f.windX[i]=o.windDirection||east;
      f.windY[i]=(lat>=0?1:-1)*(abs>=30&&abs<60?-.28:.28)+(sample(x/w*4,y/h*4)-.5)*.3;
      f.windStrength[i]=.45+.4*Math.abs(Math.sin(r*3))+.15*sample(x/w*5,y/h*5);
    }
    precipitation(f,mask,w,h,o);
    if(input.climateAdjustments){
      if(input.climateAdjustments.length!==n)throw new Error('Invalid regional rainfall adjustments');
      for(let i=0;i<n;i++){const factor=input.climateAdjustments[i];if(!Number.isFinite(factor)||factor<.01||factor>20)throw new Error('Invalid regional rainfall multiplier');f.precipitation[i]*=factor;const potential=Math.max(120,(f.temperature[i]+25)*24);f.aridity[i]=clamp(potential/Math.max(1,f.precipitation[i])/5);f.moisture[i]=clamp(f.precipitation[i]/potential);}
    }
    hydrology(f,mask,w,h);
    entities(f,mask,w,h,sample);
    return {version:VERSION,options:o,fields:f,heights,mask,margin,plates,points:macro.contPoints};
  }
  function precipitation(f,mask,w,h,o) {
    // Upwind semi-Lagrangian advection, bounded to max dimension; each sweep
    // crosses an entire zonal row. Carry-over supports meridional transport.
    const vapor=new Float32Array(w*h), next=new Float32Array(w*h);
    for(let pass=0;pass<12;pass++){
      for(let y=0;y<h;y++){
        const east=f.windX[y*w+w/2|0]>0;
        for(let k=0;k<w;k++){
          const x=east?k:w-1-k,i=y*w+x,ux=clamp(x-(east?1:-1),0,w-1),up=y*w+ux;
          const uy=clamp(Math.round(y-f.windY[i]),0,h-1),cross=uy*w+ux;
          const input=.85*next[up]+.15*vapor[cross];
          if(!mask[i]) { next[i]=clamp((f.temperature[i]+30)/60,.2,1)*1.5;f.precipitation[i]=900;continue; }
          const lat=Math.abs(f.latitude[i]),wetBelt=.065+.11*Math.exp(-((lat/15)**2))+.06*Math.exp(-(((lat-55)/14)**2));
          const dryBelt=1-.65*Math.exp(-(((lat-28)/9)**2));
          const uplift=Math.max(0,f.elevation[i]-f.elevation[up])/1500;
          const loss=clamp(wetBelt*dryBelt+uplift*.6,.018,.85);
          const rain=input*loss;
          f.precipitation[i]=(rain*10000+12)*o.rainfall;
          next[i]=Math.max(0,input-rain)*.984;
        }
      }
      vapor.set(next);
    }
    for(let i=0;i<w*h;i++){
      const potential=Math.max(120,(f.temperature[i]+25)*24);
      f.aridity[i]=clamp(potential/Math.max(1,f.precipitation[i])/5);
      f.moisture[i]=clamp(f.precipitation[i]/potential);
    }
  }
  function hydrology(f,mask,w,h) {
    // Priority flood: route depressions to spill points, guaranteeing an acyclic
    // drainage graph. Original surface is retained; fill depth marks lakes.
    const n=w*h, seen=new Uint8Array(n),filled=new Float32Array(n),order=[],heap=[];
    const push=(i,z)=>{let k=heap.length;heap.push([i,z]);while(k){const p=(k-1)>>1;if(heap[p][1]<=z)break;heap[k]=heap[p];k=p;}heap[k]=[i,z];};
    const pop=()=>{const top=heap[0],last=heap.pop();if(heap.length){let k=0;while(k*2+1<heap.length){let c=k*2+1;if(c+1<heap.length&&heap[c+1][1]<heap[c][1])c++;if(heap[c][1]>=last[1])break;heap[k]=heap[c];k=c;}heap[k]=last;}return top;};
    for(let i=0;i<n;i++)if(!mask[i]){seen[i]=1;filled[i]=f.elevation[i];push(i,filled[i]);}
    while(heap.length){const [i,z]=pop();order.push(i);for(const j of neighbors(i,w,h))if(!seen[j]){seen[j]=1;filled[j]=Math.max(f.elevation[j],z+.01);f.flow[j]=i;push(j,filled[j]);}}
    for(let i=0;i<n;i++){f.accumulation[i]=mask[i]?Math.max(.02,f.precipitation[i]/1000):0;f.lake[i]=(mask[i]&&filled[i]-f.elevation[i]>20)||(!mask[i]&&f.oceanDistance[i]>0)?1:0;}
    for(let k=order.length-1;k>=0;k--){const i=order[k],j=f.flow[i];if(j>=0)f.accumulation[j]+=f.accumulation[i];}
    for(let i=0;i<n;i++) f.groundwater[i]=mask[i]?clamp(f.moisture[i]*.45+Math.log1p(f.accumulation[i])*.11+f.lake[i]*.7-f.slope[i]*.3):1;
  }
  function entities(f,mask,w,h,sample) {
    const n=w*h;
    // Sediment supply includes fluvial alluvium and erodible sedimentary rock;
    // transport downwind precedes deposition, rather than desert-centre odds.
    for(let y=0;y<h;y++){
      const east=f.windX[y*w+(w>>1)]>0;let carried=0;
      for(let k=0;k<w;k++){
        const x=east?k:w-1-k,i=y*w+x,geology=sample(x/w*6+40,y/h*6+50);
        const supply=mask[i]?clamp(geology*.35+Math.log1p(f.accumulation[i])*.08):0;
        carried=(carried*.88+supply*.3)*f.windStrength[i];
        const deposition=clamp(.25+f.slope[i]+f.moisture[i]*.2);
        f.sediment[i]=clamp(supply*.5+carried*deposition);
        f.dune[i]=mask[i]&&f.aridity[i]>.5&&f.slope[i]<.25?clamp(f.aridity[i]*f.sediment[i]*f.windStrength[i]*(1-f.moisture[i])*2):0;
        f.duneOrientation[i]=Math.atan2(f.windY[i],f.windX[i])+Math.PI/2;
        f.mesa[i]=mask[i]&&f.elevation[i]>400&&geology>.58&&f.slope[i]>.08&&f.slope[i]<.5?clamp(geology*f.aridity[i]*(.4+f.slope[i])):0;
      }
    }
    for(let i=0;i<n;i++){
      const cold=clamp((2-f.winter[i])/15),accum=f.precipitation[i]*cold;
      const melt=Math.max(0,f.summer[i])*120;
      f.snow[i]=mask[i]?clamp((accum-melt)/700):0;
      f.glacier[i]=mask[i]&&(f.elevation[i]>1800||Math.abs(f.latitude[i])>60)?f.snow[i]:0;
      f.fertility[i]=mask[i]?clamp(f.sediment[i]*.4+f.moisture[i]*.4+f.volcano[i]*.2):0;
      f.wetland[i]=mask[i]&&!f.lake[i]&&f.slope[i]<.15?clamp(f.groundwater[i]*(1-f.slope[i]*5)*Math.min(1,f.accumulation[i]/3)):0;
      f.oasis[i]=mask[i]&&f.aridity[i]>.55&&f.groundwater[i]>.5?clamp(f.groundwater[i]*f.aridity[i]):0;
      f.farmland[i]=mask[i]&&!f.lake[i]&&f.summer[i]>10&&f.temperature[i]>0&&f.temperature[i]<30&&f.slope[i]<.18?clamp(f.fertility[i]*f.groundwater[i]*(1-f.slope[i]*4)):0;
      f.settlement[i]=f.farmland[i]*(.5+.5*Math.min(1,f.accumulation[i]/4));
      const runoff=Math.max(...neighbors(i,w,h).map(j=>f.accumulation[j]),f.accumulation[i]);
      f.clarity[i]=clamp(1-Math.log1p(runoff)*.16);
      f.salinity[i]=mask[i]||f.oceanDistance[i]>0?0:35-Math.min(22,runoff*.3);
      f.reef[i]=!mask[i]&&f.bathymetry[i]<70&&f.temperature[i]>20&&f.temperature[i]<31&&f.salinity[i]>30&&f.clarity[i]>.6?clamp((1-f.bathymetry[i]/80)*f.clarity[i]):0;
    }
    // Feed glacier tongues downhill, bounded by warm-season melt and land.
    const order=Array.from({length:n},(_,i)=>i).sort((a,b)=>f.elevation[b]-f.elevation[a]);
    for(const i of order){const j=f.flow[i];if(j>=0&&mask[j]&&f.elevation[j]<f.elevation[i]&&f.summer[j]<8)f.glacier[j]=Math.max(f.glacier[j],f.glacier[i]*.72);}
    for(let i=0;i<n;i++)if(mask[i]&&f.volcano[i]>.62&&f.elevation[i]>300&&sample(i%w*1.71+83,Math.floor(i/w)*1.37)>.65&&neighbors(i,w,h).every(j=>f.volcano[j]<=f.volcano[i])){
      let at=i;for(let step=0;step<7;step++){f.lava[at]=1-step/8;const j=f.flow[at];if(j<0||!mask[j]||f.elevation[j]>=f.elevation[at])break;at=j;}
      const x=i%w,y=Math.floor(i/w);for(let k=1;k<=10;k++){const nx=Math.round(x+f.windX[i]*k),ny=Math.round(y+f.windY[i]*k);if(nx<0||nx>=w||ny<0||ny>=h)break;const j=ny*w+nx;if(mask[j])f.ash[j]=Math.max(f.ash[j],1-k/12);}
    }
  }
  function candidates(e,i) {
    const f=e.fields, h=f.elevation[i],temp=f.temperature[i],wet=f.moisture[i];
    if(f.lake[i])return ['water'];
    if(h<=0)return f.reef[i]>.25?['water','reef']:f.bathymetry[i]>700?['deep_water']:['water'];
    if(f.lava[i]>.2)return ['lava'];if(f.ash[i]>.2)return ['ash'];
    if(f.glacier[i]>.35)return ['glacier'];if(f.snow[i]>.3)return ['snow'];
    if(f.oceanDistance[i]<=1&&h<160&&f.slope[i]<.2&&f.sediment[i]>.1)return ['sand'];
    if(f.wetland[i]>.55&&temp>-8)return ['swamp'];
    if(f.oasis[i]>.5)return ['oasis'];
    if(f.dune[i]>.13)return ['desert','dunes'];
    if(f.mesa[i]>.3)return ['mesa','hills'];
    if(h>2300)return ['mountain'];
    if(f.summer[i]<10)return ['tundra'];
    if(wet<.28)return ['desert'];
    if(temp>20&&f.precipitation[i]>1600)return ['jungle'];
    if(temp<5&&f.precipitation[i]>300)return ['taiga'];
    if(h>900&&f.slope[i]>.15)return ['hills','mountain'];
    if(wet>.65)return ['forest','grass','meadow'];
    return ['grass','meadow'];
  }
  function prepare(input, config) {
    config=input.config||config;
    const environment=build(input,config), adapted=JSON.parse(JSON.stringify(config));
    adapted.climates=[];
    // Original tree-shaped adjacency forces forest buffers even around polar
    // mountains. Environmental masks now own hard physical exclusions; all
    // baseline biomes may meet. WFC near weights/stability still join patches.
    const ids=adapted.types.map(t=>t.id);
    for(const t of adapted.types){t.neighbors=ids;t.continent='';}
    const rules=global.TerrainWFC.compileRules(adapted),masks=new Uint32Array(environment.heights.length);
    for(let i=0;i<masks.length;i++){
      const choices=candidates(environment,i);let mask=0;
      for(let t=0;t<rules.types.length;t++)if(choices.includes(adapted.types[t].environmentType||rules.types[t].id))mask|=1<<t;
      if(!mask)throw new Error('Terrain configuration is missing environmental type: '+choices.join(' / '));
      masks[i]=mask>>>0;
    }
    const o=environment.options;
    const solver=new global.TerrainWFC.Solver(rules,{width:o.columns,height:o.rows,seed:global.TerrainWFC.seedFromString(o.seed),radius2:1,stability:o.stability,selection:o.selection,environmentMasks:masks});
    solver.environment=environment;
    return {environment,solver,rules,config};
  }
  function serialize(session) {
    const {environment:e,solver,rules}=session,fields={};
    for(const name of FIELD_NAMES)fields[name]=Array.from(e.fields[name],v=>Math.round(v*100)/100);
    const savedSettings=Object.assign({},e.options);delete savedSettings.heights;delete savedSettings.sea;delete savedSettings.paint;delete savedSettings.config;delete savedSettings.previousTypes;delete savedSettings.climateAdjustments;
    return {version:VERSION,settings:savedSettings,config:session.config,climateAdjustments:e.options.climateAdjustments,margin:e.margin,fields,types:Array.from({length:solver.N},(_,i)=>rules.types[solver.typeAt(i)]?.id||'water'),palette:rules.types.map(t=>({id:t.id,name:t.name,color:t.color,pattern:t.pattern})),heights:Array.from(e.heights)};
  }
  function generate(input,config) {
    const session=prepare(input,config);let guard=session.solver.N*3;
    while(session.solver.status==='running'&&guard-->0)session.solver.step();
    if(session.solver.status!=='done')throw new Error('Environmental terrain failed to settle.');
    for(let i=0;i<session.environment.options.cleanup;i++)session.solver.cleanup();
    if(input.previousTypes)for(let i=0;i<session.solver.N;i++){const t=session.rules.types.findIndex(t=>t.id===input.previousTypes[i]);if(t>=0&&((session.solver.environmentMasks[i]>>>t)&1))session.solver._write(i,(1<<t)>>>0);}
    if(input.paint){const t=session.rules.types.findIndex(t=>t.id===input.paint.type);if(t<0||!session.solver.paint(input.paint.cells,t))throw new Error(session.solver.message||'Unknown paint type');while(session.solver.status==='running')session.solver.step();}
    return serialize(session);
  }
  global.TerrainEnvironment={VERSION,FIELD_NAMES,build,prepare,generate,serialize,settings,precipitation,hydrology,candidates};
})(globalThis);
