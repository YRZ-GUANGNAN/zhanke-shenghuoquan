# Media Publish Reference Patch Acceptance Report v1.0

## Verdict

PASS.

The Media Domain now exposes a complete `PublishReference` capability that accepts business coordinates and internally performs:

`asset_public_id -> asset_id -> reference lookup -> pending/active gate -> hidden/visible gate`

The method preserves Media Domain ownership of reference visibility rules and does not leak CAS details to business domains.

## Acceptance Matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Preflight master/tag/AutoMigrate gate | PASS | `master=baf9cb60`, required tags present, `AutoMigrate` 0 matches |
| Reference coordinate lookup | PASS | New repository method uses `(asset_id, reference_type, reference_id, slot)` |
| Publish pending/hidden reference | PASS | Real MySQL test produced `ReferenceActive + ParentVisible`, `row_version=3` |
| Gateway visibility after publish | PASS | Real MySQL test confirmed `BuildDisplayView` returns visible view |
| Idempotent repeat publish | PASS | Repeat publish returned nil and did not grow `row_version` |
| Active/hidden intermediate state | PASS | Publish completed only visibility and ended at `row_version=2` |
| Wrong asset public id | PASS | Returned `ErrNotFound`, did not mutate target reference |
| Missing asset | PASS | Returned `ErrNotFound` |
| Missing reference coordinate | PASS | Returned `ErrNotFound` |
| Existing lower-level methods unchanged | PASS | Existing reference visibility tests still pass |
| Boundary check | PASS | `npm run check:boundaries` |
| Backend full test | PASS | `go test ./... -count=1` |
| Verify | PASS | `npm run verify` |
| Build | PASS | `npm run build` with existing miniapp vendor size warning |
| Residue | PASS | `publish_reference_residue=0` |

## Real MySQL Evidence Summary

Targeted command:

```powershell
go test ./internal/module/media -run 'Test(PublishReference|ReferenceVisibility)' -count=1 -v
```

Result:

- `TestPublishReferenceAgainstMySQL84`: PASS
- `TestPublishReferenceCompletesActiveHiddenAgainstMySQL84`: PASS
- `TestPublishReferenceValidatesAssetCoordinatesAgainstMySQL84`: PASS
- `TestReferenceVisibilityPromotionAgainstMySQL84`: PASS
- `TestReferenceVisibilityCASAgainstMySQL84`: PASS

Media package command:

```powershell
go test ./internal/module/media -count=1 -v
```

Result: PASS.

## Scope Boundary

This patch intentionally stops at Media Domain capability. Takeout invocation is not implemented here and must be separately authorized.

## Residue

Test-owned media assets, references, review projections, and decisions for owners `99000801`, `99000802`, and `99000803` were cleaned. Residue assertion passed with all counts equal to 0.

## Final Status

`feat/media-publish-reference`: ready for owner review. Do not enter takeout wiring until separately authorized.
