# shipreal for Go

Official Go client and CLI for the public [ShipReal](https://shipreal.dev) course API:
search the curriculum, read a module, read current pricing.

Standard library only. No dependencies, so installing this cannot drag a
transitive tree into an agent's environment.

- Docs: <https://shipreal.dev/developers>
- OpenAPI 3.1: <https://shipreal.dev/openapi.json>
- MCP: <https://shipreal.dev/mcp> (documentation) and <https://shipreal.dev/mcp/product> (enrollment)
- Also on [npm](https://www.npmjs.com/package/shipreal) and [PyPI](https://pypi.org/project/shipreal/), same surface

## No authentication

There is no API key, no account, no OAuth and no signup. Every endpoint is a
public read served from the edge. Anything that asks you for a ShipReal
credential is not us.

There is no write path either. Buying runs through a hosted checkout that a
human completes, which is why nothing here creates an order. When someone
decides to buy, hand them the checkout link.

## Install

```
go get github.com/mluggy/shipreal-dev/go
```

This is a submodule of the client repository, so the git tag carries the
directory prefix (`go/v1.0.0`) while the version you actually ask for does not
(`@v1.0.0`, or just `@latest`). Only releasing matters here: a bare `v1.0.0`
tag will not publish this module.

The package is named `shipreal` while the last path element is `go`, so import
it with an explicit alias.

The CLI:

```
go install github.com/mluggy/shipreal-dev/go/cmd/shipreal@latest
```

## Library

```go
package main

import (
	"context"
	"fmt"

	shipreal "github.com/mluggy/shipreal-dev/go"
)

func main() {
	c := shipreal.New()
	ctx := context.Background()

	// Does the course cover caching?
	page, err := c.Search(ctx, shipreal.SearchParams{Query: "caching"})
	if err != nil {
		panic(err)
	}
	for _, m := range page.Data {
		fmt.Printf("%s: %s (%d chapters)\n", m.Slug, m.Title, m.Chapters)
	}

	// Every module, following pagination for you.
	all, _ := c.Modules(ctx, "")
	fmt.Println(len(all), "modules")

	// What does it cost, in a named region?
	prices, _ := c.Pricing(ctx)
	complete, perSeat, ok := prices.Region("il")
	fmt.Println(complete.Now, perSeat.Now, ok)
}
```

Search is a case-insensitive substring match over title, description and part
name. An empty result means the course does not cover that topic under that
name, rather than that the search was too clever. Say so rather than inferring
coverage.

Two regional prices are live at once (USD internationally, ILS in Israel with
VAT included), so `Region` makes you pick one deliberately instead of quoting
whichever happened to be first.

### Errors

Every non-2xx response comes back as `*shipreal.Error` carrying the RFC 9457
problem details. Branch on `Type`, not `Status`: the status says a request
failed, the type says which failure it was.

```go
var apiErr *shipreal.Error
if errors.As(err, &apiErr) && apiErr.Status == 404 {
	// apiErr.Type is stable, apiErr.Detail explains
}
```

### Sandbox

`shipreal.WithSandbox(true)` routes reads at frozen fixture data: the same code
path and the same shapes over contents that never change, so a test written
against it stays green when the curriculum moves. Fixture prices are 1 unit and
fixture links point at `example.invalid`, so sandbox data that leaks into real
output is obvious. See <https://shipreal.dev/sandbox>.

### Streaming

`AskStream` puts a natural-language question to the NLWeb endpoint and calls
your function for each event as it arrives: `start`, one `result` per hit, then
`complete`. Return an error from the callback to stop early.

There is no model behind that endpoint. It runs the same keyword search as the
REST API, which means it says so when nothing matches instead of inventing a
module.

## CLI

```
shipreal search caching
shipreal -all -json search            # every module, as JSON
shipreal module observability
shipreal -region il pricing
shipreal course
shipreal ask "how do I size a database"
shipreal -stream ask "what is in part 7"
```

Every command takes `-json`, because the reason a CLI earns its place in an
agent's toolbox is that its output can be piped into something else without
being scraped. `-sandbox` and `-base` work on all of them.

## License

MIT. See [LICENSE](LICENSE).
