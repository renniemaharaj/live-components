# Counter starter

A small Live Components example: ordinary HTML, go-api-kit, and a counter that
works without JavaScript. The page uses the cyan, purple, and pink brand palette.

The examples use the sibling `../go-api-kit` checkout through their own Go module.
Go 1.25.3 or newer is required. The root `go.work` keeps these commands usable from
the repository root without adding HTTP toolkit dependencies to the core library:

```sh
go run ./examples/counter
```

Open **http://127.0.0.1:9121**. To choose another address:

```sh
go run ./examples/counter -addr 127.0.0.1:9200
```

## How it works

- `index.html` is ordinary HTML, embedded with its CSS and SVG logo.
- go-api-kit registers the page, form, and asset routes through `v8.NewHandler`.
- The plus, minus, and reset buttons submit an HTML form to `POST /counter`.
- Go reads the submitted count, applies the action, and redirects to `/?count=1`.
- The GET handler parses a fresh DOM tree with `live.Parse`, updates the output
  using `SetTextContent`, and updates the hidden field using `SetAttribute`.
- `h.Render(http.StatusOK, page)` sends the complete HTML response.

The counter uses the kit's normal environment-driven middleware and built-in
`/health` endpoint. GET and HEAD are registered explicitly. The small HTTP server
retains the `-addr` flag for the local example.

The redirect prevents form resubmission on refresh. The count is carried in the
form and URL; it is intentionally editable, with no database, cookies, or shared
server counter. Each request owns its DOM tree. Changes never write back to the
template, and restarting at `/` starts from zero. Counts are signed 32-bit integers.

Edit the HTML, CSS, or Go handler and restart `go run` to rebuild embedded assets.
There is no frontend build step or live-reload dependency.

Check the core library and both examples from the repository root:

```sh
go test ./... ./examples/...
go test -race ./examples/...
go vet ./... ./examples/...
```
