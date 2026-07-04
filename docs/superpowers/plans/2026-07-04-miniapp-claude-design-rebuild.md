# Miniapp Claude Design Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use inline execution in this session. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the miniapp frontend around the Claude Design handoff while preserving existing miniapp routes and backend contracts.

**Architecture:** Keep the current Taro page structure. Move the shared visual language into `app.scss` and `components.tsx`, then rewrite each page TSX/SCSS pair with local static view models and existing navigation calls.

**Tech Stack:** Taro 4, React 18, SCSS, existing `services/taro` wrapper.

---

## File Map

- Modify: `apps/miniapp/src/app.scss` for global tokens, shared cards, bottom nav, headers, chips, image placeholders.
- Modify: `apps/miniapp/src/components.tsx` for shared primitives used by rebuilt pages.
- Modify: `apps/miniapp/src/pages/*/index.tsx` for miniapp page structure and clean Chinese content.
- Modify: `apps/miniapp/src/pages/*/index.scss` for per-page layouts.
- Verify: `npm --workspace apps/miniapp run typecheck` and `npm --workspace apps/miniapp run build:weapp`.

### Task 1: Shared System

- [ ] Replace global styles with the campus-blue design system.
- [ ] Keep bottom navigation route keys unchanged.
- [ ] Remove excessive font weights and decorative one-note gradients.

### Task 2: Student Surface

- [ ] Rebuild home as brand header, search, service grid, segmented feed, two-column cards.
- [ ] Rebuild food as partner mini-program aggregation, not in-app ordering.
- [ ] Rebuild market and product detail as second-hand listing and seller-contact flow.
- [ ] Rebuild publish, post detail, profile, and my content with clean Chinese text.

### Task 3: Delivery Surface

- [ ] Rebuild rider hall with wallet stats, status tabs, privacy-safe route cards, and claim actions.
- [ ] Rebuild merchant workspace with stats, quick actions, order tabs, and delivery-sheet form.
- [ ] Rebuild wallet and delivery conversation with plain user-facing Chinese.

### Task 4: Message Helpers

- [ ] Rebuild conversations, chat, notifications, and media history.
- [ ] Remove mojibake and internal technology wording.

### Task 5: Verification

- [ ] Run `npm --workspace apps/miniapp run typecheck`.
- [ ] Run `npm --workspace apps/miniapp run build:weapp`.
- [ ] Run a mojibake scan against `apps/miniapp/src` and `apps/miniapp/dist`.
