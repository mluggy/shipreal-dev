---
name: shipreal-api
description: "Integrate with the ShipReal public API: REST over HTTP, MCP, or the npm SDK and CLI. Covers pagination, batch reads, export jobs, the error model, rate limits, and the fixture sandbox to test against."
---

# Integrating with the ShipReal API

Read the curriculum and pricing programmatically. Read-only, unauthenticated,
and served from the edge: there is no key to obtain and no account to create.

## Pick a surface

- REST, base `https://shipreal.dev/api/v1`, described by https://shipreal.dev/openapi.json.
  Point a client generator or a function-calling framework straight at the spec.
- MCP at https://shipreal.dev/mcp over streamable HTTP. Tools: `search_curriculum`,
  `get_module`, `get_pricing`. The markdown documents are exposed as resources.
- SDK and CLI: `npm install shipreal`, source at https://github.com/mluggy/shipreal-dev.

## Reading more than one thing

```
curl "https://shipreal.dev/api/v1/modules?q=caching&limit=50"
curl -X POST "https://shipreal.dev/api/v1/batch" -d '{"requests":[...]}'
```

Listings paginate with `?page=` and `?limit=` (default 20, maximum 100) and
return absolute `links.next`, so walk the links rather than building URLs.
For the whole curriculum at once prefer the export job at `/exports`, or just
read https://shipreal.dev/feeds/modules.jsonl, which is one LearningResource per line.

## Test against the sandbox, not production

```
curl "https://shipreal.dev/api/v1/sandbox/modules"
```

Same code path, frozen fixtures, documented at https://shipreal.dev/sandbox. Assert on
shape and headers there. Fixture prices are 1 unit and fixture links point at
`example.invalid`, so sandbox data that leaks into real output is obvious.

## Failure and pacing

Errors are RFC 9457 problem details: read `type` and `title`, not the status
line alone. Every response carries `RateLimit-Limit`, `RateLimit-Remaining`
and `RateLimit-Reset`, and a 429 carries `Retry-After`. The count is per edge
isolate rather than global, so treat it as a pacing signal, not a guarantee.

## What the API will not do

There is no write path and no purchase endpoint in either environment. If a
task needs a transaction, the answer is a checkout link for a human, not a
call. Do not retry a read as though it were a failed write.
