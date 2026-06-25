# G-⑦c Rider API Acceptance Report

Date: 2026-06-25

Status: PASS

## Acceptance matrix

| Area | Result | Evidence |
|---|---:|---|
| Rider identity | PASS | Unregistered `/me` returns `registered:false`; register creates active rider; duplicate register is idempotent. |
| Claim concurrency | PASS | Two concurrent riders claim one `ready_for_pickup` order: one HTTP 200 and one HTTP 409. |
| In-flight limit | PASS | Rider with one active fulfillment and limit=1 receives HTTP 409 on second claim. |
| Fulfillment owner check | PASS | Other rider cannot view or mutate another rider fulfillment; returns 404. |
| Fulfillment progression | PASS | Owner can progress `ASSIGNED -> TO_STORE -> PICKED -> TO_USER -> DELIVERED`. |
| Completion accounting | PASS | Completion writes exactly one earning record; repeated complete remains idempotent. |
| My deliveries isolation | PASS | Rider list/detail only returns own fulfillments. |
| Wallet isolation | PASS | Wallet returns only current rider earnings and balances. |
| Withdrawal balance guard | PASS | Over-balance withdraw returns HTTP 422; zero amount returns HTTP 400; valid amount creates pending request. |
| Withdrawal isolation | PASS | Rider withdrawal list does not expose other rider records. |
| Boundary check | PASS | `npm run check:boundaries` passed. |
| Full backend tests | PASS | `go test ./... -count=1` passed. |
| Full verification | PASS | `npm run verify` passed. |
| Build | PASS | `npm run build` passed with existing miniapp vendor-size warnings. |

## Real environment

- Real MySQL: `8.4.0`
- Real Gin/HTTP route tests via `httptest.NewServer`
- Real JWT middleware via domain A user service
- No mock route bypass was used.

## Boundary result

Only the rider transport package and composition-root route registration were added. The takeout core domain package remains free of `admin`, `gin`, and `net/http` imports.

## Final conclusion

G-⑦c rider backend transport satisfies the authorized backend scope and can be handed to owner for review/merge. No G-⑦c frontend work was started.
