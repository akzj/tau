---
name: "database-patterns"
description: "Database design: migrations, indexing, N+1 query detection, connection pooling, transactions"
disable-model-invocation: false
trigger: "database|sql|migration|schema|query|postgres|mysql|sqlite"
tools: [read, write, edit, bash, grep, glob]
---

## Patterns
- **Migrations**: Every schema change via migration file. Never modify schema manually. Use `up`/`down` pairs.
- **Indexes**: Add indexes for query WHERE/JOIN/ORDER BY columns. Use `EXPLAIN ANALYZE` to verify.
- **N+1 Detection**: Use query logging or ORM tools to detect N+1 queries. Eager-load with JOINs or batch queries.
- **Connection Pooling**: Set max connections (25-50 for PostgreSQL). Use connection pool with health checks.
- **Transactions**: Wrap multi-statement operations in transactions. Use ROLLBACK on error. Keep transactions short.

## Steps
1. Design schema with migrations. Name files with timestamps: `20250101_add_users.sql`.
2. Add indexes for frequently queried columns. Run EXPLAIN to verify usage.
3. Monitor for N+1 queries in development. Fix with eager loading or batch queries.
4. Configure connection pool: max_open, max_idle, max_lifetime.
5. Use transactions with defer rollback: `tx.Rollback()` on error, `tx.Commit()` on success.
