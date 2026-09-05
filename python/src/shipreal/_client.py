"""The client itself. Standard library only: urllib, json, typing."""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, Iterator, List, Optional, Sequence

DEFAULT_BASE = "https://shipreal.dev"
VERSION = "v1"
_USER_AGENT = "shipreal-python/1.0.0 (+https://shipreal.dev/developers)"


class ShipRealError(Exception):
    """Raised for any non-2xx response, carrying the RFC 9457 problem details.

    Read ``type`` rather than switching on ``status``: the status says a request
    failed, the type says which failure it was, and only the second one is
    stable enough to branch on.
    """

    def __init__(self, status: int, problem: Optional[Dict[str, Any]], url: str) -> None:
        message = None
        if problem:
            message = problem.get("detail") or problem.get("title")
        super().__init__(message or f"Request failed with {status}")
        self.status = status
        #: RFC 9457 problem details, when the server sent them.
        self.problem = problem
        #: Stable identifier for the kind of failure.
        self.type = problem.get("type") if problem else None
        self.url = url


class ShipReal:
    """Client for the public ShipReal API.

    :param base_url: Override the origin, e.g. to point at a local worker.
    :param sandbox: Route reads at the frozen fixture data. See
        https://shipreal.dev/sandbox for what differs and what does not.
    :param timeout: Seconds before a request gives up.
    """

    def __init__(
        self,
        base_url: str = DEFAULT_BASE,
        sandbox: bool = False,
        timeout: float = 30.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.sandbox = sandbox
        self.timeout = timeout

    # -- plumbing ---------------------------------------------------------

    def _url(self, path: str, params: Optional[Dict[str, Any]] = None) -> str:
        prefix = f"/api/{VERSION}/sandbox" if self.sandbox else f"/api/{VERSION}"
        url = self.base_url + prefix + path
        query = {
            k: str(v)
            for k, v in (params or {}).items()
            if v is not None and v != ""
        }
        if query:
            url += "?" + urllib.parse.urlencode(query)
        return url

    def _request(
        self,
        url: str,
        body: Optional[Dict[str, Any]] = None,
        accept: str = "application/json",
    ) -> Any:
        data = json.dumps(body).encode("utf-8") if body is not None else None
        headers = {"accept": accept, "user-agent": _USER_AGENT}
        if data is not None:
            headers["content-type"] = "application/json"
        req = urllib.request.Request(url, data=data, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                return json.loads(res.read().decode("utf-8"))
        except urllib.error.HTTPError as err:
            # The error body is where the problem details live, so it gets
            # parsed rather than discarded. A body that is not JSON is not
            # itself an error worth raising over: the status still stands.
            problem = None
            try:
                problem = json.loads(err.read().decode("utf-8"))
            except Exception:
                problem = None
            raise ShipRealError(err.code, problem, url) from None

    def _get(self, path: str, params: Optional[Dict[str, Any]] = None) -> Any:
        return self._request(self._url(path, params))

    # -- reads ------------------------------------------------------------

    def search(
        self,
        query: Optional[str] = None,
        page: Optional[int] = None,
        limit: Optional[int] = None,
        cursor: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Search the curriculum. Without a query, returns every module in
        course order.

        Matching is a case-insensitive substring over title, description and
        part name, so an empty result means the course does not cover that
        topic under that name, rather than that the search was too clever.
        """
        return self._get(
            "/modules", {"q": query, "page": page, "limit": limit, "cursor": cursor}
        )

    def modules(self, query: Optional[str] = None) -> Iterator[Dict[str, Any]]:
        """Every matching module, following pagination for you."""
        cursor = None
        while True:
            page = self.search(query, limit=100, cursor=cursor)
            for module in page.get("data", []):
                yield module
            cursor = (page.get("pagination") or {}).get("nextCursor")
            if not cursor:
                return

    def module(self, slug_or_title: str) -> Dict[str, Any]:
        """One module by slug, or by an exact or partial title match."""
        if not slug_or_title:
            raise TypeError("module(slug_or_title) needs an argument")
        return self._get("/modules/" + urllib.parse.quote(slug_or_title, safe=""))

    def pricing(self, region: Optional[str] = None) -> Dict[str, Any]:
        """Current plans and prices.

        Two regional prices are live at once, so quoting one without naming its
        region is misleading. Pass ``region`` ("intl" or "il") when you know
        which one applies and the response is flattened to that region.
        """
        every = self._get("/pricing")
        if region not in ("intl", "il"):
            return every
        complete = dict(every["complete"][region])
        complete["url"] = every["complete"].get("url")
        teams = dict(every["teams"][region])
        teams["packSeats"] = every["teams"].get("packSeats")
        return {
            "region": region,
            "free": every["free"],
            "complete": complete,
            "teams": teams,
        }

    def course(self) -> Dict[str, Any]:
        """Totals, language and the subtitle languages."""
        return self._get("/course")

    def batch(self, requests: Sequence[Dict[str, str]]) -> Dict[str, Any]:
        """Several reads in one round trip, up to 20.

        Each item comes back with its own status, so check per item rather than
        assuming the whole batch succeeded.
        """
        items: List[Dict[str, str]] = list(requests)
        if len(items) > 20:
            raise ValueError("batch takes at most 20 requests")
        return self._request(
            f"{self.base_url}/api/{VERSION}/batch", body={"requests": items}
        )

    def ask(self, query: str) -> Dict[str, Any]:
        """Ask in natural language (NLWeb).

        There is no model behind this: it runs the same keyword search, which
        means it says so when nothing matches instead of inventing a module.
        """
        if not query:
            raise TypeError("ask(query) needs a question")
        return self._request(f"{self.base_url}/ask", body={"query": query})

    def ask_stream(self, query: str) -> Iterator[Dict[str, Any]]:
        """The same question, streamed. Yields NLWeb events as they arrive:
        ``start``, then one ``result`` per hit, then ``complete``.
        """
        if not query:
            raise TypeError("ask_stream(query) needs a question")
        url = f"{self.base_url}/ask"
        req = urllib.request.Request(
            url,
            data=json.dumps({"query": query}).encode("utf-8"),
            headers={
                "accept": "text/event-stream",
                "content-type": "application/json",
                "user-agent": _USER_AGENT,
            },
        )
        try:
            res = urllib.request.urlopen(req, timeout=self.timeout)
        except urllib.error.HTTPError as err:
            raise ShipRealError(err.code, None, url) from None
        with res:
            event = "message"
            data = ""
            for raw in res:
                line = raw.decode("utf-8").rstrip("\n").rstrip("\r")
                if line == "":
                    # A blank line closes an SSE frame. Anything short of one is
                    # a partial frame and has to wait for the next line.
                    if data:
                        try:
                            yield {"event": event, "data": json.loads(data)}
                        except ValueError:
                            pass
                    event, data = "message", ""
                elif line.startswith("event:"):
                    event = line[6:].strip()
                elif line.startswith("data:"):
                    data += line[5:].strip()
