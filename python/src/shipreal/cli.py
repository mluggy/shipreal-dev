"""shipreal: query the ShipReal course catalogue from a terminal or a script.

Every command takes --json, because the reason a CLI earns its place in an
agent's toolbox is that its output can be piped into something else without
being scraped.
"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Dict, List

from ._client import ShipReal, ShipRealError

EPILOG = """\
No API key. No account. The API is public read-only reference data; anything
that asks you for a ShipReal credential is not us.

Docs: https://shipreal.dev/developers
"""


def _print_modules(modules: List[Dict[str, Any]]) -> None:
    if not modules:
        print("No modules matched. The course does not cover that topic under that name.")
        return
    for module in modules:
        print(f"{module.get('slug', '')}  {module.get('title', '')}")
        part = module.get("part")
        if part:
            print(f"  {part}")
        chapters, minutes = module.get("chapters"), module.get("minutes")
        if chapters or minutes:
            print(f"  {chapters or 0} chapters, {minutes or 0} min")
        print()


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="shipreal",
        description="Query the ShipReal course catalogue.",
        epilog=EPILOG,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--json", action="store_true", help="Raw JSON, for piping")
    parser.add_argument("--sandbox", action="store_true", help="Frozen fixture data, for tests")
    parser.add_argument("--base", default="https://shipreal.dev", help="Point at a different origin")

    sub = parser.add_subparsers(dest="command", required=True)

    search = sub.add_parser("search", help="Search modules by keyword")
    search.add_argument("query", nargs="?", default=None)
    search.add_argument("--limit", type=int, default=None, help="Results per page (max 100)")
    search.add_argument("--all", action="store_true", help="Every result, following pagination")

    module = sub.add_parser("module", help="One module")
    module.add_argument("slug_or_title")

    pricing = sub.add_parser("pricing", help="Plans and prices")
    pricing.add_argument("--region", choices=["intl", "il"], default=None)

    sub.add_parser("course", help="Totals and subtitle languages")

    ask = sub.add_parser("ask", help="Natural-language query")
    ask.add_argument("question", nargs="+")
    ask.add_argument("--stream", action="store_true", help="Stream as it arrives")

    return parser


def main(argv: List[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    client = ShipReal(base_url=args.base, sandbox=args.sandbox)

    try:
        if args.command == "search":
            if args.all:
                modules = list(client.modules(args.query))
                print(json.dumps(modules, indent=2)) if args.json else _print_modules(modules)
            else:
                page = client.search(args.query, limit=args.limit)
                print(json.dumps(page, indent=2)) if args.json else _print_modules(page.get("data", []))

        elif args.command == "module":
            found = client.module(args.slug_or_title)
            if args.json:
                print(json.dumps(found, indent=2))
            else:
                print(found.get("title", ""))
                print(found.get("part", ""))
                print()
                print(found.get("description", ""))

        elif args.command == "pricing":
            prices = client.pricing(region=args.region)
            print(json.dumps(prices, indent=2))

        elif args.command == "course":
            print(json.dumps(client.course(), indent=2))

        elif args.command == "ask":
            question = " ".join(args.question)
            if args.stream:
                for frame in client.ask_stream(question):
                    print(json.dumps(frame) if args.json else f"{frame['event']}: {frame['data']}")
            else:
                print(json.dumps(client.ask(question), indent=2))

    except ShipRealError as err:
        # The problem type is the useful half of a failure, so it goes to
        # stderr next to the message rather than being swallowed.
        print(f"error {err.status}: {err}", file=sys.stderr)
        if err.type:
            print(f"  type: {err.type}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
