package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	live "github.com/renniemaharaj/live-components"

	"github.com/renniemaharaj/go-api-kit/kit"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type workload struct {
	instances int
	nodes     int
	unit      string
	step      func(context.Context, int, int) (sample, error)
	close     func()
}

func templateElement(name, selector string) (*live.Element, error) {
	markup, err := assets.ReadFile("templates/" + name + ".html")
	if err != nil {
		return nil, err
	}
	page, err := live.Parse(string(markup))
	if err != nil {
		return nil, err
	}
	element := page.QuerySelector(selector)
	if element == nil {
		return nil, fmt.Errorf("%s template is missing %s", name, selector)
	}
	element.Remove()
	return element, nil
}

func countNodes(markup string) (int, error) {
	context := &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	}
	nodes, err := html.ParseFragment(strings.NewReader(markup), context)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, node := range nodes {
		count++
		for range node.Descendants() {
			count++
		}
	}

	return count, nil
}

func componentPool(ctx context.Context, count int) ([]*live.Element, int, error) {
	base, err := templateElement("card", "article")
	if err != nil {
		return nil, 0, err
	}
	nodes, err := countNodes(base.Render())
	if err != nil {
		return nil, 0, err
	}

	pool := make([]*live.Element, count)
	for index := range pool {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		pool[index] = base.CloneNode(true)
	}

	return pool, nodes * count, nil
}

func prepareWorkload(ctx context.Context, c config) (*workload, error) {
	pool, nodes, err := componentPool(ctx, c.Components)
	if err != nil {
		return nil, err
	}

	work := &workload{
		instances: len(pool),
		nodes:     nodes,
		unit:      suiteByID(c.Suite).Unit,
	}
	switch c.Suite {
	case "dom":
		if c.Operation != "mixed" {
			work.unit = "DOM operations"
		}
		work.step = func(ctx context.Context, component, iteration int) (sample, error) {
			if c.Operation == "mixed" {
				return mutateComponent(pool[component], strconv.Itoa(iteration))
			}

			return measureDOMOperation(pool[component], c.Operation)
		}
	case "memory":
		work.step = func(ctx context.Context, component, iteration int) (sample, error) {
			started := time.Now()
			fresh, err := templateElement("card", "article")
			if err != nil {
				return sample{}, err
			}
			value, err := mutateComponent(fresh, strconv.Itoa(iteration))
			value.latency = time.Since(started)
			pool[component] = fresh
			return value, err
		}
	case "composition":
		work.step = func(ctx context.Context, component, iteration int) (sample, error) {
			return composeComponent(ctx, pool[component], c.Shape)
		}
	case "selectors", "render":
		root, err := composedTree(ctx, pool, c.Shape)
		if err != nil {
			return nil, err
		}
		work.nodes, err = countNodes(root.Render())
		if err != nil {
			return nil, err
		}
		workers := min(c.Concurrency, c.Components)
		if work.nodes*workers > 1000000 {
			return nil, fmt.Errorf("composed dataset exceeds the 1,000,000 in-flight node budget; use at most %d workers or fewer components", 1000000/work.nodes)
		}
		if c.Suite == "selectors" {
			work.step = func(ctx context.Context, component, iteration int) (sample, error) {
				return queryTree(root, component, c.Components)
			}
		} else {
			work.step = func(ctx context.Context, component, iteration int) (sample, error) {
				return renderTree(root, c.Components)
			}
		}
	case "concurrent":
		return requestWorkload(ctx, c, pool, nodes)
	default:
		return nil, fmt.Errorf("unknown workload %q", c.Suite)
	}

	return work, nil
}

