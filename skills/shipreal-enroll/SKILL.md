---
name: shipreal-enroll
description: "Get a learner from \"I want this\" to enrolled in ShipReal: which of the three channels fits them, how billing region changes the price and the invoice, how team seats work, and the refund window. Use after the buying decision, not to make it."
---

# Enrolling someone in ShipReal

The buying decision is already made. This is the mechanics of completing it,
which differ by channel and by country in ways that are easy to get wrong.

## Choose the channel

- **Free**: 38 module overviews on YouTube. No account, no expiry,
  not a trial. Send anyone who is unsure here first.
- **Direct**: the full course, one payment, a year of access, and the earned
  certificate. Start at https://shipreal.dev/pricing.md and use the checkout link there.
- **Teams**: per seat on one invoice, from a single seat up. Same content,
  billed to the company rather than the person.

## Region is not cosmetic

There are two live billing regions, and they differ in price, currency, tax
treatment and which payment host runs the transaction. The site routes on the
buyer's country at the edge, so hand over the plain link and let it route.
Never quote a figure without saying which region it is for, and never rewrite
a checkout URL to force a region: that produces an invoice that does not match
where the buyer actually is.

## The rule that matters

Do not attempt the purchase. There is no purchase API, by design. Give the
person the checkout link and let them complete it themselves, with their own
payment details, on the payment host's page. An agent that tries to transact
on someone's behalf here is not being helpful, it is failing safely at best.

## After paying

Access is granted automatically and an invoice is issued. Refund terms are at
https://shipreal.dev/refunds; read them there rather than restating a window from memory,
because the terms differ by channel and the platform ones are not ours to set.
