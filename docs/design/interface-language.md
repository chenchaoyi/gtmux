# Interface language

[中文](interface-language.zh.md)

Use concise, conventional labels for built-in interface text. This applies to Mac, phone, iPad and web; user-authored content, agent replies and historical release notes retain their original wording.

## Names, actions and explanations

- Name settings with short noun phrases: **Desktop notifications**, **Tab marker**, **Activity logs**, **Diagnostic report**, **Debug logging**, **Link name**.
- Name actions with verbs and their object when context is insufficient: **Export…**, **Revoke access**, **Create link**. Use **View** and **Input** consistently for sharing permissions.
- Put trigger conditions and consequences in the description under the setting, not in a sentence used as its title.
- State what the control changes. A share-link name is a label, not proof of the recipient's identity. Scope text names the selected panes and granted permissions.
- Keep status messages factual: **No access recorded**, **No warnings or errors recorded**, **Submission failed**. Missing records do not establish that an event never occurred; system acceptance does not establish visible delivery.
- Avoid rhetorical questions, personification and conversational instructions such as “What gtmux recorded” or “Who is it for?”. Prefer direct descriptions over reassurance or filler.
- Keep English and Chinese meanings aligned. Preserve wire keys, compatibility aliases, quoted user content and past release archives when updating display text.

## Hierarchy

A control and its description form one row. Group its status, action and feedback below it; do not give every explanatory sentence a Form separator. Keep independent controls in their own group. Secondary text remains readable in both appearances. Error and pending states retain their actual severity and recovery actions.

These rules change presentation, not permission grants, notification policy or persistence. A wording review does not authorize configuration changes or publishing private diagnostic data.
