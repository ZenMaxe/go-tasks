# Exercise 7 — Resilient Client (Rate Limiter + Circuit Breaker + Retry, Composed)

Standalone Go module. Run it on its own, independent of any other exercise in this
repo:

```bash
cd exercise-7
go run .
```

## Spec

Build a `ResilientClient` that wraps calls to an unreliable operation, composing
three previously-built, independent pieces: a token-bucket rate limiter, a circuit
breaker, and bounded retry with backoff.

- `func (c *ResilientClient) Call(ctx context.Context, fn func(ctx context.Context) error) error`
- Internally, `Call` should:
  1. Respect the rate limiter — don't attempt the call faster than the configured
     rate.
  2. Check the circuit breaker — if open, fail fast without attempting the call or
     consuming a rate-limit token.
  3. If allowed through, attempt the call with bounded retry + backoff — but each
     individual retry attempt should still go through the circuit breaker's
     `Allow()`/`RecordSuccess()`/`RecordFailure()` (a failing service should be
     able to trip the breaker mid-retry-sequence, not just after the whole call
     finishes).
  4. Respect `ctx` throughout — cancellation at any point should stop everything
     promptly.
- Reuse the existing `TokenBucket`, `CircuitBreaker`, and `Retry` types as-is —
  this exercise is about composition, not rewriting any of the three.
- Write a test harness: a fake operation that fails deterministically for the
  first few calls (enough to trip the breaker), then "recovers" — confirm the
  whole stack behaves sensibly end-to-end.

### Key design question

Does it make sense to consume a rate-limit token for a call the circuit breaker is
about to reject anyway? Which check should run first? If the breaker trips
mid-retry, should the retry loop stop immediately or keep retrying against a
breaker that's now rejecting everything? Do retry delays and rate-limiter waits
solve overlapping problems, or distinct ones?

**Answer used here:**
- **Breaker before limiter, on every attempt — not just once per `Call`.** A first
  pass checked the breaker exactly once per `Call()`, before handing off to
  `Retry`, which then made up to N attempts internally with zero breaker
  visibility. That meant the breaker could never observe or react to failures
  happening mid-retry-sequence, and it also meant every one of those N attempts
  would burn a rate-limit token even after the breaker had effectively given up.
  Fix: wrap `fn` in a closure and pass the wrapper to `Retry` instead of the raw
  function. Inside the wrapper, check `circuitBreaker.Allow()` **first**; only if
  it returns `true` does the wrapper consume a rate-limiter token via
  `rateLimiter.Allow(ctx)` and then call the real `fn`. A rejected breaker check
  is a cheap in-memory branch — there's no reason to pay for a token on a call
  that was never going to be attempted. `Retry` itself needs no changes; it just
  calls whatever function it's given, on every attempt, unaware that the function
  now happens to check a breaker and a limiter internally. Standard Go
  decorator-via-closure composition, not a performance concern — the overhead of
  one extra function-call layer is negligible next to the real operation being
  retried.
- **The retry loop doesn't stop early when the breaker trips mid-sequence — but
  it stops doing real work.** `Retry` has no breaker awareness by design (the
  constraint is reuse it as-is), so once the breaker opens partway through a
  retry sequence, `Retry` will still run out its remaining attempts. Each of
  those remaining attempts, though, is now a fast, no-op rejection from the
  wrapped closure — no network/DB call, no rate-limit token spent — followed by
  `Retry`'s normal backoff wait before the next attempt. The wall-clock cost is
  a few cheap backoff sleeps, not repeated real failures against a struggling
  downstream. That trade-off (accept a slightly slower final-error return in
  exchange for not touching `Retry`) is the right one for reusing three
  independent, already-tested components instead of rewriting `Retry` to know
  about breakers.
- **Rate-limiter waits and retry backoff are solving different problems and both
  apply.** The limiter caps *how often calls can be attempted at all* — a
  steady-state ceiling independent of success/failure. Backoff only exists
  *after a failure*, to give a struggling downstream room to recover, and grows
  with consecutive failures. They compound when both are active (a retry can
  wait for a token and then for backoff), but that's expected: a client that
  just tripped a breaker and is backing off shouldn't also be exempt from the
  steady-state rate cap once the breaker lets it back in.

## Requirement → code map

| # | Requirement | Where |
|---|---|---|
| 1 | Rate limiter respected | `rateLimiter.Allow(ctx)` inside `wrappedFn` in `resilientClient.Call` (`resilient_client.go`) |
| 2 | Breaker checked before attempting; rejected calls don't spend a token | `circuitBreaker.Allow()` is the first statement in `wrappedFn`, returns before `rateLimiter.Allow` is ever reached |
| 3 | Bounded retry, breaker visible on every attempt | `wrappedFn` (breaker check + real call + `RecordSuccess`/`RecordFailure`) is passed to `retrier.Retry(ctx, c.maxAttempts, wrappedFn)`, so `Retry` re-invokes the breaker-aware closure on every one of its internal attempts |
| 4 | `ctx` respected throughout | `rateLimiter.Allow(ctx)` and `retrier.Retry(ctx, ...)` both select on `ctx.Done()` internally (existing `TokenBucket`/`Retry` behavior, untouched) |
| Reuse existing pieces as-is | `TokenBucket`, `CircuitBreaker`, `Retry` | `rate_limiter.go`, `circuit_breaker.go`, `retry.go` — copied over unmodified from prior exercises, no logic changes |
| Test harness | Fake op failing deterministically then recovering | `main.go` — `unreliableOp` fails its first 5 real calls, then always succeeds; three-phase demo drives the breaker Closed → Open → (cooldown) → Closed |

## Notable implementation details

- **Bug found and fixed while composing:** the first working draft built the
  breaker-aware `wrappedFn` correctly but never called `rateLimiter.Allow(ctx)`
  anywhere in `Call` — the rate limiter was constructed and started but silently
  unused, so requirement 1 was not actually met despite looking done. Caught by
  writing the phase-2 assertion in `main.go` (`real attempts made == 0` while the
  breaker is open) and by re-reading the requirement list against the code
  line-by-line rather than trusting that "it compiles and one call works" means
  the composition is complete.
- **`maxAttempts` is a constructor parameter on `ResilientClient`** (default 5 if
  `<= 0`), not hardcoded — mirrors `Retry`'s own configurable base delay so both
  layers are tunable without touching code.
- **`Start(ctx)` must be called once before `Call`** — it starts the rate
  limiter's refill goroutine. `Call` returns an error immediately if the client
  hasn't been started, and `Start` returns an error if called twice, both guarded
  by the same `sync.Mutex`.
- **The demo's phase 3 sleeps out the breaker's real 5-second cooldown** rather
  than faking the clock — `CircuitBreaker` (from exercise-4) isn't given a
  configurable cooldown, and the constraint is to reuse it as-is, so the test
  harness just pays the wall-clock cost to observe the real Open → HalfOpen →
  Closed transition.
