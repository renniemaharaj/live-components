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

## Go conventions and compatibility

- Public types are `Component`, `Document`, `Element`, and `ClassList`.
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
  synchronization.

See [the API review](api-review.md) for the complete public-surface review and
rationale. Run `go test ./... ./examples/...` to check the library and both examples.
