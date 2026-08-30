// Package shipreal is the official Go client for the public ShipReal course
// API: search the curriculum, read a module, read current pricing.
//
// There is no authentication. No key, no account, no OAuth, no signup: every
// endpoint is a public read served from the edge, so a client that asks you
// for a credential for this domain is not this one.
//
// There is also no write path. Buying runs through a hosted checkout that a
// human completes, which is why nothing here creates an order. When someone
// decides to buy, hand them the checkout link.
//
// Standard library only, on purpose: installing this cannot drag a transitive
// dependency tree into an agent's environment.
//
//	c := shipreal.New()
//	page, err := c.Search(ctx, shipreal.SearchParams{Query: "caching"})
package shipreal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production origin.
	DefaultBaseURL = "https://shipreal.dev"
	// APIVersion is the path segment this client speaks. Breaking changes
	// ship as a new segment beside it rather than as a change underneath;
	// every response carries a Deprecation header, false today.
	APIVersion = "v1"
	// Version of this client, sent in the User-Agent.
	Version = "1.0.1"

	// MaxBatch is the server's ceiling on requests in one batch call.
	MaxBatch = 20
)

const userAgent = "shipreal-go/" + Version + " (+" + DefaultBaseURL + "/developers)"

// Error is any non-2xx response, carrying the RFC 9457 problem details the
// server sent.
//
// Branch on Type rather than Status: the status says a request failed, the
// type says which failure it was, and only the second is stable enough to
// switch on.
type Error struct {
	Status int    `json:"status"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	// URL that produced the error.
	URL string `json:"-"`
}

func (e *Error) Error() string {
	switch {
	case e.Detail != "":
		return fmt.Sprintf("shipreal: %s (%d): %s", e.Title, e.Status, e.Detail)
	case e.Title != "":
		return fmt.Sprintf("shipreal: %s (%d)", e.Title, e.Status)
	default:
		return fmt.Sprintf("shipreal: request failed with %d", e.Status)
	}
}

// Module is one module of the curriculum.
type Module struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Part        string `json:"part"`
	Description string `json:"description"`
	Chapters    int    `json:"chapters"`
	Minutes     int    `json:"minutes"`
	// URL of the free video overview.
	URL string `json:"url"`
}

// Pagination describes the window a Page covers.
type Pagination struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	Pages   int  `json:"pages"`
	HasMore bool `json:"hasMore"`
	// NextCursor is opaque and carries its own page size. Prefer it to
	// incrementing Page: page numbers shift under a caller when the
	// catalogue changes between calls, a cursor does not.
	NextCursor *string `json:"nextCursor"`
}

// Links are absolute URLs for walking the pages.
type Links struct {
	Self  string  `json:"self"`
	First string  `json:"first"`
	Last  string  `json:"last"`
	Next  *string `json:"next"`
	Prev  *string `json:"prev"`
}

// Page is one page of module results.
type Page struct {
	Query      *string    `json:"query"`
	Data       []Module   `json:"data"`
	Pagination Pagination `json:"pagination"`
	Links      Links      `json:"links"`
}

// Course is the summary: totals, language and subtitle languages.
type Course struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Parts       int      `json:"parts"`
	Modules     int      `json:"modules"`
	Chapters    int      `json:"chapters"`
	Language    string   `json:"language"`
	Subtitles   []string `json:"subtitles"`
	URL         string   `json:"url"`
}

// RegionPrice is one region's current and pre-discount price, as the display
// strings they are quoted in, symbol included.
type RegionPrice struct {
	Now  string `json:"now"`
	List string `json:"list"`
}

// FreePlan is the permanent free tier. Not a trial: no account and no expiry.
type FreePlan struct {
	Price    int    `json:"price"`
	Currency string `json:"currency"`
	Includes string `json:"includes"`
	URL      string `json:"url"`
}

// Plan is a paid plan in both live billing regions.
type Plan struct {
	Intl RegionPrice `json:"intl"`
	IL   RegionPrice `json:"il"`
	URL  string      `json:"url,omitempty"`
	// MinSeats and PerSeat are only meaningful on the teams plan.
	MinSeats int  `json:"minSeats,omitempty"`
	PerSeat  bool `json:"perSeat,omitempty"`
}

// Pricing is every plan in every region. Two regional prices are live at
// once, so quoting one without naming its region is misleading: use Region to
// pick one deliberately.
type Pricing struct {
	Free     FreePlan `json:"free"`
	Complete Plan     `json:"complete"`
	Teams    Plan     `json:"teams"`
}

// Region returns the prices for "intl" or "il". Any other value returns the
// international region and false, rather than guessing.
func (p Pricing) Region(region string) (complete, perSeat RegionPrice, ok bool) {
	switch region {
	case "il":
		return p.Complete.IL, p.Teams.IL, true
	case "intl":
		return p.Complete.Intl, p.Teams.Intl, true
	default:
		return p.Complete.Intl, p.Teams.Intl, false
	}
}

// BatchRequest is one read inside a batch. Method is always GET; the API is
// read-only and a batch item that is not a read comes back 405.
type BatchRequest struct {
	ID     string `json:"id,omitempty"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path"`
}

