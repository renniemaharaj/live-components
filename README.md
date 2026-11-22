# Live Components

**No JavaScript required.**

A Go foundation for rendering HTML and composing components with familiar DOM
operations. Build with ordinary HTML, then choose the browser experience your
application needs.

```sh
go get github.com/renniemaharaj/live-components
```

Import as `live "github.com/renniemaharaj/live-components"`:

```go
page, err := live.Parse(`<button id="count">0</button>`)
if err != nil {
    return err
}

page.GetElementById("count").SetTextContent("1")
html := page.Render()
```

## HTML first. Enhancements optional.

Live Components parses, modifies, and renders HTML in Go. It requires no browser
runtime or frontend build step, and can render complete pages or component fragments.

- **HTML only:** use native links and forms. Go processes requests and returns
  updated pages, including when JavaScript is disabled.
- **HTMX:** optionally add [HTMX](https://htmx.org/docs/) attributes and its browser
  library to request HTML fragments and update selected components in place.
- **Custom JavaScript:** use the same rendered HTML with `fetch`, SSE, WebSockets,
  or another browser integration suited to your application.

HTMX and JavaScript-based updates require JavaScript in the browser. Choose these
enhancements per application; keep native links, forms, and complete page responses
when you need a working fallback. The rendering foundation stays in Go; applications
configure their routes, state, and browser updates.

## Where it fits

Choose the interaction style for each workflow, or combine them in one application:

| Approach | Good fit | User experience |
| --- | --- | --- |
| **HTML only** | Administration and inventory tools, document portals, booking forms, catalogs, reports, and approval workflows | Users follow links or submit forms; Go returns the updated page. |
| **Optional HTMX** | Filtered tables, pagination, inline editing, and server validation feedback | Go returns HTML fragments; HTMX updates the relevant part of the page. |
| **JavaScript enhancements** | Live charts, chat and notifications, drag-and-drop interfaces, and rich editors | Browser code handles continuous interaction, with Go providing application endpoints and any required HTML. |

With [go-api-kit](https://github.com/renniemaharaj/go-api-kit) handling HTTP and
Live Components rendering HTML, your application can keep validation, business
logic, and data access in Go. Add HTMX or custom JavaScript to the screens that
benefit from it, while keeping other workflows fully functional without JavaScript.
This combines a native Go backend with the browser experience each application needs.

## Try the counter

A small Go + HTML starter with increment, decrement, and reset. Works without
JavaScript, with [go-api-kit](https://github.com/renniemaharaj/go-api-kit) serving
the page and form requests.

![Live Components counter starter](docs/images/counter.png)

With Go 1.25.3 or newer, clone both repositories side by side:

```sh
git clone https://github.com/renniemaharaj/live-components.git
git clone https://github.com/renniemaharaj/go-api-kit.git
cd live-components
go run ./examples/counter
```

Open **http://127.0.0.1:9121**.

## Examples

- [Counter starter](examples/counter/README.md): native forms and HTML responses;
  fully functional with JavaScript disabled.
- [Live Components Lab](examples/stress/README.md): component previews, stress tests,
  and benchmarks; uses JavaScript for live telemetry and preview updates.

Each example has its own screenshot, setup instructions, and explanation.

## Documentation

See the [API guide](docs/api.md) for selectors, traversal, mutations, and rendering,
and the [API review](docs/api-review.md) for design and compatibility notes.

## License

[MIT](LICENSE).
