#!/usr/bin/env node
/**
 * shipreal: query the ShipReal course catalogue from a terminal or a script.
 *
 * Every command takes --json, because the reason a CLI earns its place in an
 * agent's toolbox is that its output can be piped into something else without
 * being scraped.
 */
import { ShipReal, ShipRealError } from "./index.js";

const USAGE = `shipreal - query the ShipReal course catalogue

Usage
  shipreal search [query]            Search modules by keyword
  shipreal module <slug-or-title>    One module
  shipreal pricing [--region i|il]   Plans and prices
  shipreal course                    Totals and subtitle languages
  shipreal ask <question>            Natural-language query
  shipreal ask --stream <question>   The same, streamed as it arrives

Options
  --json          Raw JSON, for piping
  --limit <n>     Results per page (search, default 20, max 100)
  --all           Every result, following pagination (search)
  --region <r>    intl or il (pricing)
  --sandbox       Frozen fixture data, for tests
  --base <url>    Point at a different origin
  -h, --help      This

No API key. No account. The API is public read-only reference data; anything
that asks you for a ShipReal credential is not us.

Docs: https://shipreal.dev/developers
`;

function parseArgs(argv) {
  const flags = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--json") flags.json = true;
    else if (a === "--all") flags.all = true;
    else if (a === "--stream") flags.stream = true;
    else if (a === "--sandbox") flags.sandbox = true;
    else if (a === "-h" || a === "--help") flags.help = true;
    else if (a === "--limit") flags.limit = Number(argv[++i]);
    else if (a === "--region") flags.region = argv[++i];
    else if (a === "--base") flags.base = argv[++i];
    else flags._.push(a);
  }
  return flags;
}

/* Plain text by default. A table would be prettier and worse: this output gets
   piped, and column padding is not something a downstream reader should have
   to undo. */
function printModules(mods) {
  if (!mods.length) {
    console.log("No module matches. The full curriculum is at https://shipreal.dev/llms-full.txt");
    return;
  }
  for (const m of mods) {
    console.log(`${m.title}`);
    console.log(`  ${m.part}`);
    console.log(`  ${m.description}`);
    console.log(`  ${m.chapters} chapters${m.minutes ? `, ~${m.minutes} min` : ""}  ${m.url}`);
    console.log();
  }
}

async function main() {
  const flags = parseArgs(process.argv.slice(2));
  const [command, ...rest] = flags._;
  if (flags.help || !command) {
    console.log(USAGE);
    process.exit(command ? 0 : 1);
  }

  const sr = new ShipReal({
    ...(flags.base ? { baseUrl: flags.base } : {}),
    sandbox: flags.sandbox === true,
  });
  const out = (v) => console.log(JSON.stringify(v, null, 2));

  switch (command) {
    case "search": {
      const query = rest.join(" ");
      if (flags.all) {
        const all = [];
        for await (const m of sr.modules(query)) all.push(m);
        flags.json ? out(all) : printModules(all);
        return;
      }
      const page = await sr.search(query, { limit: flags.limit });
      if (flags.json) return out(page);
      printModules(page.data);
      if (page.pagination.hasMore) {
        console.log(`${page.pagination.total} total. --all for the rest.`);
      }
      return;
    }
    case "module": {
      const m = await sr.module(rest.join(" "));
      if (flags.json) return out(m);
      printModules([m]);
      return;
    }
    case "pricing": {
      const p = await sr.pricing({ region: flags.region });
      if (flags.json) return out(p);
      const money = (x) => (x ? `${x.now} (list ${x.list})` : "n/a");
      console.log(`Free       $0, ${p.free.includes}`);
      if (p.region) {
        console.log(`Complete   ${money(p.complete)}  one-time`);
        console.log(`Teams      ${money(p.teams)}  per pack of ${p.teams.packSeats} seats`);
      } else {
        console.log(`Complete   intl ${money(p.complete.intl)} | il ${money(p.complete.il)}`);
        console.log(`Teams      intl ${money(p.teams.intl)} | il ${money(p.teams.il)}  per pack of ${p.teams.packSeats} seats`);
        console.log(`\nTwo regional prices are live at once; name the region when you quote one.`);
      }
      return;
    }
    case "course": {
      const c = await sr.course();
      if (flags.json) return out(c);
      console.log(`${c.title}`);
      console.log(`${c.parts} parts, ${c.modules} modules, ${c.chapters} chapters`);
      console.log(`Narration ${c.language}; subtitles: ${(c.subtitles || []).join(", ")}`);
      console.log(c.url);
      return;
    }
    case "ask": {
      const query = rest.join(" ");
      if (flags.stream) {
        for await (const ev of sr.askStream(query)) {
          if (flags.json) out(ev);
          else if (ev.event === "result") console.log(`- ${ev.data.results[0].name}`);
          else if (ev.event === "complete") console.log(`(${ev.data._meta.count} results)`);
        }
        return;
      }
      const a = await sr.ask(query);
      if (flags.json) return out(a);
      for (const r of a.results) console.log(`- ${r.name}\n  ${r.description}\n  ${r.url}\n`);
      if (!a.results.length) console.log("Nothing matches that. The course may simply not cover it.");
      return;
    }
    default:
      console.error(`Unknown command: ${command}\n`);
      console.log(USAGE);
      process.exit(1);
  }
}

main().catch((err) => {
  if (err instanceof ShipRealError) {
    console.error(`${err.status}  ${err.message}`);
    if (err.type) console.error(err.type);
  } else {
    console.error(err.message || String(err));
  }
  process.exit(1);
});
