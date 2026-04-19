# bjaus/api — Framework Gap RFC

Intended for an agent tasked with enhancing `github.com/bjaus/api`. The
three gaps below were discovered while building a typed, spec-first API
on top of the framework. In each case, the consumer had to build a
work-around at the adapter layer because the framework offered no
idiomatic hook. Backward compatibility is not a concern — pre-1.0 —
but every change must preserve the framework's core invariants:

- **Spec-first.** Route registration produces an OpenAPI 3.1 document.
  Anything a handler can do must be expressible in the spec, or clearly
  marked as an opt-out.
- **Interface-driven.** Extension points are small Go interfaces
  (`SelfValidator`, `Resolver`, `Validator`, `Registrar`,
  `ErrorHandler`) that consumers implement; the framework composes
  them. Favor new interfaces over new struct fields or tag magic.
- **Typed-handler purity.** The core shape is
  `func(ctx, *Req) (*Resp, error)`. Consumers should almost never need
  `RawHandler` or `RawRequest` for routine concerns.

Your task: implement the three enhancements below, with tests and
OpenAPI-spec coverage where relevant. Produce one PR per gap or group
related gaps — whatever keeps the diff reviewable. Do not worry about
keeping the current API surface intact; prefer the right shape.

---

## Gap 1 — Response-side side effects (cookies, custom headers)

### Problem

`Handler[Req, Resp]` deliberately hides `http.ResponseWriter` so the
framework owns serialization. But several common HTTP flows need to
shape the response beyond the body:

- Session login/logout: `Set-Cookie` with the session token.
- CSRF issuance: rotating `Set-Cookie` and header.
- Feature flags / server-hints: custom response headers.
- `Location` on redirect-style responses.
- Cache hints (`ETag`, `Cache-Control`) that depend on the computed
  response, not just the route.

Today, a consumer that needs these must either:

1. Fall back to `RawHandler`, losing OpenAPI inference and the typed
   handler shape entirely, or
2. Build a custom per-handler middleware that wraps
   `http.ResponseWriter` into a sidecar struct, binds it on `ctx`
   (e.g. via `bjaus/bind`), then reads it back inside the handler and
   writes cookies/headers through the sidecar.

Both are workarounds. The second is what real consumers end up
shipping, and it violates the spirit of the framework: handlers end
up carrying knowledge of a mount-time wrapper, and the response-side
side effect is invisible to the spec.

### Proposed design

Introduce a first-class **response decorator** surface. Two
complementary mechanisms:

**1. A `ResponseEnvelope` returnable from handlers.** Generic over the
body type. Handlers that want to shape the response return an envelope
rather than a bare `*Resp`:

```go
type ResponseEnvelope[Resp any] struct {
    Body    *Resp
    Status  int           // optional override of the route default
    Headers http.Header   // headers to merge before the body is encoded
    Cookies []*http.Cookie
}

type EnvelopeHandler[Req, Resp any] func(ctx context.Context, req *Req) (*ResponseEnvelope[Resp], error)
```

Registration helpers mirror the typed handler ones — `api.GetEnvelope`,
`api.PostEnvelope`, etc., or a single `WithEnvelope` route option if
that reads better. The OpenAPI generator treats `Resp` the same way it
does for the plain handler; envelope metadata does not affect the spec
unless the consumer also emits `api.WithHeader("Set-Cookie", ...)` at
registration time for documentation.

**2. A narrow, typed `ResponseSink` interface** for the rare handler
that must stream or write out of band. Accessible through the request
struct via an embeddable marker (parallel to `RawRequest`):

```go
type ResponseControl struct {
    Writer http.ResponseWriter // populated by the framework before Validate/Handler run
}
```

Handlers that embed `api.ResponseControl` opt into raw writer access
without giving up typed-request decoding or validation. The OpenAPI
generator skips this field the same way it skips `RawRequest`.

### Acceptance criteria

- Typed login-style flow writes `Set-Cookie` without a user-authored
  middleware wrapper and without touching `http.ResponseWriter` in the
  common case (envelope path).
- Envelope `Status` override correctly replaces the route default.
- Envelope headers and cookies survive the codec/encoding path (no
  double-write, no lost `Content-Type`).
- `ResponseControl` is opt-in and is omitted from generated request
  schemas.
- Tests cover: cookie write, header merge, status override, error
  return from an envelope handler (error flows through `ErrorHandler`
  unchanged), `ResponseControl` embed.

---

## Gap 2 — Validation pipeline is fixed and error format is baked in

### Problem

`buildHandler` runs validation in this order, unconditionally:

1. `Resolver.Resolve`
2. `validateConstraints` (struct tags: `minLength`, `maxLength`,
   `pattern`, `minimum`, `maximum`, `enum`, `minItems`, `maxItems`)
