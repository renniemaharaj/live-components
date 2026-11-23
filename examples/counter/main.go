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
	"github.com/renniemaharaj/live-components/livehttp"

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

		page, err := renderCounter(string(markup), value)
		if err != nil {
			h.Error.Handle(err)
			return
		}

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

		count, err = nextCount(count, r.PostForm.Get("action"))
		if err != nil {
			h.Error.BadRequest(err.Error())
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			page, err := renderCounter(string(markup), count)
			if err != nil {
				h.Error.Handle(err)
				return
			}

			if err := livehttp.WriteUpdates(h.Writer(),
				page.GetElementById("count"),
				page.GetElementById("counter-state"),
			); err != nil {
				log.Printf("update counter: %v", err)
			}
			return
		}

		// Redirect so refreshing the page does not resubmit the form.
		h.Redirect("/?count="+strconv.FormatInt(count, 10), http.StatusSeeOther)
	}

	middleware, err := kit.DefaultMiddleware()
	if err != nil {
		return nil, err
	}

	// Kit documents its timeout as the second default middleware.
	// Skip only that buffer on the WebSocket route; retain authentication and limits.
	requestTimeout := middleware[1]
	middleware[1] = func(next http.Handler) http.Handler {
		timed := requestTimeout(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/counter/live" {
				next.ServeHTTP(w, r)
				return
			}

			timed.ServeHTTP(w, r)
		})
	}

	return v8.NewHandlerWithMiddleware(nil, middleware, func(router *kit.Router) {
		files := http.StripPrefix("/static/", http.FileServerFS(static))
		router.Handle(http.MethodGet, "/static/*", files)
		router.Handle(http.MethodHead, "/static/*", files)
		runtime := livehttp.RuntimeHandler()
		router.Handle(http.MethodGet, "/live.js", runtime)
		router.Handle(http.MethodHead, "/live.js", runtime)
		router.Get("/", showPage)
		router.Head("/", showPage)
		router.Post("/counter", changeCount)
		router.Handle(http.MethodGet, "/counter/live", counterSocket(string(markup)))
	})
}

func nextCount(count int64, action string) (int64, error) {
	switch action {
	case "increment":
		count++
	case "decrement":
		count--
	case "reset":
		count = 0
	default:
		return 0, fmt.Errorf("Invalid counter action.")
	}

	if count < math.MinInt32 || count > math.MaxInt32 {
		return 0, fmt.Errorf("Counter limit reached.")
	}

	return count, nil
}

func renderCounter(markup string, value int64) (*live.Component, error) {
	// Each request owns its DOM tree. The embedded HTML stays unchanged.
	page, err := live.Parse(markup)
	if err != nil {
		return nil, err
	}

	count := strconv.FormatInt(value, 10)
	page.GetElementById("count").SetTextContent(count)
	page.GetElementById("current-count").SetAttribute("value", count)

	return page, nil
}
