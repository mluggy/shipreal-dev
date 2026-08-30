"""Smoke tests against the live sandbox.

They hit the network on purpose. The sandbox exists precisely so a test can
depend on fixed responses without depending on the course staying still, and a
client that is never exercised against the real transport is a client that
passes its tests and fails in use.
"""

import unittest

from shipreal import ShipReal, ShipRealError


class SandboxTests(unittest.TestCase):
    def setUp(self) -> None:
        self.sr = ShipReal(sandbox=True)

    def test_search_returns_fixture_modules(self) -> None:
        page = self.sr.search()
        self.assertIn("data", page)
        self.assertTrue(page["data"])
        self.assertTrue(page["data"][0]["slug"].startswith("sandbox-module-"))

    def test_module_by_slug(self) -> None:
        module = self.sr.module("sandbox-module-1")
        self.assertEqual(module["slug"], "sandbox-module-1")
        self.assertIn("description", module)

    def test_modules_iterates_every_page(self) -> None:
        every = list(self.sr.modules())
        self.assertTrue(every)
        self.assertEqual(len({m["slug"] for m in every}), len(every))

    def test_pricing_flattens_to_one_region(self) -> None:
        flat = self.sr.pricing(region="intl")
        self.assertEqual(flat["region"], "intl")
        self.assertIn("now", flat["complete"])
        # Unflattened keeps both regions side by side.
        self.assertIn("intl", self.sr.pricing()["complete"])

    def test_missing_module_raises_problem_details(self) -> None:
        with self.assertRaises(ShipRealError) as caught:
            self.sr.module("does-not-exist")
        self.assertEqual(caught.exception.status, 404)
        self.assertIsNotNone(caught.exception.problem)

    def test_batch_rejects_more_than_twenty(self) -> None:
        with self.assertRaises(ValueError):
            self.sr.batch([{"path": "/modules"}] * 21)


if __name__ == "__main__":
    unittest.main()