3. `SelfValidator.Validate`
4. global `Validator.Validate`

Two problems stack:

**Problem A — ordering.** A consumer with a real validator (e.g.
`ozzo-validation`) wants their validator to be the single source of
truth. Today they cannot put `SelfValidator` before
`validateConstraints` without wrapping the handler themselves, so
they are forced to omit all constraint tags to keep errors consistent.
That loses the OpenAPI payoff of those tags — `minLength` on a body
field is a spec-worthy constraint even if runtime enforcement is owned
by the consumer's validator.

**Problem B — error shape.** `validateConstraints` emits a hard-coded
`ProblemDetail` with title `"Validation Failed"` and status 400. A
consumer with a domain-specific error envelope (e.g. a custom
`ErrCode` taxonomy carried by their own error type) cannot recolor it.
`ErrorHandler` catches the error on the way out but only sees
`*ProblemDetail`; the consumer has to re-interpret it to emit a
consistent shape across constraint errors and handler errors.

### Proposed design

**Separate spec-time tags from runtime-time enforcement.** Struct tags
like `minLength` remain authoritative for the OpenAPI schema. Runtime
enforcement becomes an opt-in mode:

```go
type ValidationMode int

const (
    ValidateConstraintsFirst ValidationMode = iota // current behavior
    ValidateConstraintsLast                        // tags run after SelfValidator/Validator
    ValidateConstraintsOff                         // spec-only; no runtime enforcement
)

func WithValidationMode(m ValidationMode) RouterOption
```

Per-route override via a route option for consumers that want per-route
behavior. Default stays `ValidateConstraintsFirst`.

**Make the constraint error shape pluggable.** The current call site
hard-codes construction of `*ProblemDetail`. Extract that into an
interface the consumer can supply:

```go
type ConstraintErrorBuilder interface {
    Build(violations []ValidationError) error
}

func WithConstraintErrorBuilder(b ConstraintErrorBuilder) RouterOption
```

If unset, the framework uses the current `*ProblemDetail` builder.
Consumers that map everything to a domain error code (`errx`,
custom taxonomy) supply a builder that produces their error type, so
`ErrorHandler` sees a uniform shape.

### Acceptance criteria

- Consumer can set `ValidateConstraintsOff` and no struct-tag
  validation runs at runtime, but the OpenAPI spec still reports the
  constraints (confirm with a spec snapshot test).
- Consumer can set `ValidateConstraintsLast` and their `SelfValidator`
  runs before struct-tag validation.
- Consumer can supply a `ConstraintErrorBuilder` and see their error
  type in the `ErrorHandler` when a tag is violated.
- Per-route override (if implemented) wins over the router default.

---

## Gap 3 — Groups cannot nest

### Problem

`Router.Group(prefix, opts...)` returns a `*Group`, but `*Group` has no
`Group` method of its own. Every sub-group requires going back to the
router with the fully-qualified prefix and repeating the parent's
middleware stack:

```go
pub := r.Group("/api/identity", WithGroupMiddleware(mwA))
authed := r.Group("/api/identity", WithGroupMiddleware(mwA, mwB)) // repeated
```

This makes nontrivial middleware stacks brittle: adding a new
middleware to the parent means touching every child. It also makes
tag and security inheritance manual.

### Proposed design

Add `(*Group).Group(prefix string, opts ...GroupOption) *Group` with
clear inheritance semantics:

- Child prefix is concatenated onto the parent's: `parent.prefix +
  child.prefix`.
- Child middleware is **appended** to the parent's middleware (parent
  runs first). New `GroupOption` to explicitly **reset** middleware if
  a child wants an isolated stack.
- Child tags are the union of parent and child tags (current behavior
  for `Group` against `Router`).
- Child security requirements: child's `WithGroupSecurity` replaces
  parent's; absence inherits.

Update `Registrar` implementation so child groups register through
their parent (so middleware composition works top-down) and land in
the router's route table with the full prefix.

### Acceptance criteria

- `r.Group("/api").Group("/identity").Group("/admin")` yields routes
  under `/api/identity/admin/...`.
- Middleware attached at each level runs in outer-to-inner order once
  per request.
- Tags from all ancestors appear on the route in the OpenAPI spec.
- Security from the nearest ancestor is inherited when the child has
  none; child's security replaces when present.
- Reset option (or equivalent) lets a child build an isolated
  middleware stack when required.

---

## Delivery notes

- Ship with tests at the framework's existing coverage bar.
- Update any examples in `cmd/` that exercise these surfaces.
- Run `golangci-lint ./...` clean.
- Where a new interface replaces a struct-tag behavior, the old behavior
  stays in place unless explicitly disabled (no silent regressions).
- Each gap is independent — land them separately if easier.
