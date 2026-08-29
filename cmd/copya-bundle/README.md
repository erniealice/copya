# copya-bundle

`copya-bundle` validates and applies one immutable Copya bundle manifest after the caller has proven
the required Esqyma schema release. Plan mode is read-only; `--apply` requires `DATABASE_URL` and the
manifest-declared password environment key.

The command writes the workspace/user/RBAC graph and its database receipt in one transaction. That
receipt is only the idempotency fact for bytes already selected by the target manifest; it never
chooses which bundle to apply. An exact rerun verifies the rows and returns a no-op. A receipt digest
conflict fails closed.
