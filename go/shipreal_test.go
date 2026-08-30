package shipreal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Fixtures deliberately mirror the sandbox: same shapes, frozen contents.
var moduleOne = Module{
	Slug: "sandbox-module-1", Title: "Sandbox: Thinking in Systems & State",
	Part: "Sandbox Part 1: The Mental Shift", Description: "A fixed fixture module.",
	Chapters: 12, Minutes: 29, URL: "https://example.invalid/sandbox-module-1",
}

func server(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL)), srv
}

func TestSearchSendsQueryAndParsesPage(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/modules" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "caching" {
			t.Errorf("q = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit = %q", got)
		}
		if !strings.HasPrefix(r.Header.Get("user-agent"), "shipreal-go/") {
			t.Errorf("user-agent = %q", r.Header.Get("user-agent"))
		}
		_ = json.NewEncoder(w).Encode(Page{Data: []Module{moduleOne}})
	})
	page, err := c.Search(context.Background(), SearchParams{Query: "caching", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].Slug != moduleOne.Slug {
		t.Fatalf("data = %+v", page.Data)
	}
}

func TestSandboxRoutesUnderSandboxPrefix(t *testing.T) {
	var seen string
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		_ = json.NewEncoder(w).Encode(Page{})
	})
	c.sandbox = true
	if _, err := c.Search(context.Background(), SearchParams{}); err != nil {
		t.Fatal(err)
	}
	if seen != "/api/v1/sandbox/modules" {
		t.Fatalf("path = %q", seen)
	}
}

func TestModulesFollowsCursorAndStops(t *testing.T) {
	calls := 0
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		cursor := r.URL.Query().Get("cursor")
		next := "page2"
		if cursor == "" {
			_ = json.NewEncoder(w).Encode(Page{
				Data:       []Module{moduleOne},
				Pagination: Pagination{HasMore: true, NextCursor: &next},
			})
			return
		}
		if cursor != "page2" {
			t.Errorf("cursor = %q", cursor)
		}
		_ = json.NewEncoder(w).Encode(Page{Data: []Module{{Slug: "second"}}})
	})
	all, err := c.Modules(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(all) != 2 || all[1].Slug != "second" {
		t.Fatalf("modules = %+v", all)
	}
}

// An empty-string cursor has to terminate the loop as surely as a null one, or
// a server that sends one spins the client forever.
func TestModulesStopsOnEmptyCursor(t *testing.T) {
	calls := 0
	empty := ""
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(Page{
			Data:       []Module{moduleOne},
			Pagination: Pagination{HasMore: true, NextCursor: &empty},
		})
	})
	if _, err := c.Modules(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestModuleEscapesTitleFragments(t *testing.T) {
	var seen string
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.EscapedPath()
		_ = json.NewEncoder(w).Encode(moduleOne)
	})
	if _, err := c.Module(context.Background(), "systems & state"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "%20&%20") && !strings.Contains(seen, "systems") {
		t.Fatalf("escaped path = %q", seen)
	}
}

func TestProblemDetailsBecomeError(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"https://shipreal.dev/developers#errors-module-not-found",` +
			`"title":"Module Not Found","status":404,"detail":"No module matches \"nope\"."}`))
	})
	_, err := c.Module(context.Background(), "nope")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v (%T)", err, err)
	}
	if apiErr.Status != 404 {
		t.Errorf("status = %d", apiErr.Status)
	}
	if !strings.HasSuffix(apiErr.Type, "#errors-module-not-found") {
		t.Errorf("type = %q", apiErr.Type)
	}
	if !strings.Contains(apiErr.Error(), "Module Not Found") {
		t.Errorf("message = %q", apiErr.Error())
	}
}

// A non-JSON error body must not swallow the status, which is the only thing
// the caller can still act on.
func TestNonJSONErrorBodyKeepsStatus(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream fell over"))
	})
	_, err := c.Course(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 502 {
		t.Fatalf("err = %v", err)
	}
}

func TestPricingRegionPicksDeliberately(t *testing.T) {
	p := Pricing{
		Complete: Plan{Intl: RegionPrice{Now: "$99"}, IL: RegionPrice{Now: "₪299"}},
		Teams:    Plan{Intl: RegionPrice{Now: "$119"}, IL: RegionPrice{Now: "₪359"}, MinSeats: 1},
	}
	complete, seat, ok := p.Region("il")
	if !ok || complete.Now != "₪299" || seat.Now != "₪359" {
		t.Fatalf("il = %v %v %v", complete, seat, ok)
	}
	// An unknown region falls back to international and says so, rather than
	// quietly quoting a price for the wrong country.
	if _, _, ok := p.Region("uk"); ok {
		t.Fatal("unknown region reported ok")
	}
}

func TestBatchRefusesOverTheCeiling(t *testing.T) {
	c := New()
	reqs := make([]BatchRequest, MaxBatch+1)
	if _, err := c.Batch(context.Background(), reqs); err == nil {
		t.Fatal("want an error for an oversized batch")
	}
}

func TestBatchPostsRequests(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		var body struct {
			Requests []BatchRequest `json:"requests"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Requests) != 1 || body.Requests[0].Path != "/pricing" {
			t.Errorf("requests = %+v", body.Requests)
		}
		_ = json.NewEncoder(w).Encode(BatchResponse{Count: 1, Responses: []BatchItem{{ID: "0", Status: 200}}})
	})
	out, err := c.Batch(context.Background(), []BatchRequest{{Path: "/pricing"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 {
		t.Fatalf("count = %d", out.Count)
	}
}

func TestAskStreamYieldsEventsInOrder(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("accept"); got != "text/event-stream" {
			t.Errorf("accept = %q", got)
		}
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte(
			"event: start\ndata: {\"_meta\":{\"response_type\":\"result_list\",\"count\":1}}\n\n" +
				"event: result\ndata: {\"results\":[{\"name\":\"Caching\"}]}\n\n" +
				"event: complete\ndata: {\"_meta\":{\"response_type\":\"complete\"}}\n\n"))
	})
	var events []string
	var names []string
	err := c.AskStream(context.Background(), "caching", func(e AskEvent) error {
		events = append(events, e.Event)
		for _, r := range e.Results {
			names = append(names, r.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, ",") != "start,result,complete" {
		t.Fatalf("events = %v", events)
	}
	if len(names) != 1 || names[0] != "Caching" {
		t.Fatalf("names = %v", names)
	}
}

func TestAskStreamStopsWhenCallbackErrors(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("event: start\ndata: {}\n\nevent: result\ndata: {}\n\n"))
	})
	stop := errors.New("enough")
	seen := 0
	err := c.AskStream(context.Background(), "caching", func(AskEvent) error {
		seen++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v", err)
	}
	if seen != 1 {
		t.Fatalf("callback ran %d times, want 1", seen)
	}
}

func TestEmptyArgumentsAreRejectedBeforeAnyRequest(t *testing.T) {
	c, srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should have been sent")
	})
	_ = srv
	if _, err := c.Module(context.Background(), ""); err == nil {
		t.Error("Module accepted an empty argument")
	}
	if _, err := c.Ask(context.Background(), ""); err == nil {
		t.Error("Ask accepted an empty question")
	}
	if err := c.AskStream(context.Background(), "", func(AskEvent) error { return nil }); err == nil {
		t.Error("AskStream accepted an empty question")
	}
}
