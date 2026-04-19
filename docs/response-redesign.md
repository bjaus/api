# Response Redesign

Status: design phase, pre-implementation
Related: `bjaus-api-enhancements.md` Gap 1 (superseded by this document)

## Motivation

The request side of the framework is **declarative**: the `Req` struct is
the contract, tags name wire-level sources, and the framework binds.

```go
type GetUserReq struct {
    ID     string `path:"id"`
    Fields string `query:"fields"`
    Auth   string `header:"Authorization"`
    Body   struct {
        Name string `json:"name"`
    }
}
```

The response side is **imperative**: the handler returns a `*Resp`, and
anything beyond the body (status, headers, cookies) lives in marker-interface
methods or `json:"-"` smuggled fields. The response type is impoverished
relative to the request type, and that asymmetry is the root of every
"how do I shape this response" workaround.

The redesign makes responses declarative too. The `Resp` struct's fields
declare every wire-level part of the outgoing HTTP message, using the same
tag vocabulary as requests.

## Tag vocabulary (unified)

| Tag              | Request semantics            | Response semantics             |
| ---------------- | ---------------------------- | ------------------------------ |
| `path:"id"`      | read from URL path           | — (request-only)               |
| `query:"x"`      | read from query string       | — (request-only)               |
| `form:"x"`       | read from multipart form     | — (request-only)               |
| `header:"X-Foo"` | read from request headers    | write to response headers      |
| `cookie:"name"`  | read value from Cookie header | emit as `Set-Cookie`          |
| `Body` field     | request body (codec-decoded) | response body (dispatched)     |
| `status:""`      | —                            | response status code           |

## Response struct shape

```go
type GetArticleResp struct {
    Status int     `status:""`        // 0 = route default
    ETag   string  `header:"ETag"`    // "" = don't emit
    Body   Article                     // value type; framework emits via codec
}
```

All fields optional. Zero-value on a tagged field means "don't emit." The
`Body` field is the only one that affects the framework's dispatch path.

If a response type doesn't need a body at all (e.g., a DELETE that always
returns 204), omit the `Body` field entirely. The status-driven suppression
rule (below) handles the conditional case where the same response type
sometimes emits a body and sometimes doesn't.

## Body polymorphism

The `Body` field's Go type tells the framework how to emit:

| `Body` field type | Dispatch                                              |
| ----------------- | ----------------------------------------------------- |
| `T` (struct)      | encode via negotiated codec (JSON/XML/...)            |
| `io.Reader`       | stream raw bytes (file download, large payload)       |
| `<-chan E`        | SSE (text/event-stream, one event per channel send)   |
| field omitted     | no body at all (DELETE → 204, etc.)                   |

The framework reflects on the `Body` field's type **once at registration**
and commits to one dispatch path per route. The handler never picks a
"response class" — the field type is the contract.

## Body suppression by status

Per RFC 9110 §6.4.1, responses with status `1xx`, `204`, or `304` MUST
NOT carry a body. The framework enforces this: when the response's
`Status` field is in this set, the body is dropped regardless of what
the handler populated.

This is the mechanism that handles the "same response type, sometimes
with a body, sometimes without" case (canonical example: conditional GET
returning 200 + body or 304 + headers only). The handler sets `Status: 304`
and leaves `Body` at its zero value; the framework skips body emission.
No pointer on `Body`, no nil checks.

The no-body status set is hard-coded (not configurable): `100–199`, `204`,
`304`. These are HTTP-level invariants, not policy.

## Cookie type

Framework-owned `api.Cookie` type, distinct from `*http.Cookie`:

```go
type Cookie struct {
    Value       string
    Path        string
    Domain      string
    Expires     time.Time
    MaxAge      int
    Secure      bool
    HttpOnly    bool
    SameSite    http.SameSite
    Partitioned bool
    Quoted      bool
}
```

- `Name` is carried by the struct tag, not the type.
- Zero-value `Cookie{}` = don't emit.
- Interop with stdlib via `Cookie.ToHTTPCookie(name string) *http.Cookie`
  and `CookieFromHTTP(*http.Cookie) Cookie`.
- All emission-relevant fields from stdlib `http.Cookie` are preserved.
  Read-only stdlib fields (`Raw`, `RawExpires`, `Unparsed`) are omitted.

