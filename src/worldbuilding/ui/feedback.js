// Async, application-owned feedback. Works from React and the terrain controller.
let items = [];
const listeners = new Set();
export const feedbackStore = {
  subscribe(fn) {
    listeners.add(fn);
    return () => listeners.delete(fn);
  },
  snapshot: () => items,
};
function emit() {
  for (const fn of listeners) fn();
}
function ask(kind, message, initial = "", options = {}) {
  return new Promise((resolve) => {
    items = [
      ...items,
      { id: crypto.randomUUID(), kind, message, initial, ...options, resolve },
    ];
    emit();
  });
}
export function settleFeedback(id, value) {
  const item = items.find((x) => x.id === id);
  items = items.filter((x) => x.id !== id);
  emit();
  item?.resolve(value);
}
export const confirmAction = (message, options) =>
  ask("confirm", message, "", options);
export const requestText = (message, initial = "", options) =>
  ask("prompt", message, initial, options);
export const inform = (message) => ask("notice", message);

const guards = new Set();
export function registerNavigationGuard(guard) {
  guards.add(guard);
  return () => guards.delete(guard);
}
export async function allowNavigation() {
  for (const guard of [...guards]) if (!(await guard())) return false;
  return true;
}
