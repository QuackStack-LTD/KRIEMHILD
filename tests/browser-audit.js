// Execute this expression in the collaborative preview on the running app.
// Uses real DOM changes and captures the resulting network requests. No mock
// generation engine: every session is validated/created by the live Go service.
(async () => {
  const $ = id => document.getElementById(id);
  const assert = (condition, message) => { if (!condition) throw new Error(message); };
  const originalFetch = window.fetch;
  const originalConfirm = window.confirm;
  const calls = [];
  window.__auditCalls = calls;
  window.__auditError = null;
  const checks = [];
  window.__auditProgress = checks;
  window.confirm = () => true;
  window.fetch = async (url, options) => {
    const response = await originalFetch(url, options);
    if (String(url) === '/api/sessions' && options?.method === 'POST') {
      calls.push({ body: JSON.parse(options.body), status: response.status, data: await response.clone().json() });
    }
    return response;
  };
  const change = (id, value) => {
    const el = $(id);
    if (el.type === 'checkbox') el.checked = value; else el.value = value;
    el.dispatchEvent(new Event('change', { bubbles: true }));
  };
  const nextSession = async action => {
    const before = calls.length;
    action();
    const deadline = Date.now() + 10000;
    while (calls.length === before && Date.now() < deadline) await new Promise(resolve => setTimeout(resolve, 20));
    assert(calls.length > before, 'No generation request was made');
    const result = calls.at(-1);
    assert(result.status === 201, JSON.stringify(result.data));
    // Let the controller accept this session before changing another control.
    await new Promise(resolve => setTimeout(resolve, 40));
    return result;
  };
  try {
    $('width').value = '48'; $('height').value = '32'; $('seed').value = 'audit'; $('instant').checked = true;
    await nextSession(()=>change('environment',false));
    await nextSession(() => change('preset', 'earth'));
    let session = await nextSession(() => change('environment', true));
    assert(session.body.environment && session.data.contPoints.length === Number($('contPoints').value), 'Physical request/markers missing');
    assert(!$('radius').disabled && $('contStrength').disabled && $('climStrength').disabled, 'Physical neighborhood/legacy controls incorrect');
    assert(!$('latitudeNorth').disabled && !$('contPoints').disabled, 'Physical latitude/point controls incorrectly disabled');
    checks.push('Physical mode routes to Go and distinguishes active controls');

    await nextSession(()=>change('environment',false));
    for (const preset of window.TerrainPresets.LIST) {
      session = await nextSession(() => change('preset', preset.id));
      assert(!session.body.environment && !$('environment').checked, preset.id + ' did not select WFC');
      assert(session.body.options.radius2 === preset.settings.radius2, preset.id + ' radius ignored');
      assert(session.body.options.continents?.count === preset.settings.continents?.points, preset.id + ' continental point count ignored');
      assert(Boolean(session.body.options.climate) === Boolean(preset.settings.climate), preset.id + ' climate toggle ignored');
      for (const [id, weight] of Object.entries(preset.rules.weights || {})) assert(session.body.config.types.find(t => t.id === id)?.weight === weight, preset.id + ' weight not applied: ' + id);
      if (preset.rules.onlyTypes) assert(session.body.config.types.length === preset.rules.onlyTypes.length, preset.id + ' palette was not filtered');
      checks.push('Preset ' + preset.id + ': rules, dimensions, weights and mode sent correctly');
    }
    session = await nextSession(() => change('stabilityStrength', '300'));
    assert(session.body.options.stability === 300, 'Maximum stability not sent/accepted');
    checks.push('Maximum stability accepted');

    await nextSession(() => change('preset', 'earth'));
    await nextSession(() => change('environment', true));
    session = await nextSession(() => change('contPoints', '24'));
    assert(session.body.environment.continentCount === 24 && session.data.contPoints.length === 24, 'Point count ignored');
    const saved = JSON.parse(localStorage.getItem('kriemhild.settings.v1'));
    assert(saved.environment === true && saved.contPoints === '24', 'Physical settings not persisted');
    checks.push('Point count changes geometry inputs and persists');

    // Renaming a biome through the actual terrain editor must retain its mask.
    $('tab-terrain').click();
    const row = [...document.querySelectorAll('#types .type-row')].find(el => el.textContent.startsWith('Water'));
    if (row.getAttribute('aria-expanded') !== 'true') row.click();
    const name = document.querySelector('#types .type-edit input[type=text]');
    assert([...document.querySelectorAll('#types .chips input')].every(el => el.disabled), 'Physical adjacency editor should be disabled');
    name.value = 'Azure sea';
    name.dispatchEvent(new Event('input', { bubbles: true }));
    name.dispatchEvent(new Event('change', { bubbles: true }));
    await nextSession(() => $('generate').click());
    const palette = JSON.parse(localStorage.getItem('kriemhild.types.v1'));
    assert(palette.types.some(t => t.id === 'azure_sea' && t.environmentType === 'water'), 'Rename lost environmental identity');
    checks.push('Terrain rename survives generation and palette persistence');

    await nextSession(()=>change('environment',false));
    await nextSession(() => change('preset', 'islands'));
    const restored = JSON.parse(localStorage.getItem('kriemhild.settings.v1'));
    assert(!restored.environment && restored.contPoints === '40', 'Preset settings were not persisted');
    window.__browserAudit = { checks, expectedAfterReload: { physical: false, preset: 'islands', points: '40', seed: 'audit', width: '48', height: '32' } };
    return window.__browserAudit;
  } finally {
    window.fetch = originalFetch;
    window.confirm = originalConfirm;
  }
})()