func mutateComponent(root *live.Element, marker string) (sample, error) {
	started := time.Now()
	value := sample{}
	label := root.GetElementById("label")
	slot := root.QuerySelector(".slot")
	all := root.QuerySelectorAll(".detail")
	tags := root.GetElementsByTagName("h3")
	value.domCalls += 4
	if label == nil || slot == nil || len(all) != 1 || len(tags) != 1 {
		return value, fmt.Errorf("component query structure is incorrect")
	}
	label.SetTextContent("Mutation " + marker)
	root.SetAttribute("data-step", marker)
	root.SetAttribute("data-temporary", "remove me")
	root.SetClassName("component-card")
	if err := root.ClassList().Add("active"); err != nil {
		return value, err
	}
	value.domCalls += 5

	child := live.CreateElement("span")
	child.SetTextContent("Inserted")
	if err := slot.AppendChild(child); err != nil {
		return value, err
	}
	clone := child.CloneNode(true)
	if err := slot.InsertBefore(clone, child); err != nil {
		return value, err
	}
	replacement := live.CreateElement("em")
	replacement.SetTextContent("Replacement")
	if err := slot.ReplaceChild(replacement, child); err != nil {
		return value, err
	}
	if err := slot.RemoveChild(clone); err != nil {
		return value, err
	}
	replacement.Remove()
	if err := slot.SetInnerHTML(`<strong class="final">Final</strong>`); err != nil {
		return value, err
	}
	root.RemoveAttribute("data-temporary")
	value.domCalls += 10

	renderStarted := time.Now()
	output := root.Render()
	value.renderLatency = time.Since(renderStarted)
	value.renders = 1
	value.outputBytes = uint64(len(output))
	value.latency = time.Since(started)

	if label.TextContent() != "Mutation "+marker || root.ClassName() != "component-card active" ||
		!root.ClassList().Contains("active") || slot.TextContent() != "Final" ||
		child.ParentElement() != nil || clone.ParentElement() != nil || replacement.ParentElement() != nil || root.HasAttribute("data-temporary") {
		return value, fmt.Errorf("DOM mutation correctness assertion failed")
	}
	if attribute, exists := root.GetAttribute("data-step"); !exists || attribute != marker {
		return value, fmt.Errorf("attribute mutation was not retained")
	}
	parsed, err := live.Parse(output)
	if err != nil || parsed.QuerySelector(".final") == nil || parsed.GetElementById("label").TextContent() != "Mutation "+marker {
		return value, fmt.Errorf("rendered mutation structure is incorrect")
	}

	return value, nil
}

func measureDOMOperation(root *live.Element, operation string) (sample, error) {
	slot := root.QuerySelector(".slot")
	label := root.GetElementById("label")
	child := live.CreateElement("span")
	child.SetTextContent("child")
	if err := slot.SetInnerHTML(""); err != nil {
		return sample{}, err
	}
	if operation == "remove-child" || operation == "replace-child" || operation == "remove" || operation == "insert-before" {
		if err := slot.AppendChild(child); err != nil {
			return sample{}, err
		}
	}
	root.SetAttribute("data-probe", "before")

	var err error
	var valid bool
	var found *live.Element
	var matches []*live.Element
	var cloned *live.Element
	replacement := live.CreateElement("b")
	started := time.Now()
	switch operation {
	case "query-selector":
		found = root.QuerySelector(".detail")
	case "query-selector-all":
		matches = root.QuerySelectorAll(".detail")
	case "get-element-by-id":
		found = root.GetElementById("label")
	case "get-elements-by-tag-name":
		matches = root.GetElementsByTagName("h3")
	case "append-child":
		err = slot.AppendChild(child)
	case "insert-before":
		err = slot.InsertBefore(replacement, child)
	case "remove-child":
		err = slot.RemoveChild(child)
	case "replace-child":
		err = slot.ReplaceChild(replacement, child)
	case "remove":
		child.Remove()
	case "clone-node":
		cloned = root.CloneNode(true)
	case "text-content":
		label.SetTextContent("Updated")
	case "inner-html":
		err = slot.SetInnerHTML("<b>Updated</b>")
	case "set-attribute":
		root.SetAttribute("data-probe", "after")
	case "remove-attribute":
		root.RemoveAttribute("data-probe")
	case "class-list":
		err = root.ClassList().Add("active")
	case "class-name":
		root.SetClassName("component-card changed")
	default:
		return sample{}, fmt.Errorf("unknown operation %q", operation)
	}
	value := sample{
		latency:  time.Since(started),
		domCalls: 1,
	}
	if err != nil {
		return value, err
	}

	switch operation {
	case "query-selector", "get-element-by-id":
		valid = found != nil && root.Contains(found)
	case "query-selector-all", "get-elements-by-tag-name":
		valid = len(matches) == 1 && root.Contains(matches[0])
	case "append-child":
		valid = slot.Contains(child) && len(slot.Children()) == 1
	case "insert-before":
		valid = len(slot.Children()) == 2 && replacement.NextElementSibling() != nil && slot.FirstElementChild().Contains(replacement)
	case "remove-child", "remove":
		valid = child.ParentElement() == nil && len(slot.Children()) == 0
	case "replace-child":
		valid = child.ParentElement() == nil && len(slot.Children()) == 1 && slot.Contains(replacement)
	case "clone-node":
		cloned.SetAttribute("data-isolated", "yes")
		valid = cloned.ParentElement() == nil && !root.HasAttribute("data-isolated") && cloned.TextContent() == root.TextContent()
	case "text-content":
		valid = label.TextContent() == "Updated" && len(label.Children()) == 0
	case "inner-html":
		valid = slot.QuerySelector("b") != nil && slot.TextContent() == "Updated"
	case "set-attribute":
		attribute, exists := root.GetAttribute("data-probe")
		valid = exists && attribute == "after"
	case "remove-attribute":
		valid = !root.HasAttribute("data-probe")
	case "class-list":
		valid = root.ClassList().Contains("active")
	case "class-name":
		valid = root.ClassName() == "component-card changed"
	}
	root.SetClassName("component-card")
	if !valid {
		return value, fmt.Errorf("%s correctness assertion failed", operation)
	}

	return value, nil
}

