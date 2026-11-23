# Using Live Components

[Back to the README](../README.md).

The rendering library requires Go 1.24 or newer.

```go
page, err := live.Load("index.html")
if err != nil {
    return err
}

document := page.Document()
title := document.GetElementById("title")
title.SetTextContent("Hello World")

banner := document.QuerySelector(".banner")
if err := banner.ClassList().Add("active"); err != nil {
    return err
}

button := document.CreateElement("button")
button.SetTextContent("Click me")
if err := banner.AppendChild(button); err != nil {
    return err
}

output := page.Render()
```

Import the package as `live "github.com/renniemaharaj/live-components"`. Use `live.Parse(markup)` to parse
an HTML string instead of loading a file. Both functions parse a complete HTML
document and insert `html`, `head`, and `body` elements as needed.

## Finding elements

`Component`, `Document`, and `Element` share these operations:

| Method | Result |
| --- | --- |
| `GetElementById(id)` | First descendant with that exact, nonempty ID, or `nil` |
| `GetElementsByClassName("card active")` | Descendants containing all requested class tokens |
| `GetElementsByTagName("button")` | Descendants with that literal tag; `"*"` matches all elements |
| `QuerySelector(".card > button")` | First matching descendant, or `nil` |
| `QuerySelectorAll(".card")` | All matching descendants in document order |

Queries exclude the receiver itself. Document queries include the `html` element.
`page.QuerySelector(...)` is a shortcut for `page.Document().QuerySelector(...)`.
`Element.GetElementById` is a convenience for searching a subtree.

ID, class, and tag lookup arguments are literal values, without CSS escaping.
HTML tag names are case-insensitive. IDs and class tokens are case-sensitive.
Class names are separated by HTML whitespace; every requested class must match.

Selectors use Cascadia's supported CSS syntax. Invalid selectors return `nil`,
just like no matches, preserving the existing single-return query API. Selectors
are intended to be application constants; this API does not distinguish malformed
CSS from a valid selector without matches. Browser state selectors and browser
APIs are outside the server-side model.

All collection methods, including `Children`, return Go slice snapshots. Later
changes to the tree do not change slice membership or order. The elements inside
a slice still refer to the original nodes, so they can be updated or moved.
Empty collections are `nil` slices, suitable for `len` and `range`.

## Traversal and mutation

`ParentElement`, `Children`, `FirstElementChild`, `LastElementChild`,
`NextElementSibling`, and `PreviousElementSibling` navigate elements, skipping
text and comments. `ParentElement` returns `nil` for the document's `html` element.
`Contains(other)` includes both the receiver itself and all descendants.

Create detached nodes with `document.CreateElement("div")` and
`document.CreateTextNode("text")`, or the equivalent package functions. The
existing `Element` type represents both elements and text nodes.

| Method | Behavior |
| --- | --- |
| `AppendChild(child)` | Move a node to the end of the children |
| `Prepend(child)` | Move a node before all children, including text and comments |
| `InsertBefore(child, reference)` | Move a node before a direct child; `nil` reference appends |
| `ReplaceChild(newChild, oldChild)` | Move the replacement into the old child's position |
| `RemoveChild(child)` | Detach a direct child |
| `Remove()` | Detach this node; safe to repeat |
| `CloneNode(deep)` | Create a detached, independent copy; `true` includes all descendants |

Insertion moves the same node rather than copying it. Clone explicitly to keep
the original. References obtained before a move remain usable after the move.
References returned by separate queries may be different Go wrappers around the
same node; do not use wrapper pointer equality as DOM identity.

Structural mutations return `error`, except the idempotent `Remove`. Invalid
parents, nil children, foreign references, and cycles are rejected before any
node is detached. Mutation methods return only errors; retain the child reference
passed to them. `Prepend` takes one node, and text insertion uses `CreateTextNode`.

## Content, attributes, and classes

Use getters and setters for DOM properties:

```go
label.SetTextContent("<Hello>") // Escaped when rendered.
text := label.TextContent()

if err := panel.SetInnerHTML("<strong>Hello</strong>"); err != nil {
    return err
}
markup := panel.InnerHTML()
outer := panel.OuterHTML()

button.SetAttribute("disabled", "")
value, exists := button.GetAttribute("disabled")
disabled := button.HasAttribute("disabled")
button.RemoveAttribute("disabled")
```

