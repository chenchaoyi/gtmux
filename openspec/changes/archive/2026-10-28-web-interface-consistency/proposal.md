# Unify browser controls and interface language

## Why
Small character icons, tiny compact controls, conversational helper copy and inconsistent clipboard feedback make the browser difficult to use. Message actions also occupy the message's colored background.

## What Changes
- Use a shared bundled icon set and desktop/touch target metrics across browser controls.
- Place message actions, sender and recorded prompt time below the bubble.
- Refine Chinese and English interface copy, terminology, empty and recovery states.
- Correct clipboard success/failure feedback across messages, pinned prompts and terminal captures.
- Make layout actions, session navigation, reply choices and appearance disclosures keyboard accessible.

## Surfaces
- terminal: preserve captures and send contracts; improve browser copy feedback and keyboard controls.
- menubar: no additional native changes; the existing local browser entry opens the same refined workspace.
- phone: no native client changes; browser touch controls use larger targets.
- iPad: no native client changes; coarse-pointer browser controls follow the same touch metrics.
- Web: all browser routes and overlays receive shared controls, language and feedback rules.
