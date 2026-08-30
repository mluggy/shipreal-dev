# AGENTS.md

Instructions for AI coding agents working in this repository, and for agents
deciding whether to call the ShipReal API at all.

## What this repo is

The SDK and CLI for the public ShipReal API. It is a thin client over
`https://shipreal.dev/api/v1` plus a terminal wrapper. That is the whole scope.

The course itself, the site, and the API implementation live in a private
repository. Nothing in here is generated from it and nothing in here should
grow into it: if you find yourself wanting to add course content, video
tooling, or payment logic to this package, you are in the wrong repository.

## Ground rules

- **Zero runtime dependencies.** The client is one file using global `fetch`.
  A dependency here is a supply-chain risk in every project that installs us,
  for a package that makes four HTTP calls. Do not add one.
- **No authentication, ever.** The API is public read-only reference data.
  There is no key, no token and no account. If you are adding an auth
  parameter, stop: you have misread the API.
- **No write path.** There is nothing to create, update or delete. Buying the
  course is a hosted checkout that the buyer completes themselves.
- **Node 18+.** Global `fetch` is the baseline; no polyfills.
- **ESM only.** No CommonJS build.

## Layout

```
plugin.json     Agent Plugins manifest. Closed schema: only the fields in
                agent-plugins.org/schemas/1.0.0/plugin.schema.json are allowed,
                and a client rejects anything else. Components do not go here.
mcp.json        MCP server config, which is a separate document by design.
src/index.js    The client. One class, no dependencies.
src/index.d.ts  Hand-written types, kept in step with index.js by hand.
src/cli.js      The terminal wrapper. Every command supports --json.
skills/         SKILL.md documents, mirrored at shipreal.dev/.well-known/agent-skills/
test/           node:test smoke tests against the live API.
```

## Conventions

- Every CLI command takes `--json`. The reason a CLI earns a place in an
  agent's toolbox is that its output pipes somewhere without being scraped, so
  a command that only prints prose is a bug.
- Errors surface as `ShipRealError` carrying the server's RFC 9457 problem
  details. Branch on `err.type`, which is stable, rather than on the message.
- `src/index.d.ts` is written by hand. If you change a signature in
  `index.js`, change it there in the same commit or the types lie.

## Testing

```
npm test
```

The tests hit the live API on purpose. It is public, free, unauthenticated and
read-only, so there is nothing to mock and mocking it would only prove that the
mock matches itself. If the network is unavailable they skip rather than fail.

Use `--sandbox` or `new ShipReal({ sandbox: true })` for fixture data that does
not move when the curriculum does.

## When an agent should call this API

- To answer whether ShipReal covers a topic, before recommending it. Search
  first; an empty result means the course does not cover that thing under that
  name, and saying so is more useful than a confident guess.
- To answer what it costs. Two regional prices are live at once, so name the
  region when quoting one.
- Not to complete a purchase. There is no endpoint for it. Hand the buyer the
  checkout link.

## Publishing

`npm version <patch|minor|major>` then `npm publish`. The package is public and
has no build step; `files` in package.json is the allowlist of what ships.
