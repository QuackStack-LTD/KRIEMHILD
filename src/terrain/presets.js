// Presets: ready-made worlds. Each one is a set of changes on top of the defaults in tiles.json
// (rules) plus generation settings for the sidebar (settings).
//
// rules:
//   onlyTypes  keep just these terrain types (everything else is removed, references included)
//   weights    { typeId: weight }
//   near       { typeId: { otherTypeId: weight } }  merged into "weight when next to…"
//   odds       { kindId: odds } for continental kinds; kinds not listed get 0
// settings:
//   radius2 (1 = 4 neighbours, 2 = 8, 4 = 12, 5 = 20, 9 = 28), stability (0 = off),
//   continents { points, strength } or null, climate { strength, layout } or null, cleanup
(function (global) {
  'use strict';

  const LIST = [
    {
      id: 'earth',
      name: 'Earth-like (default)',
      description: 'Oceans, continents and climate bands from the poles to the equator.',
      settings: { radius2: 2, stability: 3, continents: { points: 12, strength: 5 }, climate: { strength: 2, layout: 'both' }, cleanup: 2 },
      rules: {},
    },
    {
      id: 'islands',
      name: 'Islands',
      description: 'A warm archipelago: open ocean, coral reefs, jungle islands and the odd volcano.',
      settings: { radius2: 2, stability: 6, continents: { points: 40, strength: 6 }, climate: null, cleanup: 3 },
      rules: {
        odds: { water: 0.75, land: 0.12, jungle: 0.08, volcanic: 0.05 },
        weights: { reef: 0.12, sand: 0.5, jungle: 0.6, swamp: 0.2, oasis: 0, village: 0.08, glacier: 0, tundra: 0, taiga: 0, snow: 0.1 },
        near: { reef: { reef: 4 }, jungle: { jungle: 3 } },
      },
    },
    {
      id: 'lava',
      name: 'World of lava',
      description: 'Volcanic rock, lava rivers and bare mountains; barely any water.',
      settings: { radius2: 2, stability: 7, continents: { points: 16, strength: 6 }, climate: null, cleanup: 3 },
      rules: {
        odds: { volcanic: 0.6, mountain: 0.25, desert: 0.1, water: 0.05 },
        weights: {
          lava: 1.5, ash: 1.2, mountain: 0.6, desert: 0.4, mesa: 0.3,
          deep_water: 0.05, water: 0.1, reef: 0, grass: 0.1, meadow: 0, hills: 0.1, farmland: 0, village: 0,
          forest: 0.1, snow: 0.05, oasis: 0, tundra: 0, taiga: 0, glacier: 0, swamp: 0, jungle: 0,
        },
        near: { lava: { lava: 8 }, ash: { ash: 5 } },
      },
    },
    {
      id: 'diversity',
      name: 'World of diversity',
      description: 'Every kind of land equally likely, lots of regions, and rare terrain made common.',
      settings: { radius2: 2, stability: 2, continents: { points: 30, strength: 5 }, climate: { strength: 1.5, layout: 'both' }, cleanup: 1 },
      rules: {
        odds: { water: 0.2, land: 0.14, mountain: 0.14, desert: 0.14, tundra: 0.13, jungle: 0.13, volcanic: 0.12 },
        weights: { village: 0.15, farmland: 0.12, oasis: 0.1, reef: 0.15, meadow: 0.3, hills: 0.4, swamp: 0.3, mesa: 0.3 },
      },
    },
    {
      id: 'continents',
      name: 'Simple continents',
      description: 'Just water, beaches, grass, forest and mountains: a few big continents with wide coasts.',
      settings: { radius2: 5, stability: 6, continents: { points: 8, strength: 7 }, climate: null, cleanup: 3 },
      rules: {
        onlyTypes: ['deep_water', 'water', 'sand', 'grass', 'forest', 'mountain', 'snow'],
        odds: { water: 0.55, land: 0.35, mountain: 0.1 },
      },
    },
    {
      id: 'villages',
      name: 'Villages and fields',
      description: 'Countryside: farmland around villages, meadows, hills, woods and lakes.',
      settings: { radius2: 2, stability: 6, continents: { points: 14, strength: 5 }, climate: null, cleanup: 2 },
      rules: {
        odds: { land: 0.65, water: 0.2, mountain: 0.1, jungle: 0.05 },
        weights: { farmland: 0.6, village: 0.3, meadow: 0.4, hills: 0.3, grass: 0.7, forest: 0.4, swamp: 0.05, jungle: 0, desert: 0, dunes: 0, mesa: 0, tundra: 0, glacier: 0 },
        near: { farmland: { farmland: 5, village: 3 }, village: { village: 4, farmland: 1 } },
      },
    },
    {
      id: 'frozen',
      name: 'Frozen north',
      description: 'Tundra, taiga forests, glaciers and snowy mountains around cold seas.',
      settings: { radius2: 2, stability: 6, continents: { points: 14, strength: 5 }, climate: null, cleanup: 3 },
      rules: {
        odds: { tundra: 0.5, water: 0.3, mountain: 0.2 },
        weights: {
          glacier: 0.6, tundra: 0.6, taiga: 0.5, snow: 0.6, mountain: 0.4,
          jungle: 0, swamp: 0, desert: 0, dunes: 0, mesa: 0, oasis: 0, reef: 0, farmland: 0, lava: 0.02, village: 0.02,
        },
      },
    },
    {
      id: 'desert',
      name: 'Endless desert',
      description: 'Sand seas, dunes and mesas with the occasional oasis and a few mountains.',
      settings: { radius2: 2, stability: 6, continents: { points: 14, strength: 6 }, climate: null, cleanup: 3 },
      rules: {
        odds: { desert: 0.6, mountain: 0.15, water: 0.1, land: 0.1, volcanic: 0.05 },
        weights: { desert: 0.8, dunes: 0.5, mesa: 0.3, oasis: 0.1, jungle: 0, swamp: 0, tundra: 0, taiga: 0, glacier: 0, snow: 0.05, reef: 0 },
        near: { oasis: { oasis: 1 }, dunes: { dunes: 4 }, mesa: { mesa: 3 } },
      },
    },
  ];

  // Physical inputs express the same theme through causal environmental fields.
  // Palette weights remain preferences within the physically valid candidates.
  const environments = {
    earth: { islandFrequency:1, islandCoastalShare:.6, landPercent:42, temperatureOffset:0, rainfall:1, latitudeNorth:90, latitudeSouth:-90, ruggedness:100, plateCount:12, volcanism:1 },
    islands: { islandFrequency:2.2, islandCoastalShare:.45, landPercent:25, temperatureOffset:2, rainfall:1.3, latitudeNorth:25, latitudeSouth:-25, ruggedness:110, plateCount:16, volcanism:1.2 },
    lava: { islandFrequency:1, islandCoastalShare:.6, landPercent:75, temperatureOffset:20, rainfall:.2, latitudeNorth:45, latitudeSouth:-45, ruggedness:145, plateCount:24, volcanism:3 },
    diversity: { islandFrequency:1, islandCoastalShare:.6, landPercent:60, temperatureOffset:0, rainfall:1, latitudeNorth:90, latitudeSouth:-90, ruggedness:125, plateCount:20, volcanism:1.3 },
    continents: { islandFrequency:1, islandCoastalShare:.6, landPercent:45, temperatureOffset:0, rainfall:1, latitudeNorth:70, latitudeSouth:-70, ruggedness:100, plateCount:10, volcanism:.7 },
    villages: { islandFrequency:1, islandCoastalShare:.6, landPercent:65, temperatureOffset:0, rainfall:1.2, latitudeNorth:50, latitudeSouth:25, ruggedness:45, plateCount:8, volcanism:.3 },
    frozen: { islandFrequency:1, islandCoastalShare:.6, landPercent:55, temperatureOffset:-8, rainfall:1.2, latitudeNorth:85, latitudeSouth:50, ruggedness:115, plateCount:14, volcanism:.5 },
    desert: { islandFrequency:1, islandCoastalShare:.6, landPercent:75, temperatureOffset:7, rainfall:.2, latitudeNorth:35, latitudeSouth:15, ruggedness:85, plateCount:12, volcanism:.5 },
  };
  for(const preset of LIST) preset.environment=environments[preset.id];

  function withEnvironmentTypes(config, defaults) {
    const result=JSON.parse(JSON.stringify(config));
    const present=new Set(result.types.map(t=>t.environmentType||t.id));
    const missing=defaults.types.filter(t=>!present.has(t.id));
    if(result.types.length+missing.length>32)throw new Error('Physical generation needs the standard environmental terrain types. Remove unused custom types to leave room in the 32-type palette.');
    result.types.push(...JSON.parse(JSON.stringify(missing)));
    for(const kind of defaults.continents||[])if(!result.continents.some(k=>k.id===kind.id))result.continents.push({...kind});
    return result;
  }

  // Applies a preset's rule changes to a copy of the defaults (tiles.json) and returns it.
  function build(defaults, preset) {
    const cfg = JSON.parse(JSON.stringify(defaults));
    const r = preset.rules || {};
    if (r.onlyTypes) {
      const keep = new Set(r.onlyTypes);
      const filterKeys = (obj) => Object.fromEntries(Object.entries(obj || {}).filter(([id]) => keep.has(id)));
      cfg.types = cfg.types.filter((t) => keep.has(t.id));
      for (const t of cfg.types) {
        t.neighbors = t.neighbors.filter((id) => keep.has(id));
        if (t.weightNear) t.weightNear = filterKeys(t.weightNear);
      }
      for (const z of cfg.climates || []) z.types = filterKeys(z.types);
    }
    const byId = new Map(cfg.types.map((t) => [t.id, t]));
    for (const [id, w] of Object.entries(r.weights || {})) if (byId.has(id)) byId.get(id).weight = w;
    for (const [id, near] of Object.entries(r.near || {})) {
      const t = byId.get(id);
      if (!t) continue;
      t.weightNear = { ...(t.weightNear || {}) };
      for (const [other, w] of Object.entries(near)) if (byId.has(other)) t.weightNear[other] = w;
    }
    if (r.odds) for (const k of cfg.continents || []) k.odds = r.odds[k.id] || 0;
    return cfg;
  }

  global.TerrainPresets = { LIST, build, withEnvironmentTypes };
})(typeof window !== 'undefined' ? window : globalThis);