// BatchItem is one result inside a batch response. Each carries its own
// status, so check per item rather than assuming the whole batch succeeded.
type BatchItem struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// BatchResponse is the envelope. It answers 200 whenever it is itself valid,
// which is why the per-item statuses matter.
type BatchResponse struct {
	Count     int         `json:"count"`
	Responses []BatchItem `json:"responses"`
}

// AskMeta is the NLWeb _meta block naming the response type and version.
type AskMeta struct {
	ResponseType string `json:"response_type"`
	Version      string `json:"version"`
	Query        string `json:"query"`
	Site         string `json:"site"`
	Count        int    `json:"count"`
}

// AskResult is one natural-language hit, with its schema.org object attached.
type AskResult struct {
	Type         string          `json:"@type"`
	Name         string          `json:"name"`
	URL          string          `json:"url"`
	Description  string          `json:"description"`
	Score        float64         `json:"score"`
	Site         string          `json:"site"`
	SchemaObject json.RawMessage `json:"schema_object"`
}

// AskResponse is the NLWeb shape: a question in, structured results out.
type AskResponse struct {
	Meta    AskMeta     `json:"_meta"`
	Results []AskResult `json:"results"`
}

// AskEvent is one server-sent event from AskStream: "start", then one
// "result" per hit, then "complete".
type AskEvent struct {
	Event   string      `json:"-"`
	Meta    AskMeta     `json:"_meta"`
	Results []AskResult `json:"results"`
}

// SearchParams are the query, the window, and the cursor. All optional: an
// empty Query returns every module in course order.
type SearchParams struct {
	Query string
	Page  int
	Limit int
	// Cursor overrides Page and Limit when set, because it carries the page
	// size it was minted with.
	Cursor string
}

// Client talks to the ShipReal API. The zero value is not usable; call New.
type Client struct {
	baseURL string
	sandbox bool
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the client at a different origin, e.g. a local worker.
func WithBaseURL(base string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(base, "/") }
}

// WithSandbox routes reads at the frozen fixture data: the same code path and
// the same shapes over contents that never change, so a test written against
// it stays green when the curriculum moves. Fixture prices are 1 unit and
// fixture links point at example.invalid, so sandbox data that leaks into real
// output is obvious. See https://shipreal.dev/sandbox.
func WithSandbox(sandbox bool) Option {
	return func(c *Client) { c.sandbox = sandbox }
}

// WithHTTPClient supplies your own http.Client, for a custom transport or
// timeout.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New builds a client against the production origin.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) apiURL(path string, query url.Values) string {
	prefix := "/api/" + APIVersion
	if c.sandbox {
		prefix += "/sandbox"
	}
	u := c.baseURL + prefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

func (c *Client) do(ctx context.Context, method, u string, body any, accept string, out any) error {
	res, err := c.send(ctx, method, u, body, accept)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// send returns a live response with an undrained body, so AskStream can read
// it incrementally. Every caller is responsible for closing it.
func (c *Client) send(ctx context.Context, method, u string, body any, accept string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", accept)
	req.Header.Set("user-agent", userAgent)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		defer res.Body.Close()
		// The error body is where the problem details live, so it gets
		// parsed rather than discarded. A body that is not JSON is not
		// itself worth raising over: the status still stands.
		apiErr := &Error{Status: res.StatusCode, URL: u}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if readErr == nil {
			_ = json.Unmarshal(raw, apiErr)
			apiErr.Status = res.StatusCode
		}
		return nil, apiErr
	}
	return res, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, c.apiURL(path, query), nil, "application/json", out)
}

