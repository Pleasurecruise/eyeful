# Project instructions

Implement, document and verify changes directly. Product and engineering facts live in `docs/`; the
north star, verification specs and temporary evidence live in `.agents/`.

## 1. Establish scope

Read the documents that own the affected behavior, listed in the [documentation index](docs/README.md),
then implement. Do not write a plan, in chat or in files, unless asked.

## 2. Implement and document

Follow [Code style](docs/STYLEGUIDE.md). The rules broken most often:

- Run everything through mise: `dev`, `test`, `format`, `lint`, `generate`, `check`.
- API changes follow [Changing the API](docs/API.md#changing-the-api) and land with `spec/` and
  `packages/sdk` in one commit.
- No comments except `TODO(area): …`; no single-caller helpers; no `any`.
- Libraries are pinned exactly at their latest release; toolchains follow `latest`.
- Frontend commands go through Vite+ (`vp`); Go lint exceptions go in `.golangci.yml`, never inline.

Update the owning document in the same change, and its translation in `docs/zh/`. Rewrite rather
than append; never duplicate a fact.

## 3. Verify

Run `mise run check`. For feedback-sensitive work, follow the matching spec before implementing:

| Change                                                     | Spec                           |
| ---------------------------------------------------------- | ------------------------------ |
| Console or desktop                                         | `.agents/specs/frontend.md`    |
| Leases, fencing, workers, deletion, rate limits, lifecycle | `.agents/specs/concurrency.md` |

Prove the problem first, break the change as the [five adversaries](docs/STYLEGUIDE.md#review), and
end every handoff with the uncertainty checklist.

## 4. Check the north star

A change should serve an outcome in `.agents/POLARIS.md`; if it serves none, drop it or ask. Stop and
report when evidence shows a hard measure regressed.

Conventional Commits. Preserve unrelated changes. Remove `.agents/evidence/` after acceptance.
