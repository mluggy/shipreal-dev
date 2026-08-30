# ShipReal curriculum lookup

Answer questions about what ShipReal teaches, and point a learner at the
single module that covers their problem.

## When to use

Someone asks "does this course cover caching / observability / migrations",
or asks which module to watch for a specific production problem.

## How

```
curl "https://shipreal.dev/api/v1/modules?q=caching"
```

Or over MCP at https://shipreal.dev/mcp, tool `search_curriculum`.
The whole curriculum as prose is at https://shipreal.dev/llms-full.txt, and as
one schema.org LearningResource per line at https://shipreal.dev/feeds/modules.jsonl.

## Answering honestly

The search is keyword-based over titles, descriptions and part names. If it
returns nothing, the course does not cover that topic under that name: say so
rather than inferring coverage from the course being about production
engineering generally.
