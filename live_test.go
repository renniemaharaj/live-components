package live_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	live "github.com/renniemaharaj/live-components"
)

type domQueries interface {
	GetElementById(string) *live.Element
	GetElementsByClassName(string) []*live.Element
	GetElementsByTagName(string) []*live.Element
	QuerySelector(string) *live.Element
	QuerySelectorAll(string) []*live.Element
}

func parsePage(t *testing.T, markup string) *live.Component {
	t.Helper()

	page, err := live.Parse(markup)
	if err != nil {
		t.Fatal(err)
	}

	return page
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func ids(elements []*live.Element) string {
	var values []string
	for _, element := range elements {
		value, _ := element.GetAttribute("id")
		values = append(values, value)
	}

	return strings.Join(values, ",")
}

func TestDocumentWorkflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.html")
	requireNoError(t, os.WriteFile(path, []byte(`<title id="title">Old</title><div class="banner"></div>`), 0600))

	page, err := live.Load(path)
	requireNoError(t, err)

	var document *live.Document = page.Document()
	title := document.GetElementById("title")
	title.SetTextContent("Hello World")
	banner := document.QuerySelector(".banner")
	var classes *live.ClassList = banner.ClassList()
	requireNoError(t, classes.Add("active"))

	button := document.CreateElement("button")
	button.SetTextContent("Click me")
	requireNoError(t, banner.AppendChild(button))
	button.SetTextContent("Updated")

	output := page.Render()
	if !strings.Contains(output, `<title id="title">Hello World</title>`) ||
		!strings.Contains(output, `<div class="banner active"><button>Updated</button></div>`) {
		t.Fatalf("unexpected rendered document: %s", output)
	}
	if page.Root().Node != document.DocumentElement().Node {
		t.Fatal("Root and DocumentElement must reference the same node")
	}
	if page.Root().ParentElement() != nil {
		t.Fatal("the document is not an element parent")
	}
	if _, err := live.Load(filepath.Join(t.TempDir(), "missing.html")); !os.IsNotExist(err) {
		t.Fatalf("Load must preserve the file error: %v", err)
	}
}

func TestQueriesAreConsistentAcrossScopes(t *testing.T) {
	page := parsePage(t, `<main id="scope" class="item"><section id="a" class="item selected"><b id="nested" class="item"></b></section><section id="b" class="item"></section><section id="a" class="item selected"></section></main>`)
	queries := []struct {
		name  string
		scope domQueries
	}{
		{name: "component", scope: page},
		{name: "document", scope: page.Document()},
		{name: "element", scope: page.GetElementById("scope")},
	}

	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			if got := ids(query.scope.QuerySelectorAll("section, b")); got != "a,nested,b,a" {
				t.Fatalf("selector document order: %q", got)
			}
			if got := query.scope.QuerySelector("section, b"); got.Node != page.GetElementById("a").Node {
				t.Fatal("QuerySelector did not return the first matching descendant")
			}
			if got := ids(query.scope.GetElementsByTagName("SECTION")); got != "a,b,a" {
				t.Fatalf("literal case-insensitive tags: %q", got)
			}
			if got := ids(query.scope.GetElementsByClassName(" selected\titem item\n")); got != "a,a" {
				t.Fatalf("all class tokens must match: %q", got)
			}
			if query.scope.GetElementsByTagName("section, b") != nil {
				t.Fatal("tag lookup must not interpret CSS")
			}
			for _, selector := range []string{"[", ".missing", ""} {
				if query.scope.QuerySelector(selector) != nil || len(query.scope.QuerySelectorAll(selector)) != 0 {
					t.Fatalf("expected no results for %q", selector)
				}
			}
			if query.scope.GetElementById("missing") != nil || query.scope.GetElementsByClassName(" \t\n") != nil {
				t.Fatal("missing IDs and empty class lists must not match")
			}
		})
	}

	root := page.GetElementById("scope")
	if root.QuerySelector("#scope") != nil || root.GetElementById("scope") != nil {
		t.Fatal("element queries must exclude the receiver")
	}
	if got := ids(root.GetElementsByTagName("*")); got != "a,nested,b,a" {
		t.Fatalf("wildcard must match descendants only: %q", got)
	}
}