## Embedding for boilerplate reduction

Anonymous struct embedding composes tagged fields the same way
`encoding/json` composes them: the outer struct sees the embedded
type's fields as its own.

```go
type CacheHeaders struct {
    ETag         string `header:"ETag"`
    CacheControl string `header:"Cache-Control"`
}

type Pagination struct {
    TotalCount int    `header:"X-Total-Count"`
    NextCursor string `header:"X-Next-Cursor"`
}

type ListArticlesResp struct {
    CacheHeaders
    Pagination
    Body []Article
}
```

The framework's tag scanner walks into anonymous struct fields. Consumers
factor out common response metadata (cache headers, pagination signals,
trace headers) into reusable types and embed them.

The same applies to requests:

```go
type AuthHeaders struct {
    Token string `header:"Authorization"`
    Trace string `header:"X-Request-ID"`
}

type GetUserReq struct {
    AuthHeaders
    ID string `path:"id"`
}
```

## Empty responses

If a response type has no `Body` field, the framework emits zero body
bytes and does not set `Content-Type`. This covers the "just status"
case without requiring a pointer or sentinel.

| Response struct                      | Default status | Body emitted             |
| ------------------------------------ | -------------- | ------------------------ |
| `struct { Body T }`                  | 200            | yes, encoded `T`         |
| `struct {}` (no `Body` field)        | 200            | no (zero bytes)          |
| `*api.Void`                          | 204            | no (zero bytes)          |
| any type, `Status` is 1xx/204/304    | —              | suppressed (HTTP rule)   |

`api.Void` stays as a convenience sentinel: on the request side, it
means "no decoding needed"; on the response side, it defaults the route
status to 204. A consumer who wants the same effect without `api.Void`
writes `type MyResp struct {}` with `api.WithStatus(204)`.

## `api.Event` shape (for SSE body dispatch)

```go
type Event struct {
    Name  string        // emitted as `event: <Name>`
    Data  any           // emitted as `data: <json>`
    ID    string        // emitted as `id: <ID>`
    Retry time.Duration // emitted as `retry: <ms>`
}
```

The framework emits only the fields the handler populates — zero-valued
fields are skipped. Matches the W3C Server-Sent Events spec: `event`,
`data`, `id`, `retry`.

## Retirement list

Subsumed by the redesign, removed from the framework:

- `CookieSetter` marker interface
- `HeaderSetter` marker interface
- `StatusCoder` interface for responses
  (keep as error interface — errors still carry status)
- `*Redirect` special type → replaced by `api.RedirectResp` + `api.Redirect(url, status)` helper
- `*Stream` special type → replaced by `io.Reader` body dispatch
- `*SSEStream` special type → replaced by `<-chan E` body dispatch

## In scope

1. Declarative response struct with `status:""` / `header:"…"` / `cookie:"…"` tags
2. `api.Cookie` type with stdlib interop helpers
3. `Body` field polymorphism via Go type dispatch
4. Retire the marker interfaces and special types listed above
5. OpenAPI generator: reflect tagged response fields into response headers,
   cookies, status, and body schema
6. Ergonomic helpers: `api.Redirect(url, status) *RedirectResp`, etc.

## Deferred (notable but out of scope)

- **Per-status response schemas** — `WithResponse(code, schema)` for
  documenting different body shapes per outcome (200 vs 404 vs 429).
  High value for OpenAPI, orthogonal to struct shape.
- **Background tasks** — fire-and-forget work that runs after the response
  is flushed, with a detached context.
- **File download with Range support** — would slot into the `io.Reader`
  body path naturally.
- **HEAD / OPTIONS auto-generation** from the route table.
- **WebSocket upgrade** — different protocol, different lifecycle; likely
  a separate handler kind.
- **Response-body validation** — validate the outgoing body against the
  declared schema before sending. Lower value in Go than in Python.
- **Trailers** (gRPC-web etc.) — rare, can be added if needed.

## Closed design decisions

- [x] **Body dispatch:** polymorphic on the field's Go type. One mental
      model across JSON / stream / SSE endpoints. Declarative metadata
      (headers, cookies, status) applies uniformly regardless of body kind.
