package component

import (
	"github.com/renniemaharaj/live-component/internal/document"
	"github.com/renniemaharaj/live-component/internal/element"

	"golang.org/x/net/html"
)

// Component owns a parsed HTML page. Use Document for DOM operations.
type Component struct {
	// Doc is the underlying parser tree, retained for compatibility.
	// Prefer Document so callers do not depend on parser internals.
	Doc *html.Node
}

// Document returns a view of the component's existing document, without cloning.
func (c *Component) Document() *document.Document {
	if c == nil || c.Doc == nil {
		return nil
	}

	return document.New(c.Doc)
}

// QuerySelector returns the first matching descendant, or nil for no match or invalid CSS.
func (c *Component) QuerySelector(selector string) *element.Element {
	return c.Document().QuerySelector(selector)
}

// QuerySelectorAll returns a snapshot in document order, or nil for invalid CSS.
func (c *Component) QuerySelectorAll(selector string) []*element.Element {
	return c.Document().QuerySelectorAll(selector)
}

// GetElementById matches an exact ID, without CSS escaping.
func (c *Component) GetElementById(id string) *element.Element {
	return c.Document().GetElementById(id)
}

// GetElementsByClassName returns descendants containing every requested class.
func (c *Component) GetElementsByClassName(names string) []*element.Element {
	return c.Document().GetElementsByClassName(names)
}

// GetElementsByTagName returns matching descendants; "*" matches every element.
func (c *Component) GetElementsByTagName(tag string) []*element.Element {
	return c.Document().GetElementsByTagName(tag)
}

// Render serializes the complete document.
func (c *Component) Render() string {
	if c == nil {
		return ""
	}

	return element.RenderNode(c.Doc)
}

// Root is a shortcut for Document().DocumentElement().
func (c *Component) Root() *element.Element {
	return c.Document().DocumentElement()
}