func TestLiteralIDsAndClassNames(t *testing.T) {
	page := parsePage(t, `<main><p id="a:b.c[0]" class="a:b.c first"></p><p id="123 spaced" class="a:b.c"></p><p id=""></p></main>`)
	for _, scope := range []domQueries{page, page.Document(), page.QuerySelector("main")} {
		for _, id := range []string{"a:b.c[0]", "123 spaced"} {
			if found := scope.GetElementById(id); found == nil {
				t.Fatalf("literal ID %q was treated as CSS", id)
			}
		}
		if scope.GetElementById("") != nil {
			t.Fatal("empty IDs must not match")
		}
		if len(scope.GetElementsByClassName("a:b.c")) != 2 {
			t.Fatal("class lookup must not interpret CSS")
		}
	}
}

func TestCollectionsAreSnapshotsOfReusableReferences(t *testing.T) {
	page := parsePage(t, `<main><p id="first" class="item">First</p><p id="second" class="item">Second</p></main>`)
	root := page.QuerySelector("main")
	collections := [][]*live.Element{
		root.QuerySelectorAll(".item"),
		root.GetElementsByTagName("p"),
		root.GetElementsByClassName("item"),
		root.Children(),
	}

	first := root.FirstElementChild()
	first.Remove()
	requireNoError(t, root.AppendChild(live.CreateElement("p")))
	for _, snapshot := range collections {
		if len(snapshot) != 2 || snapshot[0].Node != first.Node {
			t.Fatal("mutating the tree changed snapshot membership")
		}
		snapshot[1].SetTextContent("Changed")
	}
	if root.QuerySelector("#second").TextContent() != "Changed" {
		t.Fatal("snapshot elements must still reference the underlying nodes")
	}
}

func TestElementTraversalSkipsTextAndComments(t *testing.T) {
	page := parsePage(t, `<main>leading<!-- comment --><p id="first"><b id="nested"></b></p>between<i id="last"></i>trailing</main>`)
	root := page.QuerySelector("main")
	first := root.FirstElementChild()
	last := root.LastElementChild()
	if got := ids(root.Children()); got != "first,last" {
		t.Fatalf("Children must include direct elements only: %q", got)
	}
	if first.NextElementSibling().Node != last.Node || last.PreviousElementSibling().Node != first.Node {
		t.Fatal("sibling traversal must skip text and comments")
	}
	if first.PreviousElementSibling() != nil || last.NextElementSibling() != nil {
		t.Fatal("sibling traversal must stop at the parent boundary")
	}
	if first.ParentElement().Node != root.Node || last.FirstElementChild() != nil || last.LastElementChild() != nil {
		t.Fatal("unexpected parent or empty-child traversal")
	}
	if !root.Contains(root) || !root.Contains(page.GetElementById("nested")) || first.Contains(last) || root.Contains(nil) {
		t.Fatal("Contains must include self and all descendants only")
	}
}

func TestMoveInsertPrependAndReplace(t *testing.T) {
	page := parsePage(t, `<main><div id="left">lead<b id="a"></b><!-- middle --><b id="b"></b></div><div id="right"><b id="c"></b></div></main>`)
	left := page.GetElementById("left")
	right := page.GetElementById("right")
	a := page.GetElementById("a")
	b := page.GetElementById("b")
	c := page.GetElementById("c")

	requireNoError(t, right.AppendChild(a))
	if ids(left.Children()) != "b" || ids(right.Children()) != "c,a" || a.ParentElement().Node != right.Node {
		t.Fatal("AppendChild must move the original node between parents")
	}
	requireNoError(t, right.Prepend(a))
	if ids(right.Children()) != "a,c" {
		t.Fatal("Prepend must move an existing child to the front")
	}
	requireNoError(t, left.Prepend(a))
	if !strings.HasPrefix(left.InnerHTML(), `<b id="a"></b>lead`) {
		t.Fatal("Prepend must insert before text as well as elements")
	}
	requireNoError(t, left.InsertBefore(b, a))
	if ids(left.Children()) != "b,a" {
		t.Fatal("InsertBefore failed to reorder siblings")
	}
	requireNoError(t, left.InsertBefore(b, nil))
	if ids(left.Children()) != "a,b" {
		t.Fatal("InsertBefore with nil must append")
	}
	requireNoError(t, left.ReplaceChild(c, a))
	if a.ParentElement() != nil || ids(left.Children()) != "c,b" || len(right.Children()) != 0 {
		t.Fatal("ReplaceChild must move the replacement and detach the old child")
	}

	before := left.OuterHTML()
	alias := left.QuerySelector("#c")
	requireNoError(t, left.ReplaceChild(alias, c))
	requireNoError(t, left.InsertBefore(alias, c))
	if left.OuterHTML() != before {
		t.Fatal("replacing or inserting the same underlying node must be a no-op")
	}
	requireNoError(t, left.RemoveChild(c))
	if c.ParentElement() != nil || left.Contains(c) {
		t.Fatal("RemoveChild must detach without destroying the reusable node")
	}
	c.Remove()
	c.Remove()
}

