---
layout: home

hero:
  name: eyeful
  text: Multi-agent code review with executable evidence
  tagline: '"Given enough eyeballs, all bugs are shallow." Most projects do not have that many reviewers. eyeful adds more.'
  image:
    src: /eyeful.png
    alt: eyeful sea otter logo
  actions:
    - theme: brand
      text: Get started
      link: /LOCAL
    - theme: alt
      text: How a review works
      link: /REVIEW
    - theme: alt
      text: GitHub
      link: https://github.com/Pleasurecruise/eyeful

features:
  - title: Tests as evidence
    details: An expert that claims a bug writes a test for it, and eyeful runs the test. The strongest findings fail on the change and pass with the fix.
    link: /VERIFICATION
  - title: Understand first, then review
    details: pulls.review core reads what the change does, groups it by intent and marks the core parts. Go picks the experts for each group, and each reviews its groups once, against the public standards for its area.
    link: /EXPERTS
  - title: Your own coding agent
    details: Reviews run with the Claude Code, Codex or pi you are already signed in to, from the eyeful command or the desktop app. No account or server.
    link: /LOCAL
  - title: Your working copy stays yours
    details: eyeful reviews a fixed snapshot, runs only the project commands you confirmed, and runs them in a throwaway checkout.
    link: /LOCAL#local-risks
---

Local reviews work today; cloud reviews of pull requests are planned. Start with the
[overview](/overview) or the [roadmap](/ROADMAP).
