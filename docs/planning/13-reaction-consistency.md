# Reaction Service Redis/PG Consistency

## Decision

PostgreSQL is the source of truth for likes and favorites. Redis is a rebuildable cache for fast counters, sets, and hot ranking acceleration.

## Invariants

- `user_likes` owns like state. `status = 1` means active, `status = 0` means inactive.
- `favorites` owns favorite state. `deleted_at IS NULL` means active.
- API response counts for likes/favorites must come from PostgreSQL when the PG repositories are available.
- User lists such as `/users/current/likes` and `/users/current/favorites` must read PostgreSQL.
- Hot IDs must prefer PostgreSQL aggregation, so stale Redis sorted sets cannot change visible hot results.
- Redis write failures after a successful PG mutation are cache drift, not business failure. The mutation response and emitted event still use PG-derived `count` and `changed`.
- Duplicate mutations must be idempotent. Repeated like/favorite returns `changed=false`; repeated unlike/unfavorite returns `changed=false`.

## Cache Rebuild

`reaction-service` can rebuild Redis reaction keys from PostgreSQL on startup with:

```yaml
reaction:
  rebuildCacheOnStart: true
```

The rebuild only deletes keys under `bbs:reaction:*`, then reloads active `user_likes` and `favorites`. This keeps Redis recoverable after local cache loss, stale sets, or failed best-effort syncs.

Operators can also run an explicit rebuild and verification:

```powershell
backend\services\reaction-service\bin\reaction-service.exe rebuild-cache -c configs/config.yaml --verify
```

The command returns JSON stats including `likes_loaded`, `favorites_loaded`, `hot_entries`, and `verified`.

## Operational Notes

- Keep PG migrations for `user_likes` and `favorites` idempotent.
- Do not add user-visible reaction behavior that reads Redis as the only source of truth.
- If startup rebuild becomes too expensive, move it to an explicit admin/ops command, but keep PG-backed count/list/hot reads.
- Event consumers should honor the `changed` flag to avoid duplicate feed, notification, and credit projections.