`SetTextContent` replaces existing children, and an empty string removes them all.
On text nodes it updates the text itself. `SetInnerHTML` parses in the receiver's
context, including table and select elements, and detaches previous children.
`InnerHTML` and `OuterHTML` serialize HTML; `TextContent` excludes comments and markup.
`element.Render()` remains an alias for `OuterHTML`. `page.Render()` serializes the
complete document.

`GetAttribute` returns `(string, bool)` so missing and empty attributes are
separate cases. HTML attribute names are case-insensitive; foreign element
attribute names retain their case.

`ClassList()` reads and updates the same attribute as `ClassName`, `SetClassName`,
and the attribute methods. A saved `ClassList` always sees the current value.

```go
classes := banner.ClassList()
banner.SetClassName("banner")
if err := classes.Add("active", "visible"); err != nil {
    return err
}
if err := classes.Remove("visible"); err != nil {
    return err
}
active, err := classes.Toggle("active")
if err != nil {
    return err
}
```

`Add` preserves token order and removes duplicates. `Remove` ignores absent tokens.
`Toggle` reports whether the token is present afterward. Empty tokens or tokens
containing HTML whitespace return an error without modifying the attribute.
`Contains` returns false for invalid tokens. Unicode spaces that are not HTML
whitespace remain part of the token.

## Targeted browser updates

The optional `livehttp` package serves a small shared ES module and writes targeted
HTML responses. `livews` sends the same payload over Gorilla WebSocket connections.
The rendering package itself still needs no browser JavaScript or frontend build step.

```go
import "github.com/renniemaharaj/live-components/livehttp"

mux := http.NewServeMux()
mux.Handle("/live.js", livehttp.RuntimeHandler())
mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
    page, err := live.Parse(`<span id="counter-value">0</span>`)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    value := page.GetElementById("counter-value")
    value.SetTextContent("42")

    if err := livehttp.WriteUpdates(w, value); err != nil {
        log.Printf("write updates: %v", err)
    }
})
```

Import the browser module from an application script loaded with `type="module"`:

```js
import { update } from "/live.js";

document.getElementById("refresh").addEventListener("click", async () => {
  await update("/refresh", { method: "POST" });
});
```

`update(url, options)` accepts ordinary `fetch` options and requests JSON with
`Accept: application/json`. It checks the HTTP status and applies the response's
`updates` array. It returns that array and rejects on HTTP, network, or decoding errors.
Applications decide which interactions make requests and how to display errors.

The wire format is:

```json
{"updates":[{"id":"counter-value","html":"42"}]}
```

`WriteUpdates(w, elements...)` sends each element's current `InnerHTML()`, in the
order supplied. It requires nonempty, unique target IDs and validates the batch
before writing headers or a body. No targets produces an empty array. Responses
use `application/json; charset=utf-8` and `Cache-Control: no-store`.

The browser assigns each fragment to `document.getElementById(id).innerHTML`.
Missing browser targets are skipped. Each target element keeps its identity,
attributes, and listeners; its children are recreated. Elements outside those
targets keep their values, focus, and listeners. Only use HTML produced by your
application, with user text escaped through methods such as `SetTextContent`.

Go explicitly selects the smallest useful container with an ID. To change text,
target its span or output. To change a child's attributes, insert children, or
remove children, target their nearest suitable parent. Avoid targeting a form
or other broad container when only one value changes. There is no automatic
tree diff, attribute patching, form interception, or input preservation layer.
If targets overlap, updates apply in order; prefer selecting the parent once.

`live.NewUpdates(elements...)` builds a `live.UpdateBatch` without writing it to
a transport. Each entry is a `live.Update`; `livehttp.Update` remains an alias.
The batch contains a snapshot of the selected elements' HTML, so later changes
to a rendering tree do not change earlier batches. `applyUpdates(updates)` is
also exported for applications using another transport, such as SSE.

The [counter example](../examples/counter/README.md) shows two small targets,
HTTP and WebSocket actions through the module, and native form fallback.

## WebSocket updates with Gorilla

`livehttp` remains the request-based transport. Use `livews.WriteUpdates` with an
ordinary `*websocket.Conn` for persistent, bidirectional updates. Both transports
use the same `{"updates":[{"id":"counter-value","html":"42"}]}` format and the
same browser DOM updater.

