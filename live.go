package live

import (
	"os"
	"strings"

	"github.com/renniemaharaj/live-components/internal/classlist"
	"github.com/renniemaharaj/live-components/internal/component"
	"github.com/renniemaharaj/live-components/internal/document"
	"github.com/renniemaharaj/live-components/internal/element"

	"golang.org/x/net/html"
)

// Component owns a parsed HTML page and provides document-level shortcuts.
type Component = component.Component

// Document provides DOM queries and node creation for a component.
type Document = document.Document

// Element is a reusable reference to an HTML element or text node.
type Element = element.Element

// ClassList reads and updates an element's class attribute.
type ClassList = classlist.ClassList

// CreateElement creates a detached HTML element.
func CreateElement(tag string) *Element {
	return element.CreateElement(tag)
}

// CreateTextNode creates a detached text node. Text is escaped when rendered.
func CreateTextNode(text string) *Element {
	return element.CreateTextNode(text)
}

// Load reads and parses an HTML file.
func Load(path string) (*Component, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(string(data))
}

// Parse parses a complete HTML document, inserting html, head, and body as needed.
func Parse(markup string) (*Component, error) {
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return nil, err
	}

	return &component.Component{Doc: doc}, nil
}
