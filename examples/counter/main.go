package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	live "github.com/renniemaharaj/live-components"

	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

//go:embed index.html static/*
var assets embed.FS

func main() {
	address := flag.String("addr", "127.0.0.1:9121", "HTTP listen address")
	flag.Parse()

	handler, err := newHandler()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              *address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Printf("Live Components starter: http://%s\n", *address)
	log.Fatal(server.ListenAndServe())
}

func newHandler() (http.Handler, error) {
	markup, err := assets.ReadFile("index.html")
	if err != nil {
		return nil, err
	}

	static, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}

	showPage := func(h kit.Handle) {
		count := h.Query("count")
		if count == "" {
			count = "0"
		}

		value, err := strconv.ParseInt(count, 10, 32)
		if err != nil {
			h.Error.BadRequest("Invalid count.")
			return
		}

		// Each request owns its DOM tree. The embedded HTML stays unchanged.
		page, err := live.Parse(string(markup))
		if err != nil {
			h.Error.Handle(err)
			return
		}

		count = strconv.FormatInt(value, 10)
		page.GetElementById("count").SetTextContent(count)
		page.GetElementById("current-count").SetAttribute("value", count)

		h.Writer().Header().Set("Cache-Control", "no-store")
		if err := h.Render(http.StatusOK, page); err != nil {
			log.Printf("render counter: %v", err)
		}
	}
	changeCount := func(h kit.Handle) {
		r := h.Request()
		r.Body = http.MaxBytesReader(h.Writer(), r.Body, 1024)
		if err := r.ParseForm(); err != nil {
			h.Error.BadRequest("Invalid counter form.")
			return
		}

		count, err := strconv.ParseInt(r.PostForm.Get("count"), 10, 32)
		if err != nil {
			h.Error.BadRequest("Invalid count.")
			return
		}

		switch r.PostForm.Get("action") {
		case "increment":
			count++
		case "decrement":
			count--
		case "reset":
			count = 0
		default:
			h.Error.BadRequest("Invalid counter action.")
			return
		}

		if count < math.MinInt32 || count > math.MaxInt32 {
			h.Error.BadRequest("Counter limit reached.")
			return
		}

		// Redirect so refreshing the page does not resubmit the form.
		h.Redirect("/?count="+strconv.FormatInt(count, 10), http.StatusSeeOther)
	}

	return v8.NewHandler(nil, func(router *kit.Router) {
		files := http.StripPrefix("/static/", http.FileServerFS(static))
		router.Handle(http.MethodGet, "/static/*", files)
		router.Handle(http.MethodHead, "/static/*", files)
		router.Get("/", showPage)
		router.Head("/", showPage)
		router.Post("/counter", changeCount)
	})
}
