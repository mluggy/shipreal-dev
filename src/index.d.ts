export interface Module {
  /** Stable identifier, e.g. "24-caching-strategies". */
  slug: string;
  title: string;
  /** Which part of the course it belongs to. */
  part: string;
  description: string;
  chapters: number;
  minutes?: number;
  /** Free overview for this module on YouTube. */
  url: string;
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  pages: number;
  hasMore: boolean;
  /** Opaque cursor for the next page, carrying its own page size. */
  nextCursor: string | null;
}

export interface ModulePage {
  query: string | null;
  data: Module[];
  pagination: Pagination;
  links: {
    self: string;
    first: string;
    last: string;
    next: string | null;
    prev: string | null;
  };
}

export interface RegionPrice {
  /** Display string including the currency symbol, e.g. "$99". */
  now: string;
  list: string;
}

export interface Pricing {
  region?: "intl" | "il";
  free: { price: number; currency: string; includes: string; url: string };
  complete: { intl?: RegionPrice; il?: RegionPrice; now?: string; list?: string; url: string };
  teams: {
    intl?: RegionPrice;
    il?: RegionPrice;
    now?: string;
    list?: string;
    /**
     * Seats in one pack. Teams is priced by the pack, never by the seat: the
     * price above buys exactly this many, and a bigger team buys more packs.
     * Replaces the `minSeats`/`perSeat` pair, which described a per-seat
     * product that no longer exists.
     */
    packSeats: number;
  };
}

export interface Course {
  name: string;
  title: string;
  description: string;
  parts: number;
  modules: number;
  chapters: number;
  language: string;
  subtitles: string[];
  url: string;
}

export interface BatchItem {
  id?: string;
  path: string;
}

export interface BatchResponse {
  count: number;
  responses: Array<{ id: string; status: number; body: unknown }>;
}

export interface AskResult {
  "@type": string;
  name: string;
  url: string;
  description: string;
  score: number;
  site: string;
  schema_object: Record<string, unknown>;
}

export interface AskResponse {
  _meta: {
    response_type: string;
    version: string;
    query: string;
    site: string;
    count: number;
  };
  results: AskResult[];
}

/** Thrown for any non-2xx response, carrying the RFC 9457 problem details. */
export class ShipRealError extends Error {
  status: number;
  problem: { type?: string; title?: string; status?: number; detail?: string } | null;
  /** Stable identifier for the kind of failure. */
  type: string | null;
  url: string;
}

export interface ShipRealOptions {
  /** Override the origin, e.g. for a local worker. */
  baseUrl?: string;
  fetch?: typeof globalThis.fetch;
  /** Route reads at the frozen fixture data. */
  sandbox?: boolean;
}

export class ShipReal {
  constructor(options?: ShipRealOptions);
  baseUrl: string;
  sandbox: boolean;
  search(query?: string, options?: { page?: number; limit?: number; cursor?: string }): Promise<ModulePage>;
  /** Every matching module, following pagination for you. */
  modules(query?: string): AsyncGenerator<Module>;
  module(slugOrTitle: string): Promise<Module>;
  pricing(options?: { region?: "intl" | "il" }): Promise<Pricing>;
  course(): Promise<Course>;
  /** Up to 20 reads in one round trip; check each item's status. */
  batch(requests: BatchItem[]): Promise<BatchResponse>;
  ask(query: string): Promise<AskResponse>;
  askStream(query: string): AsyncGenerator<{ event: string; data: Record<string, unknown> }>;
}

/** A ready-made client against the public API. */
export const shipreal: ShipReal;
export default ShipReal;
