# shipreal

SDK and CLI for the public [ShipReal](https://shipreal.dev) course API. Search
the curriculum, read a module, read pricing.

**No API key. No account. No signup.** It is public read-only reference data
about one course, served from the edge. If something asks you for a ShipReal
credential, it is not us.

```
npm install shipreal
```

## Library

```js
import { ShipReal } from "shipreal";

const sr = new ShipReal();

// Does the course cover caching?
const { data } = await sr.search("caching");
//=> [{ slug: "24-caching-strategies", title: "Caching Strategies", part: "Part 7: ...", chapters: 12, ... }]

// One module, by slug or by title fragment
await sr.module("observability");

// Everything, pagination handled for you
for await (const m of sr.modules()) console.log(m.title);

// What does it cost? Two regional prices are live at once.
await sr.pricing();                   // both
await sr.pricing({ region: "il" });   // one, already flattened

// Several reads in one round trip
await sr.batch([
  { id: "caching", path: "/modules?q=caching" },
  { id: "price", path: "/pricing" },
]);

// Natural language, or the same thing streamed
await sr.ask("how do I make deploys safe");
for await (const ev of sr.askStream("caching")) console.log(ev.event);
```

## CLI

```
npx shipreal search caching
npx shipreal module observability
npx shipreal pricing --region il
npx shipreal ask "how do I make deploys safe"
```

Every command takes `--json`, because the reason a CLI earns a place in an
agent's toolbox is that its output pipes into something else without being
scraped:

```
npx shipreal search --all --json | jq '.[] | select(.chapters > 12) | .title'
```

`--sandbox` routes at frozen fixture data, which is what you want in a test:
the live catalogue moves whenever a module is added, and a suite that asserts
on real data breaks for reasons that have nothing to do with your code.

## Errors

Non-2xx responses throw `ShipRealError` carrying the server's
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details. Branch on
`err.type`, which is stable, rather than on the message, which is prose.

```js
import { ShipReal, ShipRealError } from "shipreal";

try {
  await new ShipReal().module("kubernetes");
} catch (err) {
  if (err instanceof ShipRealError && err.status === 404) {
    // The course does not cover it under that name. Say so; do not guess.
  }
}
```

## What there is no method for

Buying. Payment runs through a hosted checkout, and there is no endpoint that
takes money or creates an account. An agent helping someone enrol should hand
them the link and let them complete it themselves. That is a deliberate limit,
not a gap waiting to be filled.

## Other languages

The same client, same surface, in this repository:

| | | |
| --- | --- | --- |
| JavaScript | `npm install shipreal` | [`src/`](src) |
| Python | `pip install shipreal` | [`python/`](python) |
| Ruby | `gem install shipreal` | [`ruby/`](ruby) |
| Go | `go get github.com/mluggy/shipreal-dev/go/v2` | [`go/`](go) |

Each is standalone and dependency-free. The Go module is a submodule, so its
git tags carry the directory prefix (`go/v1.0.0`) even though the version you
request does not (`@v1.0.0`).

## Other surfaces

The same three capabilities are also available without this package:

| | |
| --- | --- |
| REST | [`/api/v1`](https://shipreal.dev/api/v1), [OpenAPI 3.1](https://shipreal.dev/openapi.json) |
| MCP | [`/mcp`](https://shipreal.dev/.well-known/mcp) — `search_curriculum`, `get_module`, `get_pricing` |
| MCP | [`/mcp/product`](https://shipreal.dev/mcp/product) — `quote_order`, `create_enrollment_link`, `check_enrollment_channel` |
| Natural language | [`/ask`](https://shipreal.dev/ask) (NLWeb, JSON or SSE) |
| Markdown | every page has a twin: `Accept: text/markdown`, or `?mode=agent` |
| Bulk | [modules.jsonl](https://shipreal.dev/feeds/modules.jsonl) |

Install the skills into your own agent:

```
npx skills add mluggy/shipreal-dev
```

This repository is also an [Agent Plugin](https://agent-plugins.org): the
manifest is [`plugin.json`](plugin.json), the MCP server is configured in
[`mcp.json`](mcp.json), and the five skills live under
[`skills/`](skills), mirrored at
[`/.well-known/agent-skills/`](https://shipreal.dev/.well-known/agent-skills/index.json).
Working notes for AI coding agents are in [AGENTS.md](AGENTS.md).

## Rate limit

600 requests a minute per address, counted per edge isolate. Every response
reports `RateLimit-Limit`, `RateLimit-Remaining` and `RateLimit-Reset`, and a
429 carries `Retry-After`. Because the count is per isolate rather than global,
treat those numbers as a pacing signal rather than a guarantee.

## Tests

```
npm test
```

They hit the live API on purpose: it is public, free and read-only, so there is
nothing to mock and a mock would only prove that the mock matches itself. They
skip rather than fail when the network is unavailable, and they assert on shape
and invariants rather than on counts or prices, which change legitimately.

## About

ShipReal is a 38 module course on production engineering for people who ship
software with AI assistance: [shipreal.dev](https://shipreal.dev).

This repository is the client only. The course, the site and the API
implementation live elsewhere.

MIT licensed.
