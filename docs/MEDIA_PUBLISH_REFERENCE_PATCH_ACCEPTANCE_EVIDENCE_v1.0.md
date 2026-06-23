# Media Publish Reference Patch Acceptance Evidence v1.0

## Commands Executed

All commands were run from `C:\Users\33141\Documents\media-publish-reference`.

### Targeted real MySQL media tests

```powershell
$env:MEDIA_MODULE6_MYSQL_DSN=$env:MYSQL_DSN
go test ./internal/module/media -run 'Test(PublishReference|ReferenceVisibility)' -count=1 -v
```

Evidence lines:

```text
TestPublishReferenceAgainstMySQL84: pending_hidden=NOT_VISIBLE publish_once=GATEWAY_VISIBLE row_version=3 idempotent=PASS
TestPublishReferenceCompletesActiveHiddenAgainstMySQL84: active_hidden=NOT_VISIBLE publish_completes_visibility row_version=2
TestPublishReferenceValidatesAssetCoordinatesAgainstMySQL84: wrong_asset=ErrNotFound missing_asset=ErrNotFound missing_reference=ErrNotFound no_mutation=PASS
TestReferenceVisibilityPromotionAgainstMySQL84: pending_hidden=NOT_VISIBLE active_hidden=NOT_VISIBLE active_visible=GATEWAY_VISIBLE idempotent=PASS visible_to_hidden=FAIL_CLOSED
TestReferenceVisibilityCASAgainstMySQL84: stale_row_version=ErrConcurrentUpdate visibility_retry=PASS
```

### Media package regression

```powershell
go test ./internal/module/media -count=1 -v
```

Evidence lines:

```text
TestPublishReferenceAgainstMySQL84: PASS
TestPublishReferenceCompletesActiveHiddenAgainstMySQL84: PASS
TestPublishReferenceValidatesAssetCoordinatesAgainstMySQL84: PASS
TestZZPublishReferenceResidueAgainstMySQL84: publish_reference_residue=0
```

### Backend full regression

```powershell
go test ./... -count=1
```

Result: PASS across backend packages.

### Boundary check

```powershell
npm run check:boundaries
```

Result:

```text
ok A-G boundaries plus isolated P0-4 media domain
```

### Full verify

```powershell
npm run verify
```

Result: PASS.

Included:

- `check:boundaries`
- admin typecheck
- miniapp typecheck
- backend tests

### Full build

```powershell
npm run build
```

Result: PASS.

Observed non-blocking existing warning:

- miniapp `vendors.js` exceeds recommended size limit.

## Security and Boundary Evidence

- No credentials were copied or committed.
- `.env` was read only to inject DSN into the local test process.
- No migration files changed.
- No takeout/admin/miniapp files changed.
- No `main.go` change.
- No external service call was introduced by this patch.

## Residue Evidence

`TestZZPublishReferenceResidueAgainstMySQL84` asserted:

```text
publish_reference_residue=0
```

The test checks media assets, references, review projections, and decisions for this patch's test owner ids.
