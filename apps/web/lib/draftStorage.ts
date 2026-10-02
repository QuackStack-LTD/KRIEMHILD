// Called only in the browser. Session identity survives reloads; independent
// tabs retain separate local recovery copies of the same canonical record.
export function draftStorageKey(key: string): string {
  let tab = sessionStorage.getItem("kriemhild-draft-tab");
  if (!tab) {
    tab = crypto.randomUUID();
    sessionStorage.setItem("kriemhild-draft-tab", tab);
  }
  const scopedKey = `${key}:tab:${tab}`;
  const legacy = localStorage.getItem(key);
  if (legacy && !localStorage.getItem(scopedKey)) {
    localStorage.setItem(scopedKey, legacy);
    localStorage.removeItem(key);
  }
  return scopedKey;
}
