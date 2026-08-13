# Normalized HTTP Source Adapter

Goflow beta exposes vendor-neutral source integration through the reviewed
`normalizedHttpSource` built-in node. Packs cannot execute adapter binaries or
native plugins. A workflow should keep destination actions, such as Telegram,
in separate downstream nodes and consume only the normalized source output.

## Pack capability

New Packs using this node must declare
`goflow.adapter.normalized-http.v1` in `required_capabilities`. An explicit
declaration without that capability fails validation. A legacy Pack Format v1
manifest that omits `required_capabilities` retains its existing compatibility.

## Request contract

The node performs only `GET` requests to an absolute `http` or `https` URL.
`auth_mode` is `none`, `bearer`, or `api_key`. Authenticated modes require a
`credential_id` resolved from the encrypted credential store; workflow literals
are not credentials. `api_key_header` defaults to `X-API-Key` and cannot be a
reserved authorization, cookie, or proxy-authorization header.

Every mode requires `response_contract`, using the same strict JSON contract
shape as source connection tests. Only fields declared in `required` are
emitted; undeclared vendor fields are discarded.

`pagination: cursor` requires `cursor_query_param`, `items_field`, and
`next_cursor_field`. Each response is an object whose items field is an array
of objects matching `response_contract` and whose next cursor is a string or
`null`. Output is:

```json
{
  "items": [],
  "page_count": 1
}
```

Pack authors must choose `max_pages` from 1 through 20 and `max_items` from 1
through 5000. Responses are limited to 1 MiB and cursors to 512 characters.
Repeated cursors, malformed JSON, invalid shapes, and limit overflow fail
closed before downstream nodes execute.

## Network and error behavior

Requests have a maximum 30-second timeout. Redirects use the shared safe
redirect policy and do not forward credentials across origins. A `429` is
retried at most twice only when `Retry-After` is an integer delta from 0 through
5 seconds. GET makes this retry idempotent; the engine does not add another
node-level retry.

Failures expose only closed public categories such as invalid contract,
unreachable source, timeout, rate limit, response limit, or HTTP error. They do
not expose the credential, authorization header, source URL, raw response body,
or vendor-specific error text. Contract fixtures should use loopback servers
and fake credentials, and cover pagination, bounds, rate limits, redirects,
cancellation, malformed responses, and downstream side-effect blocking.

DailyOps remains vendor-neutral and may use the existing generic HTTP node when
its source already supplies the documented normalized DailyOps contract. A
vendor-specific adapter requires a separate approved vendor decision and test
sandbox.