func composedTree(ctx context.Context, pool []*live.Element, shape string) (*live.Element, error) {
	root := live.CreateElement("main")
	parent := root
	if shape == "deep" {
		for depth := 0; depth < min(len(pool), 256); depth++ {
			nested := live.CreateElement("div")
			if err := parent.AppendChild(nested); err != nil {
				return nil, err
			}
			parent = nested
		}
	}

	for index, component := range pool {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		component.SetAttribute("id", "item-"+strconv.Itoa(index))
		if shape == "small" {
			component.SetTextContent("Small component")
		} else if shape == "medium" {
			if err := component.QuerySelector(".slot").SetInnerHTML(strings.Repeat("<span>Cell</span>", 12)); err != nil {
				return nil, err
			}
		} else if shape == "mutated" {
			if _, err := mutateComponent(component, strconv.Itoa(index)); err != nil {
				return nil, err
			}
		}
		for _, descendant := range component.QuerySelectorAll("[id]") {
			id, _ := descendant.GetAttribute("id")
			descendant.SetAttribute("id", "item-"+strconv.Itoa(index)+"-"+id)
		}
		if err := parent.AppendChild(component); err != nil {
			return nil, err
		}
	}

	return root, nil
}

func composeComponent(ctx context.Context, original *live.Element, shape string) (sample, error) {
	started := time.Now()
	container := live.CreateElement("section")
	parent := container
	depth := 4
	if shape == "deep" {
		depth = 64
	}
	for index := 0; index < depth; index++ {
		if err := ctx.Err(); err != nil {
			return sample{}, err
		}
		next := live.CreateElement("div")
		if err := parent.AppendChild(next); err != nil {
			return sample{}, err
		}
		parent = next
	}
	clone := original.CloneNode(true)
	clone.GetElementById("label").SetTextContent("Isolated clone")
	if err := parent.AppendChild(clone); err != nil {
		return sample{}, err
	}
	sibling := clone.CloneNode(true)
	if err := parent.AppendChild(sibling); err != nil {
		return sample{}, err
	}
	sibling.Remove()

	renderStarted := time.Now()
	output := container.Render()
	value := sample{
		latency:       time.Since(started),
		renderLatency: time.Since(renderStarted),
		renders:       1,
		domCalls:      uint64(depth + 7),
		outputBytes:   uint64(len(output)),
	}
	if original.GetElementById("label").TextContent() != "Template" ||
		clone.ParentElement() == nil || sibling.ParentElement() != nil || !container.Contains(clone) {
		return value, fmt.Errorf("composition changed its source or lost parent relationships")
	}
	parsed, err := live.Parse(output)
	if err != nil || parsed.GetElementById("label").TextContent() != "Isolated clone" || len(parsed.QuerySelectorAll("article")) != 1 {
		return value, fmt.Errorf("composed output structure is incorrect")
	}

	return value, nil
}

func queryTree(root *live.Element, index, count int) (sample, error) {
	id := "item-" + strconv.Itoa(index)
	started := time.Now()
	first := root.QuerySelector("#" + id)
	byID := root.GetElementById(id)
	all := root.QuerySelectorAll("article")
	tags := root.GetElementsByTagName("article")
	value := sample{
		latency:  time.Since(started),
		domCalls: 4,
	}
	if first == nil || byID == nil || !first.Contains(byID) || len(all) != count || len(tags) != count {
		return value, fmt.Errorf("selector lookup count or identity mismatch")
	}
	for position, match := range all {
		actual, _ := match.GetAttribute("id")
		if actual != "item-"+strconv.Itoa(position) {
			return value, fmt.Errorf("selector results are not in document order")
		}
	}

	return value, nil
}

