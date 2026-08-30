"""ShipReal SDK: a thin, dependency-free client for the public ShipReal API.

There is no authentication anywhere in this package, and that is not an
omission. The API is public read-only reference data about one course, so there
is no key to hold, no token to refresh, and no credential this client could
leak. If something asks you for a ShipReal API key, it is not us.

Python 3.9+, standard library only.
"""

from ._client import ShipReal, ShipRealError

__all__ = ["ShipReal", "ShipRealError", "__version__"]
__version__ = "1.0.0"
