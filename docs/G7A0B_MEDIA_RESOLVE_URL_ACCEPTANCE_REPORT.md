# G-7a-0b Media Resolve URL Acceptance Report

## Conclusion

Status: PASS

Branch: `feat/media-resolve-url`

Baseline: `813eb765 feat(G-7a): add buyer takeout backend API`

Final commit: `TO_BE_FILLED`

The authenticated resolve endpoint is implemented and verified. It lets the miniapp call with `Taro.request` and receive a short-lived COS signed URL that can be assigned to `<Image src>`.

## Acceptance matrix

| Item | Result | Evidence |
| --- | --- | --- |
| Resolve endpoint registered under `/api/v1/miniapp/media` | PASS | Handler registers `GET /resolve/:referenceType/:referenceId/:slot` |
| AuthRequired + UnmutedRequired preserved | PASS | Existing route group unchanged in `backend/cmd/server/main.go`; real HTTP no-token test returns `401` |
| Visible media returns JSON signed URL | PASS | Real HTTP E2E returns `200` and signed COS URL |
| Signed URL is usable | PASS | Real COS GET via returned URL returns expected payload |
| Pending/hidden/non-visible media blocked | PASS | Real HTTP hidden reference returns `404`; unit tests cover fail-closed errors |
| Missing coordinate blocked | PASS | Real HTTP missing reference returns `404` |
| Invalid reference type blocked | PASS | Real HTTP invalid type returns `400` |
| Display endpoint behavior unchanged | PASS | Unit test still verifies `302`; real HTTP display route returns `302` |
| No raw storage identity fields in JSON | PASS | DTO only contains `url` and `expiresInSeconds` |
| Existing media tests unaffected | PASS | `go test ./... -count=1` PASS |
| Boundary checks | PASS | `npm run check:boundaries` PASS |
| Full verify | PASS | `npm run verify` PASS |
| Build | PASS | `npm run build` PASS, with existing miniapp vendor-size warning |
| Residue cleanup | PASS | Real E2E asserts DB residue `0`; COS cleanup helper verifies no version/delete-marker residue |

## Final route

```text
GET /api/v1/miniapp/media/resolve/:referenceType/:referenceId/:slot
```

The endpoint is intended for authenticated miniapp calls. Frontend should call it through `Taro.request` with the existing bearer token, cache the returned URL for at most `expiresInSeconds`, and assign the returned URL to `<Image src>`.

## Notes

- The returned URL is a short-lived Tencent COS signed GET URL.
- The endpoint does not return permanent media URLs.
- The endpoint does not accept JWT in query string.
- The endpoint does not alter media visibility, review state, references, or COS objects.