func TestFailedMutationsLeaveTreesUnchanged(t *testing.T) {
	page := parsePage(t, `<main><div id="parent"><p id="child"><b id="nested"></b></p></div><div id="other"></div></main>`)
	parent := page.GetElementById("parent")
	child := page.GetElementById("child")
	nested := page.GetElementById("nested")
	other := page.GetElementById("other")
	operations := []struct {
		name string
		run  func() error
	}{
		{name: "self cycle", run: func() error { return parent.AppendChild(parent) }},
		{name: "ancestor cycle", run: func() error { return nested.AppendChild(parent) }},
		{name: "foreign reference", run: func() error { return other.InsertBefore(child, nested) }},
		{name: "zero reference", run: func() error { return other.InsertBefore(child, &live.Element{}) }},
		{name: "foreign replacement", run: func() error { return other.ReplaceChild(child, nested) }},
		{name: "replacement cycle", run: func() error { return parent.ReplaceChild(parent, child) }},
		{name: "foreign removal", run: func() error { return other.RemoveChild(nested) }},
		{name: "nil child", run: func() error { return parent.AppendChild(nil) }},
		{name: "text parent", run: func() error { return live.CreateTextNode("text").AppendChild(child) }},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			before := page.Render()
			if err := operation.run(); err == nil {
				t.Fatal("expected a mutation error")
			}
			if page.Render() != before {
				t.Fatal("failed operation changed the tree")
			}
		})
	}
}

func TestReplaceChildWithDescendant(t *testing.T) {
	page := parsePage(t, `<main><p id="old"><b id="new">Kept</b></p><i id="last"></i></main>`)
	parent := page.QuerySelector("main")
	old := page.GetElementById("old")
	replacement := page.GetElementById("new")
	requireNoError(t, parent.ReplaceChild(replacement, old))
	if ids(parent.Children()) != "new,last" || replacement.TextContent() != "Kept" || old.Contains(replacement) {
		t.Fatal("replacement must be detached from the removed subtree")
	}
}

func TestCloneNodeCopiesAttributesAndDescendants(t *testing.T) {
	page := parsePage(t, `<main><p id="original" class="card">before<b>nested</b><!--comment-->after</p></main>`)
	original := page.GetElementById("original")
	shallow := original.CloneNode(false)
	deep := original.CloneNode(true)
	if shallow.ParentElement() != nil || deep.ParentElement() != nil || shallow.InnerHTML() != "" {
		t.Fatal("clones must be detached and shallow clones must have no children")
	}
	if shallow.ClassName() != "card" || deep.OuterHTML() != original.OuterHTML() {
		t.Fatal("clones must copy attributes and deep clones must include all node types")
	}

	deep.SetClassName("changed")
	deep.QuerySelector("b").SetTextContent("Changed")
	if original.ClassName() != "card" || original.QuerySelector("b").TextContent() != "nested" {
		t.Fatal("clone mutations must not affect the original")
	}
	text := live.CreateTextNode("original")
	textClone := text.CloneNode(true)
	textClone.SetTextContent("copy")
	if text.TextContent() != "original" || textClone.TextContent() != "copy" {
		t.Fatal("text clones must copy and independently update their data")
	}
}

