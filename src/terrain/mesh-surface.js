// Interpolate the same a/c/b, b/c/d triangles used by the terrain mesh.
export function meshSurface(position,nx,ny,u,v){
 u=Math.max(0,Math.min(nx-1,u));v=Math.max(0,Math.min(ny-1,v));
 const x=Math.min(nx-2,Math.floor(u)),y=Math.min(ny-2,Math.floor(v)),a=u-x,b=v-y,i=y*nx+x;
 const corners=a+b<=1?[[i,1-a-b],[i+1,a],[i+nx,b]]:[[i+nx+1,a+b-1],[i+1,1-b],[i+nx,1-a]];
 const p=[0,0,0];for(const [at,w]of corners)for(let k=0;k<3;k++)p[k]+=position[at*3+k]*w;
 return p;
}
