package document

import (
	"github.com/renniemaharaj/live-components/internal/element"

	"golang.org/x/net/html"
)

// Document provides queries and node creation over a parsed HTML document.
type Document struct {
	root *element.Element
}

func New(node *html.Node) *Document {
	return &Document{root: &element.Element{Node: node}}
}

func (d *Document) rootElement() *element.Element {
	if d == nil {
		return nil
	}

	return d.root
}

// DocumentElement returns the document's html element.
func (d *Document) DocumentElement() *element.Element {
	return d.rootElement().FirstElementChild()
}

// QuerySelector returns the first matching descendant, or nil for no match or invalid CSS.
func (d *Document) QuerySelector(selector string) *element.Element {
	return d.rootElement().QuerySelector(selector)
}

// QuerySelectorAll returns a snapshot in document order, or nil for invalid CSS.
func (d *Document) QuerySelectorAll(selector string) []*element.Element {
	return d.rootElement().QuerySelectorAll(selector)
}

// GetElementById matches an exact ID, without CSS escaping.
func (d *Document) GetElementById(id string) *element.Element {
	return d.rootElement().GetElementById(id)
}

// GetElementsByClassName returns descendants containing every requested class.
func (d *Document) GetElementsByClassName(names string) []*element.Element {
	return d.rootElement().GetElementsByClassName(names)
}

// GetElementsByTagName returns matching descendants; "*" matches every element.
func (d *Document) GetElementsByTagName(tag string) []*element.Element {
	return d.rootElement().GetElementsByTagName(tag)
}

// CreateElement creates a detached HTML element.
func (d *Document) CreateElement(tag string) *element.Element {
	return element.CreateElement(tag)
}

// CreateTextNode creates a detached text node. Text is escaped when rendered.
func (d *Document) CreateTextNode(text string) *element.Element {
	return element.CreateTextNode(text)
}
