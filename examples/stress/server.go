package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	live "github.com/renniemaharaj/live-component"

	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

type lab struct {
	runner  *runner
	streams chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newLab() *lab {
	return &lab{
		runner:  newRunner(),
		streams: make(chan struct{}, 32),
		closed:  make(chan struct{}),
	}
}

func (app *lab) closeStreams() {
	app.once.Do(func() {
		close(app.closed)
	})
}

func (app *lab) routes() http.Handler {
	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	state := func(h kit.Handle) {
		writeJSON(h, http.StatusOK, app.runner.snapshot())
	}
	results := func(h kit.Handle) {
		writeJSON(h, http.StatusOK, app.runner.snapshot().Results)
	}

	// Kit's default timeout buffers responses, which would break SSE streaming.
	handler, err := v8.NewHandlerWithMiddleware(nil, []kit.Middleware{}, func(router *kit.Router) {
		files := http.StripPrefix("/static/", http.FileServerFS(static))
		router.Handle(http.MethodGet, "/static/*", files)
		router.Handle(http.MethodHead, "/static/*", files)
		router.Get("/", app.index)
		router.Head("/", app.index)
		router.Get("/api/state", state)
		router.Head("/api/state", state)
		router.Get("/api/results", results)
		router.Head("/api/results", results)
		router.Handle(http.MethodGet, "/api/events", http.HandlerFunc(app.events))
		router.Handle(http.MethodHead, "/api/events", http.HandlerFunc(app.events))
		router.Post("/api/run", app.start)
		router.Post("/api/stop", func(h kit.Handle) {
			if !sameOrigin(h.Request()) {
				h.Error.Status(http.StatusForbidden, "cross-origin requests are not allowed")
				return
			}
			app.runner.stop()
			writeJSON(h, http.StatusOK, map[string]string{"status": "stopping"})
		})
		router.Get("/api/preview", app.preview)
		router.Head("/api/preview", app.preview)
	})
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		handler.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == r.Host
}

func writeJSON(h kit.Handle, status int, value any) {
	if err := h.JSON(status, value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func (app *lab) start(h kit.Handle) {
	r := h.Request()
	if !sameOrigin(r) {
		h.Error.Status(http.StatusForbidden, "cross-origin requests are not allowed")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(h.Writer(), r.Body, 4096))
	decoder.DisallowUnknownFields()
	var c config
	if err := decoder.Decode(&c); err != nil {
		writeJSON(h, http.StatusBadRequest, map[string]string{"error": "invalid configuration: " + err.Error()})
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeJSON(h, http.StatusBadRequest, map[string]string{"error": "send exactly one configuration object"})
		return
	}
	if err := app.runner.start(c); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBusy) {
			status = http.StatusConflict
		}
		writeJSON(h, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(h, http.StatusAccepted, app.runner.snapshot())
}

func (app *lab) index(h kit.Handle) {
	markup, err := assets.ReadFile("templates/index.html")
	if err != nil {
		h.Error.Handle(err)
		return
	}
	page, err := live.Parse(string(markup))
	if err != nil {
		h.Error.Handle(err)
		return
	}

	document := page.Document()
	options := document.GetElementById("suite-select")
	list := document.GetElementById("suite-list")
	for _, s := range suites {
		option := document.CreateElement("option")
		option.SetAttribute("value", s.ID)
		option.SetTextContent(s.Name)
		if s.ID == "dom" {
			option.SetAttribute("selected", "")
		}
		if err := options.AppendChild(option); err != nil {
			h.Error.Handle(err)
			return
		}

		row := document.CreateElement("tr")
		row.SetAttribute("data-suite", s.ID)
		for _, text := range []string{s.Name, s.Description, "Ready"} {
			cell := document.CreateElement("td")
			cell.SetTextContent(text)
			if err := row.AppendChild(cell); err != nil {
				h.Error.Handle(err)
				return
			}
		}
		row.LastElementChild().SetClassName("suite-status")
		if err := list.AppendChild(row); err != nil {
			h.Error.Handle(err)
			return
		}
	}
	for _, operation := range operations {
		option := document.CreateElement("option")
		option.SetAttribute("value", operation)
		option.SetTextContent(strings.ReplaceAll(operation, "-", " "))
		if err := document.GetElementById("operation-select").AppendChild(option); err != nil {
			h.Error.Handle(err)
			return
		}
	}

	preview, err := renderPreview(url.Values{})
	if err != nil {
		h.Error.Handle(err)
		return
	}
	if err := document.GetElementById("preview-stage").SetInnerHTML(preview); err != nil {
		h.Error.Handle(err)
		return
	}
	if err := h.Render(http.StatusOK, page); err != nil {
		log.Printf("render dashboard: %v", err)
	}
}

func (app *lab) events(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/event-stream")
		return
	}
	select {
	case app.streams <- struct{}{}:
		defer func() { <-app.streams }()
	default:
		http.Error(w, "too many event streams", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		value, err := json.Marshal(app.runner.snapshot())
		if err != nil {
			return
		}
		controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", value); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-ticker.C:
		case <-r.Context().Done():
			return
		case <-app.closed:
			return
		}
	}
}

func (app *lab) preview(h kit.Handle) {
	markup, err := renderPreview(h.QueryValues())
	if err != nil {
		writeJSON(h, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(h, http.StatusOK, map[string]string{"html": markup})
}

func renderPreview(values url.Values) (string, error) {
	template := values.Get("template")
	if template == "" {
		template = "banner"
	}
	if template != "banner" && template != "card" && template != "table" {
		return "", fmt.Errorf("unknown preview template")
	}
	name := values.Get("name")
	if name == "" {
		name = "Ada"
	}
	if len(name) > 80 {
		return "", fmt.Errorf("preview name must be at most 80 bytes")
	}
	counts := make(map[string]int)
	for _, key := range []string{"children", "clones"} {
		if values.Get(key) == "" {
			continue
		}
		number, err := strconv.Atoi(values.Get(key))
		if err != nil || number < 0 || number > 8 || key == "clones" && number > 3 {
			return "", fmt.Errorf("preview children are limited to 8 and clones to 3")
		}
		counts[key] = number
	}

	selector := "section"
	if template == "card" {
		selector = "article"
	}
	root, err := templateElement(template, selector)
	if err != nil {
		return "", err
	}
	root.SetAttribute("data-component", template)
	title := root.GetElementById("preview-title")
	message := root.GetElementById("preview-message")
	if template == "card" {
		root.SetClassName("demo-card")
		title = root.GetElementById("label")
		message = root.QuerySelector(".detail")
	}
	title.SetTextContent("Hello " + name)
	message.SetTextContent("Welcome, " + name + "!")
	if values.Get("active") == "true" {
		if err := root.ClassList().Add("highlighted"); err != nil {
			return "", err
		}
	}
	if values.Get("attribute") == "true" {
		root.SetAttribute("data-version", "mutated")
	}
	if values.Get("removed") == "true" {
		root.GetElementById("preview-button").Remove()
	}
	children := counts["children"]
	if values.Get("action") == "remove-child" {
		children++
	}
	childContainer := root
	if template == "table" {
		childContainer = root.QuerySelector("tbody")
	}
	for index := 0; index < children; index++ {
		child := live.CreateElement("span")
		child.SetClassName("demo-child")
		child.SetTextContent("Child " + strconv.Itoa(index+1))
		if template == "table" {
			child = live.CreateElement("tr")
			for _, text := range []string{"Member " + strconv.Itoa(index+1), "Component", "Inserted"} {
				cell := live.CreateElement("td")
				cell.SetTextContent(text)
				if err := child.AppendChild(cell); err != nil {
					return "", err
				}
			}
		}
		if err := childContainer.AppendChild(child); err != nil {
			return "", err
		}
	}
	if values.Get("action") == "remove-child" {
		if err := childContainer.RemoveChild(childContainer.LastElementChild()); err != nil {
			return "", err
		}
	}

	container := live.CreateElement("div")
	container.SetClassName("preview-stack")
	if err := container.AppendChild(root); err != nil {
		return "", err
	}
	clones := counts["clones"]
	if values.Get("action") == "remove-clone" {
		clones++
	}
	for index := 0; index < clones; index++ {
		clone := root.CloneNode(true)
		cloneTitle := clone.QuerySelector("h2, h3")
		cloneTitle.SetTextContent("Hello " + name + " · clone " + strconv.Itoa(index+1))
		clone.RemoveAttribute("id")
		for _, child := range clone.QuerySelectorAll("[id]") {
			child.RemoveAttribute("id")
		}
		if err := container.AppendChild(clone); err != nil {
			return "", err
		}
	}
	if values.Get("action") == "remove-clone" {
		container.LastElementChild().Remove()
	}

	return container.Render(), nil
}
