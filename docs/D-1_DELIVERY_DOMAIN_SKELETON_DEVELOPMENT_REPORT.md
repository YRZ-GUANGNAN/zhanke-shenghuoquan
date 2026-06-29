# D-1 Delivery Domain Skeleton Development Report

## 1. Scope and baseline

- Baseline: `master@e812103f`
- Baseline tag: `d0-legacy-gate-baseline`
- Implementation branch: `feat/d1-delivery-skeleton`
- Implementation commit: `437926ba feat(D-1): add delivery domain skeleton with migration 023`
- Scope: one Delivery-owned table, domain model, state machine, Repository/CAS, Service, migration 023 and tests.

D-1 does not mount runtime routes and does not implement payment, rider identity,
claim-pool transport, photos, IM, webhook, wallet, frontend or workers.

## 2. Pre-development gate

The required preflight passed:

- `master` contained the frozen D-0 gate.
- `d0-legacy-gate-baseline` existed.
- `backend/cmd/server` contained zero `db.AutoMigrate(` calls.
- Migration `022_takeout_payment.sql` was the highest existing migration; 023 was free.
- The main worktree was clean.
- `backend/internal/module/delivery` did not exist.

File-level review confirmed:

- `scripts/check-boundaries.cjs` classified all backend module directories and had a dedicated import rule for the frozen takeout core.
- Existing takeout CAS used transaction + `SELECT ... FOR UPDATE` + expected status/row-version update + affected-row validation.
- The takeout ULID helper was package-private and therefore had to be reproduced locally without importing takeout.
- `takeout_orders` coupled buyer, catalog item amounts and preparation states, so it was not suitable for the Delivery domain.
- Forward and rollback migrations are maintained in `backend/migrations` and `backend/migrations/rollback`.

## 3. Domain implementation

New package: `backend/internal/module/delivery`.

Production files:

```text
model.go
repository.go
service.go
state_machine.go
ulid.go
```

The package defines:

- a Delivery-local ULID implementation;
- `Order` and nine valid statuses;
- terminal-state semantics;
- all 15 authorized state-machine edges;
- a Repository interface and GORM implementation;
- creation, lookup and explicit transition methods;
- fail-closed domain errors;
- integer-cent fee snapshots.

No Delivery production file imports:

- `takeout`;
- another A-G business domain;
- Gin or `net/http`;
- COS, IM, WeChat or another external SDK.

GORM is confined to `repository.go`.

## 4. State machine

Implemented transitions:

```text
pending_fee_payment -> ready_for_claim | payment_failed | cancelled
payment_failed      -> pending_fee_payment | cancelled
ready_for_claim     -> claimed | cancelled
claimed             -> to_pickup | ready_for_claim | cancelled
to_pickup           -> picked_up | ready_for_claim | cancelled
picked_up           -> delivering
delivering          -> delivered
```

Terminal statuses:

```text
delivered
cancelled
```

The retry edge keeps `payment_failed` non-terminal. Requeue transitions clear the
current rider assignment. Normal cancellation is rejected after pickup.

## 5. Repository and CAS

Every transition:

1. starts a database transaction;
2. reads the order using `SELECT ... FOR UPDATE`;
3. validates current state and critical rider parameters;
4. returns idempotently if the same target state and critical parameters already match;
5. updates using `id + expected status + expected row_version`;
6. increments `row_version` atomically;
7. returns `ErrConcurrentUpdate` unless exactly one row changed.

Repeated assignment to the same rider is idempotent. Repeating the same target
state with another rider returns `ErrRiderMismatch`.

## 6. Migration 023

Added:

```text
backend/migrations/023_delivery_order.sql
backend/migrations/rollback/023_delivery_order.sql
```

The single `delivery_orders` table contains:

- public ULID and logical merchant/rider identifiers;
- pickup and recipient snapshots;
- integer-cent fee snapshots;
- status and stage timestamps;
- CAS row version and audit timestamps.

It contains no foreign key and no soft-delete column. Cancellation is an
auditable terminal state.

Migration SHA-256:

```text
7CD122CA3839FC532E7860281B3F0C65E9E5457C5B193150E33E99A8DFE62981  023_delivery_order.sql
A35CD5A3FCF44F0BA9FBB198B2FF2EC8536E602FB52158A8D32B9659BDA6EC36  rollback/023_delivery_order.sql
```

## 7. Boundary update

`delivery` was added to the classified backend module set.

A dedicated production-source check rejects Delivery imports of HTTP, takeout
or any other business domain. The shared database-access rule continues to
reject GORM access outside Repository files.

## 8. Tests added

```text
boundary_test.go
model_test.go
repository_mysql_test.go
service_test.go
state_machine_test.go
```

Coverage includes:

- ULID validation;
- model and fee-snapshot validation;
- every legal and illegal state transition;
- service input validation;
- real MySQL schema and reversible migration;
- real MySQL lifecycle and timestamps;
- stale CAS and two-way concurrent competition;
- idempotency and critical-rider mismatch;
- database CHECK enforcement;
- scoped cleanup and residue assertion.

## 9. Explicitly deferred

D-1 does not include:

- HTTP, Gin or `main.go` registration;
- rider eligibility or in-flight limits;
- payment, recharge, wallet or fee configuration;
- photos, MediaGateway or COS;
- Tencent IM or enterprise WeChat webhook;
- frontend pages;
- fulfillment, claim-attempt, payment or media tables.

These remain D-2+ work and require separate authorization.
