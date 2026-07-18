# WalletConnect bridge implementation plan

1. Add an explicit persisted mainnet-transaction setter and warning-backed Settings control; keep the existing backend guard authoritative.
2. Add WalletConnect request DTOs and a TxService prepare method that reconstructs a clean Bridge call from immutable intent fields and holds it through the existing confirmation gate.
3. Refactor publication internally so the normal flow returns a hash and WalletConnect can return the finalized published account-block JSON without duplicating signing logic.
4. Add WalletConnect v2 SignClient transport, exact namespace validation, proposal/session/request state, and lifecycle/error handling in the frontend.
5. Add the WalletConnect route, navigation entry, pairing/session screen, and Go-derived request confirmation UI.
6. Add backend and frontend compatibility/security tests, generate Wails bindings, update dependency locks, and run the full verification suite.
7. Manually pair and exercise both bridge dapps with the same wallet build; use a small mainnet amount only after the explicit mainnet opt-in is enabled.
8. Apply the follow-up security review: make publication terminal before relay response, handle lock/session/account lifecycle races, serialize the backend pending slot, enforce canonical Wrap/Redeem funding, retry failed client initialization, reconcile restored sessions, and add regression coverage for each path.
9. Close the refuse-if-occupied follow-up regression: error-state dialog/navigation/transfer-retry cleanup releases retryable holds by identity, and a WalletConnect publication failure after session termination does the same before dropping local state.
