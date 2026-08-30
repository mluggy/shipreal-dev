# shipreal

SDK and CLI for the public [ShipReal](https://shipreal.dev) course API: search
the curriculum, read a module, read pricing.

Zero dependencies, standard library only. No authentication: the API is public
read-only reference data about one course, so there is no key to hold and no
token to refresh. If something asks you for a ShipReal API key, it is not us.

```
pip install shipreal
```

## Library

```python
from shipreal import ShipReal

sr = ShipReal()

sr.search("caching")                  # one page of matches
list(sr.modules())                    # every module, pagination followed
sr.module("observability")            # by slug or partial title
sr.pricing(region="il")               # flattened to one billing region
sr.course()                           # totals and subtitle languages
sr.ask("how do I size a database")    # natural language, keyword search behind it
```

Errors carry [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem
details. Branch on `type`, not on `status`: the status says a request failed,
the type says which failure it was.

```python
from shipreal import ShipReal, ShipRealError

try:
    ShipReal().module("does-not-exist")
except ShipRealError as err:
    print(err.status, err.type, err)
```

## CLI

```
shipreal search caching
shipreal search --all --json
shipreal module observability
shipreal pricing --region il
shipreal ask "what should I learn before deploying"
```

## Testing against the sandbox

Frozen fixture data over the same code path, so a test written against it stays
green when the course changes:

```python
sr = ShipReal(sandbox=True)
```

Fixture prices are 1 unit and fixture links point at `example.invalid`, so
sandbox data that leaks into real output is obvious on sight. Assert on shape,
pagination and error format, not on prices or titles. Details at
[shipreal.dev/sandbox](https://shipreal.dev/sandbox).

## There is nothing to write

The API has no write path and no purchase endpoint in either environment.
Enrollment runs through a hosted checkout that a person completes, so if a task
needs a transaction the answer is a link for a human, not a call.

## Other surfaces

REST is one of several. There is an
[OpenAPI 3.1 spec](https://shipreal.dev/openapi.json), an MCP server at
[/mcp](https://shipreal.dev/mcp), and a
[JavaScript client](https://www.npmjs.com/package/shipreal) with the same
surface. Full notes at [shipreal.dev/developers](https://shipreal.dev/developers).

MIT licensed. Source at
[github.com/mluggy/shipreal-dev](https://github.com/mluggy/shipreal-dev).
