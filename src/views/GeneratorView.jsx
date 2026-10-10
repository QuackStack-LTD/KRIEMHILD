import { useEffect, useRef } from 'react';
import { mountTerrain } from '../terrain/controller.js';
export default function GeneratorView({onReady,onWorld}) {
  const callbacks=useRef({onReady,onWorld});callbacks.current={onReady,onWorld};
  useEffect(() => mountTerrain({onReady:api=>callbacks.current.onReady(api),onWorld:world=>callbacks.current.onWorld(world)}), []);
  return (<main className="app">
    <section className="stage">
      <div className="canvas-wrap">
        <canvas id="map" aria-label="Generated terrain map"></canvas>
      </div>
      <div id="mapNavigation" className="map-navigation" hidden>
        <button id="zoomOut" aria-label="Zoom out">?</button>
        <button id="zoomIn" aria-label="Zoom in">+</button>
        <button id="fitMap">World view</button>
        <output id="detailStatus" aria-live="polite">World</output>
        <span>Scroll at cursor to explore ? drag to pan</span>
      </div>
      <div className="toolbar">
        <label className="check"><input id="brushOn" type="checkbox" /> Brush <kbd>B</kbd></label>
        <span className="swatch" id="brushSwatch"></span>
        <select id="brushType" aria-label="Terrain type to paint"></select>
        <label className="tool-size">Size
          <input id="brushSize" type="range" min="0" max="12" step="1" defaultValue="2" />
          <output id="brushSizeOut"></output>
        </label>
      </div>
      <div id="brushCursor" hidden></div>
      <div className="hover" id="hover">Hover over a cell to see what it can still become.</div>
      <div id="naturalHover" />
    </section>

    <aside className="panel">
      <header className="panel-head">
        <h1>Terrain Generation</h1>
        <label className="preset">
          <span>Preset</span>
          <select id="preset" aria-describedby="presetDesc"></select>
        </label>
        <p className="preset-desc" id="presetDesc"></p>
      </header>

      <div className="action-bar">
        <div className="actions">
          <button id="generate" className="primary" title="Generate a new map (G)">Generate</button>
          <button id="pause" title="Pause or resume (Space)">Pause</button>
          <button id="step" title="Collapse one cell (S)">Step</button>
          <button id="cleanup" title="Run one cleanup pass on the finished map (C)" disabled>Clean up</button>
        </div>
        <label className="seed-row">
          <span>Seed</span>
          <input id="seed" type="text" placeholder="random each time" spellCheck="false" />
        </label>
        <details className="status">
          <summary id="statusLine">Loading…</summary>
          <dl className="stats" id="stats"></dl>
        </details>
      </div>

      <div className="error" id="error" hidden></div>

      <nav className="tabs" role="tablist" aria-label="Settings">
        <button role="tab" id="tab-world" aria-controls="panel-world" aria-selected="true">World</button>
        <button role="tab" id="tab-terrain" aria-controls="panel-terrain" aria-selected="false" tabIndex="-1">Terrain</button>
        <button role="tab" id="tab-generator" aria-controls="panel-generator" aria-selected="false" tabIndex="-1">Generator</button>
        <button role="tab" id="tab-view" aria-controls="panel-view" aria-selected="false" tabIndex="-1">View</button>
      </nav>

      <div className="tab-body">
      
      <section className="tab-panel" id="panel-world" role="tabpanel" aria-labelledby="tab-world">
        <section className="group">
          <h2>Map</h2>
          <label className="check"><input id="environment" type="checkbox" defaultChecked /> Physical environment</label>
          <label className="check"><input id="realism" type="checkbox" /> Real-world geology &amp; climate</label>
          <p className="hint">Enables physical mode with derived climate zones, rivers, wind-shaped dunes, volcanic activity and water-dependent farms and villages. Turn off to use the earlier physical model.</p>
          <p className="hint">Choose a preset, then layer physical environment and real-world rules onto it. Presets set the land coverage, climate and geology; physical conditions constrain where their terrain can appear.</p>
          <p id="modeHint" className="hint"></p>
          <div className="row3"><label>Land %<input id="landCoverage" type="number" min="1" max="75" defaultValue="42" /></label><label>Plates<input id="plateCount" type="number" min="3" max="32" defaultValue="12" /></label></div>
          <div className="row3"><label>North latitude<input id="latitudeNorth" type="number" min="-90" max="90" defaultValue="90" /></label><label>South latitude<input id="latitudeSouth" type="number" min="-90" max="90" defaultValue="-90" /></label></div>
          <div className="row3"><label>Island groups<input id="islandFrequency" type="number" min="0" max="3" step="0.1" defaultValue="1" title="Geological island-group frequency: 0 disables added groups, 1 normal, 3 abundant" /></label><label>Coastal share<input id="islandCoastalShare" type="number" min="0" max="1" step="0.05" defaultValue="0.6" title="Share of new groups near continental margins and marginal seas; the rest favor open-ocean tectonics and hotspots" /></label></div>
          <div className="row3"><label>Relief %<input id="ruggedness" type="number" min="1" max="200" defaultValue="100" /></label><label>Volcanic activity<input id="volcanism" type="number" min="0.1" max="3" step="0.1" defaultValue="1" /></label></div>
          <div className="row3"><label>Temperature offset °C<input id="temperatureOffset" type="number" min="-25" max="25" defaultValue="0" /></label><label>Rain multiplier<input id="rainfall" type="number" min="0.1" max="3" step="0.1" defaultValue="1" /></label></div>
          <div className="row3">
            <label>Width <input id="width" type="number" min="16" max="256" defaultValue="160" /></label>
            <label>Height <input id="height" type="number" min="16" max="256" defaultValue="100" /></label>
            <label>Cell px <input id="cellSize" type="number" min="1" max="40" defaultValue="6" /></label>
          </div>
          <details className="climate-settings">
            <summary>Climate, seasons &amp; planetary scale</summary>
            <p className="hint">Available with Physical environment. Blank planetary values vary with the generation seed. Default coverage is a full Earth-sized planet in an equirectangular projection. Width and height above are cells; pixel dimensions are cells multiplied by Cell px.</p>
            <div className="row3"><label>Seasons<select id="seasonMode" defaultValue="automatic"><option value="automatic">Automatic climate phases</option><option value="custom">Custom count</option></select></label><label>Season count<input id="seasonCount" type="number" min="1" max="6" defaultValue="4" disabled /></label></div>
            <label className="field">Optional custom season names<input id="seasonNames" type="text" maxLength="485" placeholder="Ember, Rain, Frost (in annual order)" /></label>
            <label className="field">Geographic coverage<select id="climateCoverage" defaultValue="planet"><option value="planet">Whole planet (default)</option><option value="hemisphere">Hemisphere</option><option value="continent">Continent</option><option value="island">Island</option><option value="local">Local region</option></select></label>
            <div className="row3"><label>Axial tilt (degrees)<input id="planetTilt" type="number" min="0" max="90" step="0.1" placeholder="Seeded" /></label><label>Year (days)<input id="planetYearDays" type="number" min="30" max="3000" placeholder="Seeded" /></label></div>
            <div className="row3"><label>Orbital eccentricity<input id="planetEccentricity" type="number" min="0" max="0.6" step="0.01" placeholder="Seeded" /></label><label>Perihelion (year fraction)<input id="planetPerihelion" type="number" min="0" max="1" step="0.01" placeholder="Seeded" /></label></div>
            <div className="row3"><label>Circulation strength<input id="planetCirculation" type="number" min="0" max="2" step="0.1" placeholder="Seeded" /></label><label>Planet radius (km)<input id="planetRadius" type="number" min="500" max="50000" placeholder="6371" /></label></div>
            <div className="row3"><label>West longitude<input id="longitudeWest" type="number" min="-180" max="180" step="0.01" placeholder="-180" /></label><label>East longitude<input id="longitudeEast" type="number" min="-180" max="180" step="0.01" placeholder="180" /></label></div>
            <label className="field">North?south km per cell (optional)<input id="worldScale" type="number" min="0.001" max="10000" step="0.001" placeholder="Derived from planet size and latitude bounds" /></label>
            <p className="hint">Distances and cell areas account for spherical geometry. Full planets wrap east?west. For regional coverage, km per cell can set the extent around the latitude midpoint; leave it blank to use geographic bounds.</p>
<p className="hint">Latitude bounds above and longitude bounds here set the actual coverage. Season counts come from climate curves, never pixel dimensions. Custom counts subdivide the modeled year without inventing weather changes.</p>
          </details>
          <details className="climate-settings"><summary>River networks &amp; geological erosion</summary>
            <div className="row3"><label>Minimum catchment (km?)<input id="riverMinArea" type="number" min="1" max="10000000" placeholder="80" /></label><label>Minimum flow (m?/s)<input id="riverMinDischarge" type="number" min="0.01" max="1000000" step="0.01" placeholder="0.5" /></label></div>
            <div className="row3"><label>Visible stream order<input id="riverVisibleOrder" type="number" min="1" max="12" placeholder="2" /></label><label>Ephemeral density<input id="ephemeralDensity" type="number" min="0" max="1" step="0.05" placeholder="0.35" /></label></div>
            <div className="row3"><label>Major catchment (km?)<input id="riverMajorArea" type="number" min="1" max="100000000" placeholder="20000" /></label><label>Regional catchment (km?)<input id="riverRegionalArea" type="number" min="1" max="100000000" placeholder="1500" /></label></div>
            <div className="row3"><label>Erosion passes<input id="erosionIterations" type="number" min="0" max="6" defaultValue="2" /></label><label>Duration (million years)<input id="erosionDuration" type="number" min="0" max="50" step="0.1" defaultValue="2" /></label><label>Erosion strength<input id="erosionStrength" type="number" min="0" max="3" step="0.1" defaultValue="1" /></label></div>
            <p className="hint">Channels require concentrated upstream water. Small streams are revealed at closer zooms. Erosion modifies actual elevation, then recalculates drainage; zero passes disables it.</p>
          </details>
          <label className="field shape-field">Shape
            <select id="shape">
              <option value="flat">Flat</option>
              <option value="sphere">Sphere (edges wrap around; a globe in 3D)</option>
            </select>
          </label>
        </section>

        <section className="group">
          <div className="group-head">
            <h2>Continental layer</h2>
            <label className="switch" title="Random water, land, desert… points that steer each area"><input id="continents" type="checkbox" aria-label="Continental layer" defaultChecked /><span></span></label>
          </div>
          <label className="field slider-label" htmlFor="contPoints">Points</label>
          <div className="speed">
            <input id="contPoints" type="range" min="1" max="40" step="1" defaultValue="12" />
            <output id="contPointsOut"></output>
          </div>
          <label className="field slider-label" htmlFor="contStrength">Strength: how hard cells lean toward the nearest points' kind</label>
          <div className="speed">
            <input id="contStrength" type="range" min="0" max="20" step="0.5" defaultValue="5" />
            <output id="contStrengthOut"></output>
          </div>
          <details id="kindDetails" className="sub-editor">
            <summary>Kinds and spawn odds</summary>
            <div className="group-head sub-head">
              <span className="hint-inline">Each point picks a kind by these odds.</span>
              <button id="addKind" className="small" type="button">+ Add kind</button>
            </div>
            <ul className="kinds" id="kinds"></ul>
            <p className="hint">The two most likely kinds always get at least one point. Assign terrain types to a kind in the Terrain tab.</p>
          </details>
        </section>

        <section className="group">
          <div className="group-head">
            <h2>Legacy climate bands</h2><p className="hint">Band weights apply when Physical environment is off. Physical mode uses the north/south latitude and climate settings in Map.</p>
            <label className="switch" title="Latitude bands from the equator to the poles"><input id="climate" type="checkbox" aria-label="Climate zones" defaultChecked /><span></span></label>
          </div>
          <label className="field">Equator
            <select id="climLayout">
              <option value="both">Across the middle (poles at top and bottom)</option>
              <option value="north">Along the bottom (pole at the top)</option>
            </select>
          </label>
          <label className="field slider-label" htmlFor="climStrength">Strength: 1 = multipliers as set, 2 = squared, 0 = off</label>
          <div className="speed">
            <input id="climStrength" type="range" min="0" max="5" step="0.1" defaultValue="2" />
            <output id="climStrengthOut"></output>
          </div>
          <details id="climateDetails" className="sub-editor">
            <summary>Zones, from the equator to the poles</summary>
            <ul className="types zones" id="zones"></ul>
            <p className="hint">Click a zone to edit it. Each zone multiplies continental kinds' spawn odds and terrain types' weights: ×1 = no change, ×0 = never.</p>
          </details>
        </section>
      </section>

      
      <section className="tab-panel" id="panel-terrain" role="tabpanel" aria-labelledby="tab-terrain" hidden>
        <section className="group">
          <div className="group-head">
            <h2>Terrain types</h2>
            <button id="addType" className="small" type="button">+ Add type</button>
          </div>
          <ul className="types" id="types"></ul>
          <p className="hint">Click a type to edit its colour, weight, neighbours, texture and 3D height. Changes apply right away and are saved in this browser.</p>
          <div className="actions small-actions">
            <button id="exportJson" type="button" title="Download the current types as tiles.json">Export</button>
            <label className="button">Import<input id="importJson" type="file" accept=".json,application/json" hidden /></label>
            <button id="resetTypes" type="button" title="Throw away your edits and load tiles.json">Reset to defaults</button>
          </div>
        </section>
      </section>

      
      <section className="tab-panel" id="panel-generator" role="tabpanel" aria-labelledby="tab-generator" hidden>
        <section className="group">
          <h2>Algorithm</h2>
          <label className="field">Next cell to collapse
            <select id="selection">
              <option value="random">Random cell in superposition</option>
              <option value="entropy">Fewest options first (classic WFC)</option>
            </select>
          </label>
          <label className="field slider-label" htmlFor="radius">Neighbour radius: which cells the rules apply to</label>
          <div className="speed">
            <input id="radius" type="range" min="0" max="1" step="1" defaultValue="1" />
            <output id="radiusOut"></output>
          </div>
        </section>

        <section className="group">
          <div className="group-head">
            <h2>Stability</h2>
            <label className="switch" title="Cells lean toward what their neighbours already are"><input id="stability" type="checkbox" aria-label="Stability" defaultChecked /><span></span></label>
          </div>
          <div className="speed">
            <input id="stabilityStrength" type="range" min="0" max="300" step="1" defaultValue="30" aria-label="Stability strength" />
            <output id="stabilityOut"></output>
          </div>
        </section>

        <section className="group">
          <h2>Cleanup</h2>
          <label className="field">Passes after generating
            <input id="cleanupPasses" type="number" min="0" max="50" defaultValue="2"
                   title="Each pass turns lone cells into the type that surrounds them, where the rules allow it" />
          </label>
        </section>

        <section className="group">
          <h2>Speed</h2>
          <div className="speed">
            <input id="speed" type="range" min="0" max="100" defaultValue="45" aria-label="Cells per frame" />
            <output id="speedOut"></output>
          </div>
          <label className="check"><input id="instant" type="checkbox" /> Instant (skip the animation)</label>
        </section>
      </section>

      
      <section className="tab-panel" id="panel-view" role="tabpanel" aria-labelledby="tab-view" hidden><section className="group"><h2>Environmental overlay</h2><select id="environmentOverlay"><option value="terrain">Terrain</option><option value="temperature">temperature</option><option value="summer">summer</option><option value="winter">winter</option><option value="precipitation">precipitation</option><option value="elevation">elevation</option><option value="windX">windX</option><option value="windStrength">windStrength</option><option value="moisture">moisture</option><option value="aridity">aridity</option><option value="plate">plate</option><option value="boundary">boundary</option><option value="bathymetry">bathymetry</option><option value="flow">flow</option><option value="accumulation">accumulation</option><option value="groundwater">groundwater</option><option value="snow">snow</option><option value="glacier">glacier</option><option value="dune">dune</option><option value="reef">reef</option><option value="volcano">volcano</option><option value="sediment">sediment</option><option value="fertility">fertility</option><option value="mesa">mesa</option><option value="oasis">oasis</option><option value="wetland">wetland</option><option value="farmland">farmland</option><option value="settlement">settlement</option><option value="lava">lava</option><option value="ash">ash</option><option value="lake">lake</option><option value="salinity">salinity</option><option value="clarity">clarity</option></select></section>
        <section className="group">
          <div className="group-head">
            <h2>Voronoi cells</h2>
            <label className="switch" title="Organic cell shapes instead of squares"><input id="voronoi" type="checkbox" aria-label="Voronoi cells" defaultChecked /><span></span></label>
          </div>
          <label className="field slider-label" htmlFor="jitter">Point offset: how far each cell's point moves from its centre</label>
          <div className="speed">
            <input id="jitter" type="range" min="0" max="1" step="0.01" defaultValue="0.35" />
            <output id="jitterOut"></output>
          </div>
        </section>

        <section className="group">
          <div className="group-head">
            <h2>Textures</h2>
            <label className="switch" title="Repeating icons per terrain type"><input id="textures" type="checkbox" aria-label="Textures" defaultChecked /><span></span></label>
          </div>
          <label className="field slider-label" htmlFor="iconSize">Icon size</label>
          <div className="speed">
            <input id="iconSize" type="range" min="8" max="40" step="1" defaultValue="16" />
            <output id="iconSizeOut"></output>
          </div>
          <label className="check"><input id="contShow" type="checkbox" /> Show the continental points</label>
        </section>

        <section className="group">
          <div className="group-head">
            <h2>3D view</h2>
            <label className="switch" title="Show the map as a landscape (or a globe for sphere worlds)"><input id="view3d" type="checkbox" aria-label="3D view" /><span></span></label>
          </div>
          <label className="field slider-label" htmlFor="height3d">Height exaggeration</label>
          <div className="speed">
            <input id="height3d" type="range" min="0" max="3" step="0.1" defaultValue="1" />
            <output id="height3dOut"></output>
          </div>
          <button id="resetCamera" className="small" type="button" title="Back to the starting angle">Reset camera</button>
        </section>

        <section className="group">
          <h2>Save</h2>
          <div className="save-row">
            <button id="saveProject" type="button">Export terrain ZIP</button>
            <label className="button">Open terrain ZIP<input id="openProject" type="file" accept=".zip,application/zip" hidden /></label>
            <label className="button">Open terrain folder<input id="openProjectFolder" type="file" webkitdirectory="" multiple hidden /></label>
          </div>
          <p className="hint" id="autosaveStatus" role="status">Click Save in the top bar to store the world and explored detail.</p>
          <p className="hint" id="projectStatus" role="status">Reopen previous worlds below. ZIP files provide portable backups.</p>
          <label className="field"><span>Saved worlds</span><select id="storedProjects"><option value="">No saved worlds</option></select></label>
          <div className="save-row"><button id="refreshProjects" type="button">Refresh saved worlds</button><button id="openStoredProject" type="button">Open saved world</button></div>
          <div className="save-row">
            <button id="savePng" type="button">PNG image</button>
            <button id="saveSvg" type="button" title="Vector map: Voronoi shapes, flat colours (no textures)">SVG vector</button>
          </div>
        </section>
      </section>
      </div>
    </aside>
  </main>);
}
