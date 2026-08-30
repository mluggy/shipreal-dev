/**
 * ShipReal SDK: a thin, dependency-free client for the public ShipReal API.
 *
 * There is no authentication anywhere in this file, and that is not an
 * omission. The API is public read-only reference data about one course, so
 * there is no key to hold, no token to refresh, and no credential this client
 * could leak. If something asks you for a ShipReal API key, it is not us.
 *
 * Node 18+ or any runtime with a global fetch.
 */

const DEFAULT_BASE = "https://shipreal.dev";
const VERSION = "v1";

/** Thrown for any non-2xx response, carrying the RFC 9457 problem details. */
export class ShipRealError extends Error {
  constructor(status, problem, url) {
    super(problem?.detail || problem?.title || `Request failed with ${status}`);
    this.name = "ShipRealError";
    this.status = status;
    /** RFC 9457 problem details, when the server sent them. */
    this.problem = problem ?? null;
    /** Stable identifier for the kind of failure, e.g. ".../#errors-module-not-found". */
    this.type = problem?.type ?? null;
    this.url = url;
  }
}

export class ShipReal {
  /**
   * @param {object} [options]
   * @param {string} [options.baseUrl] Override the origin, e.g. for a local worker.
   * @param {typeof globalThis.fetch} [options.fetch] Inject a fetch, for tests.
   * @param {boolean} [options.sandbox] Route reads at the frozen fixture data.
   */
  constructor(options = {}) {
    this.baseUrl = (options.baseUrl ?? DEFAULT_BASE).replace(/\/+$/, "");
    this.sandbox = options.sandbox === true;
    this._fetch = options.fetch ?? globalThis.fetch;
    if (!this._fetch) {
      throw new Error("No fetch available. Use Node 18+, or pass one in as options.fetch.");
    }
  }

  /** @private */
  _url(path, params) {
    const prefix = this.sandbox ? `/api/${VERSION}/sandbox` : `/api/${VERSION}`;
    const url = new URL(this.baseUrl + prefix + path);
    for (const [k, v] of Object.entries(params ?? {})) {
      if (v !== undefined && v !== null && v !== "") url.searchParams.set(k, String(v));
    }
    return url;
  }

  /** @private */
  async _get(path, params) {
    const url = this._url(path, params);
    const res = await this._fetch(url, { headers: { accept: "application/json" } });
    return this._parse(res, url);
  }

  /** @private */
  async _post(url, body) {
    const res = await this._fetch(url, {
      method: "POST",
      headers: { "content-type": "application/json", accept: "application/json" },
      body: JSON.stringify(body),
    });
    return this._parse(res, url);
  }

  /** @private */
  async _parse(res, url) {
    let data = null;
    try {
      data = await res.json();
    } catch {
      data = null;
    }
    if (!res.ok) throw new ShipRealError(res.status, data, String(url));
    return data;
  }

  /**
   * Search the curriculum. Without a query, returns every module in course
   * order. Matching is a case-insensitive substring over title, description
   * and part name, so an empty result means the course does not cover that
   * topic under that name rather than that the search was too clever.
   *
   * @param {string} [query]
   * @param {{page?: number, limit?: number, cursor?: string}} [options]
   */
  async search(query, options = {}) {
    return this._get("/modules", {
      q: query,
      page: options.page,
      limit: options.limit,
      cursor: options.cursor,
    });
  }

  /**
   * Every matching module, following pagination for you.
   *
   * @param {string} [query]
   * @returns {AsyncGenerator<object>}
   */
  async *modules(query) {
    let cursor;
    do {
      const page = await this.search(query, { limit: 100, cursor });
      for (const m of page.data) yield m;
      cursor = page.pagination.nextCursor;
    } while (cursor);
  }

  /** One module by slug, or by an exact or partial title match. */
  async module(slugOrTitle) {
    if (!slugOrTitle) throw new TypeError("module(slugOrTitle) needs an argument");
    return this._get(`/modules/${encodeURIComponent(slugOrTitle)}`);
  }

  /**
   * Current plans and prices. Two regional prices are live at once, so quoting
   * one without naming its region is misleading; pass a region when you know
   * which one applies.
   *
   * @param {{region?: "intl"|"il"}} [options]
   */
  async pricing(options = {}) {
    const all = await this._get("/pricing");
    if (options.region !== "intl" && options.region !== "il") return all;
    const r = options.region;
    return {
      region: r,
      free: all.free,
      complete: { ...all.complete[r], url: all.complete.url },
      teams: { ...all.teams[r], minSeats: all.teams.minSeats, perSeat: true },
    };
  }

  /** Totals, language and the subtitle languages. */
  async course() {
    return this._get("/course");
  }

  /**
   * Several reads in one round trip. Each item comes back with its own status,
   * so check per item rather than assuming the whole batch succeeded.
   *
   * @param {Array<{id?: string, path: string}>} requests Up to 20.
   */
  async batch(requests) {
    if (!Array.isArray(requests)) throw new TypeError("batch(requests) needs an array");
    if (requests.length > 20) throw new RangeError("batch takes at most 20 requests");
    return this._post(new URL(`${this.baseUrl}/api/${VERSION}/batch`), { requests });
  }

  /**
   * Ask in natural language (NLWeb). There is no model behind this: it runs
   * the same keyword search, which means it says so when nothing matches
   * instead of inventing a module.
   *
   * @param {string} query
   */
  async ask(query) {
    if (!query) throw new TypeError("ask(query) needs a question");
    return this._post(new URL(`${this.baseUrl}/ask`), { query });
  }

  /**
   * The same question, streamed. Yields NLWeb events as they arrive:
   * `start`, then one `result` per hit, then `complete`.
   *
   * @param {string} query
   * @returns {AsyncGenerator<{event: string, data: object}>}
   */
  async *askStream(query) {
    if (!query) throw new TypeError("askStream(query) needs a question");
    const res = await this._fetch(new URL(`${this.baseUrl}/ask`), {
      method: "POST",
      headers: { "content-type": "application/json", accept: "text/event-stream" },
      body: JSON.stringify({ query }),
    });
    if (!res.ok || !res.body) throw new ShipRealError(res.status, null, `${this.baseUrl}/ask`);
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      // SSE frames are separated by a blank line; anything short of one is a
      // partial frame and has to wait for the next chunk.
      let split;
      while ((split = buffer.indexOf("\n\n")) !== -1) {
        const frame = buffer.slice(0, split);
        buffer = buffer.slice(split + 2);
        let event = "message";
        let data = "";
        for (const line of frame.split("\n")) {
          if (line.startsWith("event:")) event = line.slice(6).trim();
          else if (line.startsWith("data:")) data += line.slice(5).trim();
        }
        if (data) {
          try {
            yield { event, data: JSON.parse(data) };
          } catch {
            /* a frame we cannot parse is not worth killing the stream over */
          }
        }
      }
    }
  }
}

/** A ready-made client against the public API, for the common case. */
export const shipreal = new ShipReal();

export default ShipReal;