```go
import (
    "github.com/gorilla/websocket"
    "github.com/renniemaharaj/live-components/livews"
)

upgrader := websocket.Upgrader{HandshakeTimeout: 5 * time.Second}
mux.HandleFunc("GET /live", func(w http.ResponseWriter, r *http.Request) {
    page, err := live.Parse(`<span id="counter-value">0</span>`)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    conn.SetReadLimit(4096)

    value := page.GetElementById("counter-value")
    for {
        var action struct {
            Value string `json:"value"`
        }
        if err := conn.ReadJSON(&action); err != nil {
            return
        }

        value.SetTextContent(action.Value)
        if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
            return
        }

        if err := livews.WriteUpdates(conn, value); err != nil {
            return
        }
    }
})
```

The browser connects once and applies each incoming update automatically:

```js
import { connect } from "/live.js";

const socket = connect("/live");

socket.addEventListener("open", () => {
  socket.send(JSON.stringify({ value: "42" }));
});

socket.addEventListener("updateerror", (event) => {
  console.error(event.detail);
});
```

`connect(url)` returns a native `WebSocket`. Relative URLs resolve against the
page, with HTTP converted to `ws:` and HTTPS converted to `wss:`. Use the native
`open`, `error`, and `close` events and send application actions with `send()`
after the connection opens. Incoming JSON is passed to the same `applyUpdates`
function used by HTTP. A JSON message with an `error` string emits `updateerror`
with an `Error` in `event.detail` and leaves the DOM unchanged. JSON parsing or
update application failures also emit that event.

Go can send updates without waiting for a client action. Keep one rendering tree
per connection or otherwise give it a single owner. Cache target references when
reusing the tree. For fan-out to many clients, build a batch with `live.NewUpdates`
and use Gorilla's prepared messages rather than repeatedly rendering the same HTML.

Gorilla requires one reader and one writer per connection. Serialize updates and
other data writes in one writer goroutine or under a mutex. Configure upgrades,
origins, deadlines, read limits, heartbeat, and connection lifetime in your server.
Keep Gorilla's default same-origin check unless your application needs a defined
cross-origin policy. See [Gorilla's documentation](https://pkg.go.dev/github.com/gorilla/websocket).

The module does not reconnect or replay actions after a disconnect. Applications
choose when to reconnect and fetch current state, avoiding accidental duplicate
actions. The counter reports a closed connection and offers a reload or HTTP mode.

The counter reuses one tree per WebSocket, pools write buffers, sends initial state
without a client action, and uses ping/pong heartbeats and bounded writes. Its
WebSocket route bypasses the kit's buffering timeout while keeping the other
default middleware. Servers must support HTTP connection hijacking for upgrades.

## Go conventions and compatibility

- Public rendering types are `Component`, `Document`, `Element`, and `ClassList`.
  `Update`, `UpdateBatch`, and `NewUpdates` define transport-independent updates.
- Read methods tolerate nil and zero-value receivers, returning empty results.
  Simple setters and `Remove` are no-ops on absent nodes. Structural mutations and
  `SetInnerHTML` return errors on absent nodes. Check query results when an element
  might be missing rather than relying on no-op setters.
- Existing package constructors, component query shortcuts, `Root`, and element
  `Render` remain available. `Root` is the shortcut for
  `page.Document().DocumentElement()`.
- `ClassList.Add` and `Remove` now return errors. Existing standalone calls still
  compile, but callers should handle errors. `Toggle` now returns `(bool, error)`;
  update callers that previously assigned its single boolean result. Interfaces
  declaring the old class mutation signatures also need updating.
- `Component.Doc`, `Element.Node`, and `ClassList.Node` remain low-level compatibility
  fields. Prefer DOM methods and document creation over constructing parser nodes
  or editing tree links directly. Such edits bypass validation and any future
  synchronization hooks.
- Tree access is synchronous and not safe for concurrent mutation. This package
  does not implement browser events, layout, live collections, or client state
  synchronization. Optional HTTP delivery and explicit browser updates are
  provided separately by `livehttp` and `livews`.

See [the API review](api-review.md) for the complete public-surface review and
rationale. Run `go test ./... ./examples/...` to check the library and both examples.