func TestTextAndHTMLReplacement(t *testing.T) {
	for _, tag := range []string{"div", "custom-card", "table", "select"} {
		t.Run(tag, func(t *testing.T) {
			element := live.CreateElement(strings.ToUpper(tag))
			markup := `<b>child</b>`
			selector := "b"
			switch tag {
			case "table":
				markup = `<tr><td>child</td></tr>`
				selector = "tbody > tr > td"
			case "select":
				markup = `<option>child</option>`
				selector = "option"
			}

			requireNoError(t, element.SetInnerHTML(markup))
			child := element.QuerySelector(selector)
			if child == nil || child.TextContent() != "child" {
				t.Fatalf("HTML parsing did not use the %s context: %s", tag, element.InnerHTML())
			}

			element.SetTextContent(`<new>&text`)
			if element.TextContent() != `<new>&text` || element.InnerHTML() != `&lt;new&gt;&amp;text` || child.ParentElement() != nil && element.Contains(child) {
				t.Fatal("SetTextContent must replace children with escaped text")
			}
			element.SetTextContent("")
			if element.Node.FirstChild != nil {
				t.Fatal("empty textContent must remove all children")
			}
		})
	}

	page := parsePage(t, `<main>first<!--ignored--><b>nested</b>last</main>`)
	root := page.QuerySelector("main")
	old := root.QuerySelector("b")
	if root.TextContent() != "firstnestedlast" {
		t.Fatal("TextContent must skip comments and include nested text")
	}
	requireNoError(t, root.SetInnerHTML(`<i>replacement</i>`))
	if old.ParentElement() != nil || root.InnerHTML() != `<i>replacement</i>` || root.OuterHTML() != `<main><i>replacement</i></main>` {
		t.Fatal("SetInnerHTML must detach the previous children")
	}
	if root.Render() != root.OuterHTML() {
		t.Fatal("Render must remain an alias for OuterHTML")
	}
	text := live.CreateTextNode("old")
	text.SetTextContent("<updated>")
	if text.OuterHTML() != "&lt;updated&gt;" || text.Node.FirstChild != nil {
		t.Fatal("SetTextContent on text nodes must update data")
	}
	if err := text.SetInnerHTML("invalid"); err == nil {
		t.Fatal("SetInnerHTML requires an element")
	}
}

func TestAttributesAndClassListShareState(t *testing.T) {
	element := live.CreateElement("div")
	element.SetAttribute("DATA-VALUE", "")
	if value, exists := element.GetAttribute("data-value"); value != "" || !exists || !element.HasAttribute("DATA-VALUE") {
		t.Fatal("empty attributes must exist and HTML names must be case-insensitive")
	}
	element.SetAttribute("data-value", "updated")
	element.RemoveAttribute("DATA-VALUE")
	if _, exists := element.GetAttribute("data-value"); exists || element.HasAttribute("data-value") {
		t.Fatal("RemoveAttribute must remove the same underlying attribute")
	}

	classes := element.ClassList()
	element.SetClassName("first first\tsecond")
	if !classes.Contains("second") {
		t.Fatal("an existing ClassList must observe SetClassName")
	}
	requireNoError(t, classes.Add("second", "third", "third"))
	if element.ClassName() != "first second third" {
		t.Fatalf("class tokens must be unique and ordered: %q", element.ClassName())
	}
	requireNoError(t, classes.Remove("second", "missing"))
	if value, _ := element.GetAttribute("class"); value != "first third" {
		t.Fatalf("ClassList must update the class attribute: %q", value)
	}

	present, err := classes.Toggle("first")
	requireNoError(t, err)
	if present || element.ClassName() != "third" {
		t.Fatal("Toggle must return false after removal")
	}
	present, err = classes.Toggle("first")
	requireNoError(t, err)
	if !present || element.ClassName() != "third first" {
		t.Fatal("Toggle must return true after adding")
	}
	element.SetAttribute("class", "external")
	if !classes.Contains("external") || classes.Contains("first") {
		t.Fatal("ClassList must observe direct attribute updates")
	}
	element.RemoveAttribute("class")
	if classes.Contains("external") || element.ClassName() != "" {
		t.Fatal("ClassList must observe attribute removal")
	}
}

