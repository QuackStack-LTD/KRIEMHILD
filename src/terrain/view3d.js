// 3D view: drapes the 2D map image over a height field and lets you orbit around it.
// main.js loads this module on demand the first time the 3D toggle is switched on; three.js comes
// from the CDN listed in the import map in index.html.
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { waterSurfacePositions } from './water-surface.js';
import { createDetailScene } from './detail-scene.js';

const MAX_VERTICES = 250000; // bigger maps sample every n-th cell

export function createView(container) {
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.domElement.className = 'view3d';
  container.append(renderer.domElement);

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x111418);
  const camera = new THREE.PerspectiveCamera(40, 1, 0.1, 10000);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.zoomToCursor = true;
  controls.maxPolarAngle = Math.PI * 0.47; // don't go under the map

  scene.add(new THREE.HemisphereLight(0xffffff, 0x3a4048, 1.3));
  const sun = new THREE.DirectionalLight(0xfff4e0, 2.4);
  scene.add(sun);

  let land = null;
  let sea = null;
  let texture = null;
  let W = 0;
  let H = 0;
  let gw = 0;
  let gh = 0;
  let sphere = false;
  let radius = 1;
  let dirs = null; // sphere: each vertex's direction from the centre
  let waterFields=null;
  let waterScale=null;
  let detailScene=null;
  let detailLayer='terrain';

  // A new map (or new size or shape): rebuild the meshes. `canvas` is the 2D map, used as the
  // texture. On a sphere the map is wrapped like a world map: x is longitude, y latitude.
  function setMap(width, height, canvas, opts = {}) {
    disposeMeshes();
    W = width;
    H = height;
    sphere = !!opts.sphere;
    waterFields=opts.waterFields||null;
    waterScale=null;
    const step = Math.max(1, Math.ceil(Math.sqrt((W * H) / MAX_VERTICES)));
    gw = Math.max(2, Math.ceil(W / step));
    gh = Math.max(2, Math.ceil(H / step));

    texture = new THREE.CanvasTexture(canvas);
    texture.colorSpace = THREE.SRGBColorSpace;
    texture.anisotropy = renderer.capabilities.getMaxAnisotropy();
    const seaMaterial = new THREE.MeshStandardMaterial({ color: 0x2a6fc9, transparent: true, opacity: 0.35, roughness: 0.25, metalness: 0 });

    let geometry;
    if (sphere) {
      radius = W / (2 * Math.PI); // the map's width becomes the equator
      texture.wrapS = THREE.RepeatWrapping; // no seam where the edges meet
      geometry = new THREE.SphereGeometry(radius, gw, gh);
      const pos = geometry.attributes.position;
      dirs = new Float32Array(pos.count * 3);
      for (let i = 0; i < pos.count; i++) {
        const v = new THREE.Vector3().fromBufferAttribute(pos, i).normalize();
        dirs.set([v.x, v.y, v.z], i * 3);
      }
      sea = new THREE.Mesh(new THREE.SphereGeometry(radius, 96, 64), seaMaterial);
    } else {
      dirs = null;
      geometry = new THREE.PlaneGeometry(W, H, gw - 1, gh - 1).rotateX(-Math.PI / 2);
      // Translucent sea surface at height 0; terrain below it reads as underwater.
      sea = new THREE.Mesh(new THREE.PlaneGeometry(W, H).rotateX(-Math.PI / 2), seaMaterial);
    }
    land = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ map: texture, roughness: 0.95, metalness: 0 }));
    if(waterFields){sea.geometry.dispose();sea.geometry=new THREE.BufferGeometry();sea.material.depthWrite=false;sea.material.side=THREE.DoubleSide;sea.material.opacity=.25;}
    scene.add(land);
    scene.add(sea);
    if(opts.solver?.environment){detailScene=createDetailScene(scene,camera,renderer,opts.solver);land.visible=false;sea.visible=false;}


    const size = sphere ? radius * 3 : Math.max(W, H);
    sun.position.set(-size * 0.6, size * 0.8, size * 0.35);
    camera.near = size / 500;
    camera.far = size * 20;
    controls.maxPolarAngle = sphere ? Math.PI : Math.PI * 0.47; // a globe can be seen from below
    controls.minDistance = detailScene ? .001 : (sphere ? radius * 1.2 : 0);
    resetCamera();
  }

  // heights: one value per map cell (row by row), already in world units.
  function setHeights(heights,scale=1) {
    if (!land) return;
    if(waterFields&&waterScale!==scale){
      waterScale=scale;
      const positions=waterSurfacePositions(waterFields,W,H,scale,sphere);
      const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.BufferAttribute(positions,3));geometry.computeVertexNormals();
      sea.geometry.dispose();sea.geometry=geometry;
    }
    const pos = land.geometry.attributes.position;
    if (!sphere) {
      for (let j = 0; j < gh; j++) {
        const cy = Math.round((j * (H - 1)) / (gh - 1));
        for (let i = 0; i < gw; i++) {
          const cx = Math.round((i * (W - 1)) / (gw - 1));
          pos.setY(j * gw + i, heights[cy * W + cx]);
        }
      }
      pos.needsUpdate = true;
      land.geometry.computeVertexNormals();
      return;
    }

    // Sphere: (gw + 1) × (gh + 1) vertices, the first and last column on the same meridian.
    // Each vertex moves out along its direction by its cell's height.
    for (let j = 0; j <= gh; j++) {
      const cy = Math.min(H - 1, Math.floor((j / gh) * H));
      // All vertices of a pole row sit on the pole itself: give them one shared height.
      let poleH = 0;
      if (j === 0 || j === gh) {
        for (let x = 0; x < W; x++) poleH += heights[cy * W + x];
        poleH /= W;
      }
      for (let i = 0; i <= gw; i++) {
        const cx = Math.floor((i / gw) * W) % W;
        const r = radius + (j === 0 || j === gh ? poleH : heights[cy * W + cx]);
        const v = j * (gw + 1) + i;
        pos.setXYZ(v, dirs[v * 3] * r, dirs[v * 3 + 1] * r, dirs[v * 3 + 2] * r);
      }
    }
    pos.needsUpdate = true;
    land.geometry.computeVertexNormals();
    // The seam's two vertex columns get separate normals; average them so no line shows.
    const nrm = land.geometry.attributes.normal;
    for (let j = 0; j <= gh; j++) {
      const a = j * (gw + 1);
      const b = a + gw;
      const n = new THREE.Vector3(nrm.getX(a) + nrm.getX(b), nrm.getY(a) + nrm.getY(b), nrm.getZ(a) + nrm.getZ(b)).normalize();
      nrm.setXYZ(a, n.x, n.y, n.z);
      nrm.setXYZ(b, n.x, n.y, n.z);
    }
    nrm.needsUpdate = true;
  }

  function textureChanged() {
    detailScene?.refresh();
    if (texture) texture.needsUpdate = true;
  }

  function resetCamera() {
    if (sphere) {
      camera.position.set(0, radius * 0.9, radius * 3.2);
    } else {
      const size = Math.max(W, H);
      camera.position.set(0, size * 0.55, size * 0.75);
    }
    controls.target.set(0, 0, 0);
    controls.update();
  }

  const resize = () => {
    const w = container.clientWidth;
    const h = container.clientHeight;
    if (!w || !h) return;
    renderer.setSize(w, h, false);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  };
  const observer = new ResizeObserver(resize);
  observer.observe(container);
  resize();

  let raf = 0;
  const loop = () => {
    raf = requestAnimationFrame(loop);
    controls.update();
    detailScene?.update(waterScale??1,detailLayer);
    renderer.render(scene, camera);
  };
  loop();

  function disposeMeshes() {
    detailScene?.dispose();detailScene=null;
    for (const m of [land, sea]) {
      if (!m) continue;
      scene.remove(m);
      m.geometry.dispose();
      m.material.dispose();
    }
    if (texture) texture.dispose();
    land = sea = texture = null;
  }

  function dispose() {
    cancelAnimationFrame(raf);
    observer.disconnect();
    controls.dispose();
    disposeMeshes();
    renderer.dispose();
    renderer.domElement.remove();
  }

  const raycaster=new THREE.Raycaster();
  const zoomAtTerrain=e=>{
    if(!detailScene)return;
    e.preventDefault();e.stopImmediatePropagation();
    const rect=renderer.domElement.getBoundingClientRect();
    raycaster.setFromCamera(new THREE.Vector2((e.clientX-rect.left)/rect.width*2-1,1-(e.clientY-rect.top)/rect.height*2),camera);
    const hit=raycaster.intersectObjects(detailScene.meshes(),false)[0];if(!hit)return;
    const delta=e.deltaY*(e.deltaMode===1?16:e.deltaMode===2?rect.height:1);
    const minimum=rect.height/(2*3072*Math.tan(THREE.MathUtils.degToRad(camera.fov/2)));
    const factor=Math.max(minimum/Math.max(.00001,camera.position.distanceTo(hit.point)),Math.exp(Math.max(-1,Math.min(1,delta*.0015))));
    camera.position.sub(hit.point).multiplyScalar(factor).add(hit.point);
    controls.target.sub(hit.point).multiplyScalar(factor).add(hit.point);
    camera.near=Math.max(.00001,Math.min(camera.near,camera.position.distanceTo(hit.point)/100));camera.updateProjectionMatrix();
    controls.update();
    const projected=hit.point.clone().project(camera);
    const targetX=(e.clientX-rect.left)/rect.width*2-1,targetY=1-(e.clientY-rect.top)/rect.height*2;
    renderer.domElement.dataset.anchorError=String(Math.hypot(projected.x-targetX,projected.y-targetY));
  };
  renderer.domElement.addEventListener('wheel',zoomAtTerrain,{capture:true,passive:false});
  return { setMap, setHeights, textureChanged, resetCamera, dispose, setDetailLayer(layer){detailLayer=layer;},
    cameraState(){return {position:camera.position.toArray(),target:controls.target.toArray(),near:camera.near};},
    restoreCamera(state){if(!state||!Array.isArray(state.position)||!Array.isArray(state.target)||state.position.length!==3||state.target.length!==3||![...state.position,...state.target].every(Number.isFinite))return;camera.position.fromArray(state.position);controls.target.fromArray(state.target);if(Number.isFinite(state.near)&&state.near>0)camera.near=state.near;camera.updateProjectionMatrix();controls.update();}
  };
}
