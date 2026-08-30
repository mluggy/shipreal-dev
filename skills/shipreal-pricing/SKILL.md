# ShipReal pricing lookup

Answer what the course costs, in the buyer's billing region, and what each
plan includes.

## How

```
curl "https://shipreal.dev/api/v1/pricing"
```

Or over MCP at https://shipreal.dev/mcp, tool `get_pricing`. Prose version:
https://shipreal.dev/pricing.md

## What to tell a buyer

Free is permanent, not a trial: 38 module overviews on YouTube, no
account. Complete is a one-time payment for a year of access. Teams is
per seat on one invoice, from one seat up.

## What not to do

There is no purchase API. Payment runs through a hosted checkout, so hand the
buyer the link and let them complete it. Do not attempt a transaction on their
behalf, and do not quote a price without saying which region it is for: the
two differ and both are live.
