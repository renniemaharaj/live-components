# Public API review

The review covered every exported type, field, package function, and method in the
original four source files before implementation. The initial code did not build:
it called the nonexistent `goquery.Compile`. Its selector traversal was otherwise
descendant-only, but ID and tag lookups incorrectly delegated literal values to CSS.

The primary design is a document with reusable node references. Document access
and component shortcuts use the same element operations, so root and nested
behavior share one implementation. There are no callback constructors, option
objects, reflected properties, or component-specific lookup names.

## Types, fields, and package functions

| Export | Decision |
| --- | --- |
| `Component` | Keep the page owner, with `Document()` as the normal DOM entry point |
| `Element` | Keep reusable references, including existing text-node support |
| `ClassList` | Add a public alias so callers can name an already exposed return type |
| `Document` | Add a distinct query and creation view, keeping document nodes out of ordinary child mutation |
| `Component.Doc` | Retain compatibility access to parser internals; prefer `Document` |
| `Element.Node`, `ClassList.Node` | Retain compatibility fields; prefer validated DOM methods |
| `Load(path)` | Keep the short constructor with explicit file errors; expose `*Component` in the signature |
| `Parse(markup)` | Keep the string constructor and its error result; expose `*Component` in the signature |
| `CreateElement(tag)` | Keep package convenience; initialize parser atom metadata so created elements support fragment parsing |
| `CreateTextNode(text)` | Keep explicit text-node creation and escaped serialization |

The public aliases make all returned types nameable through `live`, without
requiring imports from internal packages. The internal packages' construction,
serialization, tokenization, and selector helpers are implementation details.

## Component and document methods

| Export | Decision |
| --- | --- |
| `Component.Document` | Add access to the existing tree without reparsing or cloning |
| `Component.QuerySelector`, `QuerySelectorAll` | Keep shortcuts; first match and document-order snapshots |
| `Component.GetElementById` | Keep; match the exact ID rather than constructing CSS |
| `Component.GetElementsByTagName` | Keep; match literal tags and the wildcard rather than arbitrary CSS |
| `Component.GetElementsByClassName` | Add the useful class shortcut with all-token matching |
| `Component.Render` | Keep whole-document string output |
| `Component.Root` | Keep compatibility convenience for `DocumentElement` |
| `Document.DocumentElement` | Add recognizable access to the document's root element |
| `Document.QuerySelector`, `QuerySelectorAll` | Add the same query contracts as component and element |
| `Document.GetElementById` | Add literal ID lookup across the whole document |
| `Document.GetElementsByTagName` | Add literal tag lookup across the whole document |
| `Document.GetElementsByClassName` | Add all-token class lookup across the whole document |
| `Document.CreateElement`, `CreateTextNode` | Add familiar creation methods without options or ownership constraints |

A document view does not create another tree. Nodes can be moved across parents
or documents; there is no browser `ownerDocument` restriction. A missing document
has empty query results. Detached node construction needs no document state.

## Element methods

| Export | Decision |
| --- | --- |
| `QuerySelector` | Keep descendant-only matching; stop immediately at the first match |
| `QuerySelectorAll` | Keep descendant-only, document-order snapshot results |
| `GetElementById` | Keep subtree convenience; fix CSS metacharacters and empty-ID behavior |
| `GetElementsByTagName` | Keep; fix literal matching, HTML case folding, and wildcard behavior |
| `GetElementsByClassName` | Add literal, all-token matching using the same token rules as `ClassList` |
| `ParentElement` | Keep; inspect only the immediate parent, excluding the document |
| `Children` | Keep direct-element snapshot results |
| `FirstElementChild`, `LastElementChild` | Add direct access without allocating a collection |
| `NextElementSibling`, `PreviousElementSibling` | Add navigation that skips text and comments |
| `Contains` | Add self-inclusive descendant checking, also shared by cycle validation |
| `AppendChild` | Keep error-only mutation; move the existing node |
| `Prepend` | Add the common single-node operation before all node types |
| `InsertBefore` | Keep; validate references and cycles before detaching; nil reference appends |
| `ReplaceChild` | Keep; use underlying-node identity for self-replacement, including distinct wrappers |
| `RemoveChild` | Keep direct-child validation with explicit errors |
| `Remove` | Keep idempotent, no-result detachment |
| `CloneNode` | Keep shallow or recursive independent copies, including attributes and text |
| `GetAttribute` | Keep `(value, exists)` to distinguish missing and empty attributes |
| `HasAttribute` | Add the common presence check without discarding a getter result |
| `SetAttribute`, `RemoveAttribute` | Keep direct setters; consistently fold HTML attribute names |
| `ClassName`, `SetClassName` | Keep raw class attribute access |
| `ClassList` | Keep a current view of the same attribute, including nil-safe access |
| `TextContent` | Keep descendant text without markup or comments |
| `SetTextContent` | Keep child replacement; fix text-node data updates |
| `InnerHTML`, `SetInnerHTML` | Keep child serialization and contextual parsing with errors |
| `OuterHTML` | Add the recognizable name for serializing the whole element |
| `Render` | Keep as a compatibility alias for `OuterHTML` |

## ClassList methods

| Export | Decision |
| --- | --- |
| `Contains` | Keep case-sensitive, single-token presence checks |
| `Add` | Return an error for invalid tokens; normalize duplicates and preserve order |
| `Remove` | Return an error for invalid tokens; ignore absent valid tokens |
| `Toggle` | Return `(present, error)` so invalid input can be reported without panics |

Class token validation happens before mutation. Tokenization uses HTML's five
ASCII whitespace characters rather than Go's broader Unicode whitespace rules.
The saved list object reads the node's current attribute on every operation.

## Deliberate limits and stability

All collection methods return Go slice snapshots, not live collections. All
mutations act on the underlying node, including references captured by an earlier
query or collection. Mutation methods return errors instead of echoing their
input node, since the caller already has that reference.

Selector methods retain one result to preserve existing callers and the requested
short query expressions. Invalid selectors produce nil results, as before. This
tradeoff is documented rather than adding a second parallel family of checked
queries. CSS selector support comes from Cascadia; browser state and layout are
outside the package's contract.

Keep the useful DOM operations without adding events, style objects, document
fragments, live collection wrappers, namespace constructors, or generic mutation
callbacks. `Prepend` accepts a single node; the text-node constructor handles text.
`CloneNode` deliberately retains the boolean deep argument familiar from the DOM.

No state synchronization is introduced here. Future tracking can be added at the
shared mutation methods without changing query names, slice semantics, or retained
node references. Direct edits through compatibility parser fields bypass those
methods; new code should avoid them. Removing those fields would require a
separate compatibility decision.

Regression tests cover the public API from an external package, including the
load/query/create/render workflow, literal lookups, descendant order, snapshots,
node moves and failure atomicity, traversal, independent cloning, content
replacement, class sharing and validation, and nil/zero-value behavior.
