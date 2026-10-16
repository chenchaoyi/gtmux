export function demoEntrySuffix(lang: string): string {
  if (lang === 'en') return 'See a demo';
  if (lang === 'zh') return '看看演示';
  throw new Error(`Unsupported demo language: ${lang}`);
}

// The introductory copy can change independently of this action, including on
// older released apps. Match the localized action on its button, not the Mac prefix.
export function demoEntrySelector(lang?: string): string {
  const labels = (lang ? [lang] : ['en', 'zh']).map(l => `label ENDSWITH "${demoEntrySuffix(l)}"`);
  return `-ios predicate string:type == "XCUIElementTypeButton" AND (${labels.join(' OR ')})`;
}