- [x] **Cookie type:** `api.Cookie`, not `*http.Cookie`. Zero-value means
      "don't emit." Name lives on the struct tag. Bidirectional stdlib
      interop via `Cookie.ToHTTPCookie(name)` and `CookieFromHTTP(*http.Cookie)`.
      All emission-relevant fields from stdlib are preserved; read-only
      fields (`Raw`, `RawExpires`, `Unparsed`) are intentionally omitted.
- [x] **`Body` is a value on both sides** (`Body T`). Restores symmetry
      with request `Body`. No nil checks. The conditional-emission case
      (304 on If-None-Match, 200 with body otherwise) is handled by
      status-driven body suppression — set `Status: 304` and the framework
      drops the body per RFC 9110.
- [x] **Status-driven body suppression.** Framework silently drops the
      body when `Status` is `1xx`, `204`, or `304`. Hard-coded; matches
      the HTTP spec. Handlers that populate `Body` alongside one of these
      statuses see the body dropped — small buglet, wire output still
      correct.
- [x] **No `body:"…"` tag.** The field literally named `Body` is the body
      on both sides. Composition inside the body is handled by struct
      nesting + the codec, not by a framework-level body-composition
      layer. Multiple `body:"…"` tags would duplicate what the codec
      already does and wouldn't translate cleanly across codecs (XML,
      YAML, MessagePack).

## Performance strategy

The redesign relies on reflection to walk tagged fields on request and
response types. Reflection cost is real, but manageable with one
discipline: **reflect once at registration, iterate descriptors per
request.**

### Descriptor caching at registration

When a route is registered, the framework reflects on the `Req` and
`Resp` types once and builds compact descriptors stored on `routeInfo`:

```go
type responseDescriptor struct {
    statusFieldIndex int                 // -1 if not present
    headerFields     []headerFieldDesc   // precomputed field index + header name
    cookieFields     []cookieFieldDesc   // precomputed field index + cookie name
    bodyFieldIndex   int                 // -1 if no body field
    bodyKind         bodyKind            // codec | reader | chan | none
}

type headerFieldDesc struct {
    index []int  // reflect.Value.FieldByIndex path (handles embedding)
    name  string // header name from tag
    kind  reflect.Kind
}
```

At request time the emitter (and binder, on the request side) walks the
descriptor's slice entries, accessing fields by cached `[]int` index via
`reflect.Value.FieldByIndex`. No repeated tag parsing, no string
lookups, no `FieldByName` in hot paths.

### Rules for the hot path

- **Never** call `reflect.StructTag.Get(...)` per request — tags are
  parsed once and stored in descriptors.
- **Never** call `reflect.Value.FieldByName(...)` per request — use
  the cached `[]int` index from `reflect.VisibleFields`.
- **Do** call `reflect.Value.FieldByIndex(cached)` — this is fast
  (indexed offset walk, handles embedding correctly).
- **Avoid** allocation in binder/emitter loops where possible; pre-size
  slices based on descriptor counts.

### Benchmarking

Add `BenchmarkBind_*` and `BenchmarkEmit_*` benchmarks for the hot paths
from day one. Track:

- Small struct (3-5 fields) bind/emit
- Medium struct (10-15 fields, embedded) bind/emit
- Body-only (no tagged fields)
- Stream body (`io.Reader`)
- SSE body (`<-chan Event`)

CI runs benchmarks and flags regressions >10% on any path. This prevents
reflection shortcuts from being silently undone by future edits.

### Escape hatches (future, not v1)

If real users hit real performance walls:

1. `sync.Pool` for hot-path allocations.
2. Opt-in `go generate` codegen tool that emits typed binders/emitters
   for chosen types — eliminates reflection entirely on generated paths.
3. `RawHandler` is always available for per-endpoint hand-tuning.

None of these are required for the initial implementation. The
descriptor-caching discipline alone puts us in the Echo/Gin performance
range — well ahead of FastAPI-style frameworks and plenty for anyone
whose real bottleneck is the network or the database.

## Non-goals

- Backward compatibility with the current special types. Pre-1.0, we
  rewrite.
- Supporting arbitrary custom response kinds via user-defined dispatch.
  Framework owns the Body dispatch table; consumers who need custom
  emission use `RawHandler`.