func renderTree(root *live.Element, count int) (sample, error) {
	started := time.Now()
	output := root.Render()
	elapsed := time.Since(started)
	value := sample{
		latency:       elapsed,
		renderLatency: elapsed,
		renders:       1,
		outputBytes:   uint64(len(output)),
	}
	parsed, err := live.Parse(output)
	if err != nil || len(parsed.QuerySelectorAll("article")) != count {
		return value, fmt.Errorf("rendered component count differs from source")
	}
	if count > 0 && parsed.GetElementById("item-"+strconv.Itoa(count-1)) == nil {
		return value, fmt.Errorf("last component missing from rendered output")
	}
	if parsed.QuerySelector("main").TextContent() != root.TextContent() {
		return value, fmt.Errorf("rendered text differs from the source component tree")
	}

	return value, nil
}

func requestWorkload(ctx context.Context, c config, pool []*live.Element, nodes int) (*workload, error) {
	// Measure the kit route without rate limits or buffering timeout middleware.
	router := kit.New([]kit.Middleware{}...)
	router.Get("/", func(h kit.Handle) {
		index, err := strconv.Atoi(h.Query("component"))
		if err != nil || index < 0 || index >= len(pool) {
			h.Error.BadRequest("invalid component")
			return
		}

		root := pool[0]
		if c.Scenario == "independent" {
			root = pool[index].CloneNode(true)
			root.GetElementById("label").SetTextContent("Mutation " + h.Query("marker"))
			root.SetAttribute("data-request", h.Query("marker"))
			slot := root.QuerySelector(".slot")
			child := live.CreateElement("span")
			child.SetTextContent("Temporary")
			if err := slot.AppendChild(child); err != nil {
				h.Error.Handle(err)
				return
			}
			if err := slot.RemoveChild(child); err != nil {
				h.Error.Handle(err)
				return
			}
			if err := root.ClassList().Add("active"); err != nil {
				h.Error.Handle(err)
				return
			}
		}
		started := time.Now()
		output := root.Render()
		h.Writer().Header().Set("X-Render-Nanoseconds", strconv.FormatInt(int64(time.Since(started)), 10))
		h.HTML(http.StatusOK, output)
	})
	server := httptest.NewServer(router)
	transport := &http.Transport{
		MaxIdleConns:        min(c.Concurrency, c.Components),
		MaxIdleConnsPerHost: min(c.Concurrency, c.Components),
		MaxConnsPerHost:     min(c.Concurrency, c.Components),
		IdleConnTimeout:     5 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	work := &workload{
		instances: len(pool),
		nodes:     nodes,
		unit:      "requests",
	}
	work.close = func() {
		transport.CloseIdleConnections()
		server.Close()
	}
	work.step = func(ctx context.Context, component, iteration int) (sample, error) {
		marker := strconv.Itoa(component) + "-" + strconv.Itoa(iteration)
		url := server.URL + "/?component=" + strconv.Itoa(component) + "&marker=" + marker
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return sample{}, err
		}
		started := time.Now()
		response, err := client.Do(request)
		if err != nil {
			return sample{requests: 1}, err
		}
		defer response.Body.Close()

		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		value := sample{
			latency:     time.Since(started),
			requests:    1,
			renders:     1,
			outputBytes: uint64(len(body)),
		}
		renderNS, parseError := strconv.ParseInt(response.Header.Get("X-Render-Nanoseconds"), 10, 64)
		value.renderLatency = time.Duration(renderNS)
		if err != nil || response.StatusCode != http.StatusOK || parseError != nil {
			return value, fmt.Errorf("HTTP render failed: status %d, body error %v", response.StatusCode, err)
		}
		page, err := live.Parse(string(body))
		if err != nil {
			return value, err
		}
		expected := "Template"
		if c.Scenario == "independent" {
			expected = "Mutation " + marker
			value.domCalls = 9
		}
		if page.GetElementById("label").TextContent() != expected || pool[component].GetElementById("label").TextContent() != "Template" {
			return value, fmt.Errorf("HTTP request instance isolation failed")
		}

		return value, nil
	}

	return work, nil
}