func TestClassTokensValidateAtomicallyAndUseHTMLWhitespace(t *testing.T) {
	element := live.CreateElement("div")
	element.SetClassName("original")
	classes := element.ClassList()
	for _, invalid := range []string{"", "two words", "tab\tword", "line\nword", "return\rword", "form\fword"} {
		if err := classes.Add("valid", invalid); err == nil || element.ClassName() != "original" {
			t.Fatalf("Add must reject %q before any mutation", invalid)
		}
		if err := classes.Remove("original", invalid); err == nil || element.ClassName() != "original" {
			t.Fatalf("Remove must reject %q before any mutation", invalid)
		}
		if _, err := classes.Toggle(invalid); err == nil || element.ClassName() != "original" {
			t.Fatalf("Toggle must reject %q before any mutation", invalid)
		}
	}

	// A nonbreaking space is part of a class token, not HTML whitespace.
	token := "with\u00a0space"
	requireNoError(t, classes.Add(token))
	if !classes.Contains(token) || classes.Contains("with") {
		t.Fatal("class tokenization must use HTML whitespace rather than Unicode whitespace")
	}
	page := parsePage(t, `<main></main>`)
	requireNoError(t, page.QuerySelector("main").AppendChild(element))
	if len(page.GetElementsByClassName(token)) != 1 {
		t.Fatal("class lookups and ClassList must use the same token rules")
	}
}

func TestNilAndZeroValues(t *testing.T) {
	for _, element := range []*live.Element{nil, {}} {
		if element.TextContent() != "" || element.InnerHTML() != "" || element.OuterHTML() != "" || element.ClassName() != "" {
			t.Fatal("empty elements must have empty content")
		}
		if element.ParentElement() != nil || element.FirstElementChild() != nil || element.LastElementChild() != nil ||
			element.NextElementSibling() != nil || element.PreviousElementSibling() != nil || len(element.Children()) != 0 || element.Contains(element) {
			t.Fatal("empty elements must not have tree relations")
		}
		if element.QuerySelector("*") != nil || len(element.QuerySelectorAll("*")) != 0 || element.GetElementById("id") != nil ||
			len(element.GetElementsByTagName("*")) != 0 || len(element.GetElementsByClassName("class")) != 0 || element.CloneNode(true) != nil {
			t.Fatal("empty elements must not produce query results or clones")
		}

		element.SetTextContent("no-op")
		element.SetClassName("no-op")
		element.SetAttribute("name", "value")
		element.RemoveAttribute("name")
		element.Remove()
		requireNoError(t, element.ClassList().Add("safe"))
		if element.HasAttribute("name") || element.ClassList().Contains("safe") {
			t.Fatal("empty elements must not gain attributes")
		}
		child := live.CreateElement("p")
		if element.AppendChild(child) == nil || element.Prepend(child) == nil || element.InsertBefore(child, nil) == nil ||
			element.RemoveChild(child) == nil || element.ReplaceChild(child, child) == nil || element.SetInnerHTML("text") == nil {
			t.Fatal("invalid structural mutations must return errors")
		}
	}

	for _, component := range []*live.Component{nil, {}} {
		if component.Document() != nil || component.Root() != nil || component.Render() != "" || component.QuerySelector("*") != nil ||
			len(component.QuerySelectorAll("*")) != 0 || component.GetElementById("id") != nil ||
			len(component.GetElementsByTagName("*")) != 0 || len(component.GetElementsByClassName("class")) != 0 {
			t.Fatal("empty components must be safe to query")
		}
	}
	for _, document := range []*live.Document{nil, {}} {
		if document.DocumentElement() != nil || document.QuerySelector("*") != nil || len(document.QuerySelectorAll("*")) != 0 ||
			document.GetElementById("id") != nil || len(document.GetElementsByTagName("*")) != 0 || len(document.GetElementsByClassName("class")) != 0 {
			t.Fatal("empty documents must be safe to query")
		}
	}
}
