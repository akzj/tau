---
name: "api-design"
description: "REST API design: endpoints, error handling, pagination, idempotency, versioning"
disable-model-invocation: false
trigger: "api|rest|endpoint|route|http handler|controller"
tools: [read, write, edit, grep, glob]
---

## Principles
- **Resources, not actions**: `/users/123` not `/getUser?id=123`. Use HTTP methods (GET/POST/PUT/DELETE).
- **Consistent errors**: `{"error":{"code":"NOT_FOUND","message":"User 123 not found"}}`. Always same structure.
- **Pagination**: `?page=2&per_page=20`. Return `total`, `next`, `prev` in response.
- **Idempotency**: PUT and DELETE must be idempotent. Use idempotency keys for POST.
- **Versioning**: `/v1/users` or `Accept: application/vnd.api+v1`. Don't break existing clients.

## Steps
1. Define resources and their relationships (1:1, 1:N, M:N).
2. Design URL structure: `/resource/:id/sub-resource/:id`.
3. Specify request/response schemas (JSON Schema or OpenAPI).
4. Implement error handling: 400 (bad request), 401 (unauth), 403 (forbidden), 404 (not found), 409 (conflict), 429 (rate limit), 500 (internal).
5. Add pagination to all list endpoints. Default page size: 20.
