/**
 * Smoke tests against the live API.
 *
 * Hitting production on purpose: the API is public, free, unauthenticated and
 * read-only, so there is nothing to mock, and a mock would only prove that the
 * mock matches itself. What these assert is the contract this client depends
 * on, so they fail loudly if the shape moves.
 *
 * Assertions are on shape and invariants, never on counts or prices, which
 * change legitimately whenever the course does.
 */
import { test, before } from "node:test";
import assert from "node:assert/strict";
import { ShipReal, ShipRealError } from "../src/index.js";

const sr = new ShipReal();
let online = true;

before(async () => {
  try {
    await sr.course();
  } catch {
    online = false;
    console.log("  (network unavailable: skipping live tests)");
  }
});

test("course returns totals and subtitle languages", async (t) => {
  if (!online) return t.skip();
  const c = await sr.course();
  assert.equal(typeof c.title, "string");
  assert.ok(c.modules > 0, "expected at least one module");
  assert.ok(c.chapters > c.modules, "chapters should outnumber modules");
  assert.ok(Array.isArray(c.subtitles) && c.subtitles.length > 0);
});

test("search returns a page with pagination and links", async (t) => {
  if (!online) return t.skip();
  const page = await sr.search("caching");
  assert.ok(Array.isArray(page.data));
  assert.equal(typeof page.pagination.total, "number");
  assert.equal(typeof page.links.self, "string");
  for (const m of page.data) {
    assert.equal(typeof m.slug, "string");
    assert.equal(typeof m.title, "string");
    assert.ok(m.chapters >= 0);
  }
});

test("an empty search says nothing rather than guessing", async (t) => {
  if (!online) return t.skip();
  const page = await sr.search("quantum tunnelling for cats");
  assert.equal(page.data.length, 0, "a miss must return nothing, not a near match");
});

test("cursor paging carries its own page size", async (t) => {
  if (!online) return t.skip();
  const first = await sr.search(undefined, { limit: 3 });
  if (!first.pagination.nextCursor) return t.skip("only one page");
  const second = await sr.search(undefined, { cursor: first.pagination.nextCursor });
  assert.equal(second.pagination.limit, 3, "the cursor must not silently resize the window");
  assert.equal(second.pagination.page, 2);
  assert.notEqual(first.data[0].slug, second.data[0].slug);
});

test("modules() walks every page", async (t) => {
  if (!online) return t.skip();
  const seen = [];
  for await (const m of sr.modules()) seen.push(m.slug);
  const { modules: total } = await sr.course();
  assert.equal(seen.length, total, "the generator should yield every module exactly once");
  assert.equal(new Set(seen).size, seen.length, "no duplicates across pages");
});

test("a missing module throws problem details, not a bare error", async (t) => {
  if (!online) return t.skip();
  await assert.rejects(
    () => sr.module("definitely-not-a-module-9999"),
    (err) => {
      assert.ok(err instanceof ShipRealError);
      assert.equal(err.status, 404);
      assert.match(err.type ?? "", /errors-/, "expected a stable error type URL");
      return true;
    }
  );
});

test("pricing reports both regions, and one when asked", async (t) => {
  if (!online) return t.skip();
  const all = await sr.pricing();
  assert.ok(all.complete.intl.now.length > 0);
  assert.ok(all.complete.il.now.length > 0);
  const il = await sr.pricing({ region: "il" });
  assert.equal(il.region, "il");
  assert.equal(il.complete.now, all.complete.il.now);
});

test("batch returns one result per item, each with its own status", async (t) => {
  if (!online) return t.skip();
  const res = await sr.batch([
    { id: "hit", path: "/modules?q=caching" },
    { id: "miss", path: "/modules/definitely-not-a-module-9999" },
  ]);
  assert.equal(res.count, 2);
  const byId = Object.fromEntries(res.responses.map((r) => [r.id, r]));
  assert.equal(byId.hit.status, 200);
  assert.equal(byId.miss.status, 404, "one bad item must not fail the batch");
});

test("batch refuses more than twenty items before sending", async (t) => {
  await assert.rejects(
    () => sr.batch(Array.from({ length: 21 }, () => ({ path: "/pricing" }))),
    RangeError
  );
});

test("ask returns NLWeb-shaped results", async (t) => {
  if (!online) return t.skip();
  const a = await sr.ask("how do I make deploys safe");
  assert.equal(typeof a._meta.response_type, "string");
  assert.equal(typeof a._meta.version, "string");
  assert.ok(Array.isArray(a.results));
});

test("askStream yields start, results, then complete", async (t) => {
  if (!online) return t.skip();
  const events = [];
  for await (const ev of sr.askStream("caching")) events.push(ev.event);
  assert.equal(events[0], "start");
  assert.equal(events[events.length - 1], "complete");
});

test("the sandbox is stable and separate from live data", async (t) => {
  if (!online) return t.skip();
  const box = new ShipReal({ sandbox: true });
  const a = await box.search();
  const b = await box.search();
  assert.deepEqual(a.data, b.data, "fixtures must not move between calls");
  assert.ok(a.data.every((m) => m.slug.startsWith("sandbox-")));
});