// Search returns one page of the curriculum.
//
// Matching is a case-insensitive substring over title, description and part
// name, so an empty result means the course does not cover that topic under
// that name, rather than that the search was too clever.
func (c *Client) Search(ctx context.Context, params SearchParams) (*Page, error) {
	query := url.Values{}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.Page > 0 {
		query.Set("page", strconv.Itoa(params.Page))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Cursor != "" {
		query.Set("cursor", params.Cursor)
	}
	var page Page
	if err := c.get(ctx, "/modules", query, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Modules returns every matching module, following pagination for you. An
// empty query returns the whole curriculum in course order.
func (c *Client) Modules(ctx context.Context, query string) ([]Module, error) {
	var all []Module
	cursor := ""
	for {
		page, err := c.Search(ctx, SearchParams{Query: query, Limit: 100, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if page.Pagination.NextCursor == nil || *page.Pagination.NextCursor == "" {
			return all, nil
		}
		cursor = *page.Pagination.NextCursor
	}
}

// Module returns one module by slug, or by an exact or partial title match.
func (c *Client) Module(ctx context.Context, slugOrTitle string) (*Module, error) {
	if slugOrTitle == "" {
		return nil, fmt.Errorf("shipreal: Module needs a slug or title")
	}
	var m Module
	if err := c.get(ctx, "/modules/"+url.PathEscape(slugOrTitle), nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Pricing returns current plans and prices for both billing regions.
func (c *Client) Pricing(ctx context.Context) (*Pricing, error) {
	var p Pricing
	if err := c.get(ctx, "/pricing", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Course returns the totals, language and subtitle languages.
func (c *Client) Course(ctx context.Context) (*Course, error) {
	var course Course
	if err := c.get(ctx, "/course", nil, &course); err != nil {
		return nil, err
	}
	return &course, nil
}

// Batch runs several reads in one round trip, up to MaxBatch.
func (c *Client) Batch(ctx context.Context, requests []BatchRequest) (*BatchResponse, error) {
	if len(requests) > MaxBatch {
		return nil, fmt.Errorf("shipreal: Batch takes at most %d requests, got %d", MaxBatch, len(requests))
	}
	body := map[string]any{"requests": requests}
	var out BatchResponse
	u := c.baseURL + "/api/" + APIVersion + "/batch"
	if err := c.do(ctx, http.MethodPost, u, body, "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Ask puts a natural-language question to the NLWeb endpoint.
//
// There is no model behind it: it runs the same keyword search as the REST
// API, which means it says so when nothing matches instead of inventing a
// module.
func (c *Client) Ask(ctx context.Context, query string) (*AskResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("shipreal: Ask needs a question")
	}
	var out AskResponse
	if err := c.do(ctx, http.MethodPost, c.baseURL+"/ask", map[string]string{"query": query}, "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AskStream is the same question, streamed. It calls fn for each server-sent
// event as it arrives and stops early if fn returns an error, which it
// returns unchanged.
func (c *Client) AskStream(ctx context.Context, query string, fn func(AskEvent) error) error {
	if query == "" {
		return fmt.Errorf("shipreal: AskStream needs a question")
	}
	res, err := c.send(ctx, http.MethodPost, c.baseURL+"/ask", map[string]string{"query": query}, "text/event-stream")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	event, data := "message", ""
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "":
			// A blank line closes an SSE frame. Anything short of one is
			// a partial frame and has to wait for the next line.
			if data != "" {
				parsed := AskEvent{Event: event}
				if json.Unmarshal([]byte(data), &parsed) == nil {
					if err := fn(parsed); err != nil {
						return err
					}
				}
			}
			event, data = "message", ""
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimSpace(line[len("data:"):])
		}
	}
	return scanner.Err()
}
