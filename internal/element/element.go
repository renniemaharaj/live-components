package element

import (
	"fmt"
	"strings"

	"github.com/renniemaharaj/live-components/internal/classlist"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Element is a reusable reference to an HTML element or text node.
type Element struct {
	// Node is the underlying parser node, retained for compatibility.
	// Prefer DOM methods so callers do not depend on parser internals.
	Node *html.Node
}

func CreateElement(tag string) *Element {
	tag = lowerASCII(tag)

	return &Element{Node: &html.Node{
		Type:     html.ElementNode,
		Data:     tag,
		DataAtom: atom.Lookup([]byte(tag)),
	}}
}

func CreateTextNode(text string) *Element {
	return &Element{Node: &html.Node{Type: html.TextNode, Data: text}}
}

// TextContent returns descendant text, excluding comments and markup.
func (e *Element) TextContent() string {
	if e == nil || e.Node == nil {
		return ""
	}
	if e.Node.Type == html.TextNode {
		return e.Node.Data
	}

	var text strings.Builder
	for node := range e.Node.Descendants() {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
	}

	return text.String()
}

// SetTextContent replaces all children with text, or updates a text node's data.
func (e *Element) SetTextContent(text string) {
	if e == nil || e.Node == nil {
		return
	}
	if e.Node.Type == html.TextNode {
		e.Node.Data = text
		return
	}
	if e.Node.Type != html.ElementNode {
		return
	}

	clearChildren(e.Node)
	if text != "" {
		e.Node.AppendChild(CreateTextNode(text).Node)
	}
}

// InnerHTML serializes the node's children.
func (e *Element) InnerHTML() string {
	if e == nil || e.Node == nil {
		return ""
	}

	var markup strings.Builder
	for node := e.Node.FirstChild; node != nil; node = node.NextSibling {
		_ = html.Render(&markup, node)
	}

	return markup.String()
}

// SetInnerHTML parses markup in this element's context and replaces its children.
func (e *Element) SetInnerHTML(markup string) error {
	if e == nil || e.Node == nil || e.Node.Type != html.ElementNode {
		return fmt.Errorf("receiver is not an element")
	}

	nodes, err := html.ParseFragment(strings.NewReader(markup), e.Node)
	if err != nil {
		return err
	}

	clearChildren(e.Node)
	for _, node := range nodes {
		e.Node.AppendChild(node)
	}

	return nil
}

// GetAttribute distinguishes a missing attribute from an empty value.
func (e *Element) GetAttribute(name string) (string, bool) {
	if e == nil || e.Node == nil || e.Node.Type != html.ElementNode {
		return "", false
	}

	name = e.attributeName(name)
	for _, attribute := range e.Node.Attr {
		if attribute.Namespace == "" && attribute.Key == name {
			return attribute.Val, true
		}
	}

	return "", false
}

// HasAttribute reports whether the attribute exists, including empty attributes.
func (e *Element) HasAttribute(name string) bool {
	_, exists := e.GetAttribute(name)
	return exists
}

// SetAttribute sets an attribute, folding HTML attribute names to lowercase.
func (e *Element) SetAttribute(name, value string) {
	if e == nil || e.Node == nil || e.Node.Type != html.ElementNode {
		return
	}

	name = e.attributeName(name)
	for i := range e.Node.Attr {
		if e.Node.Attr[i].Namespace == "" && e.Node.Attr[i].Key == name {
			e.Node.Attr[i].Val = value
			return
		}
	}

	e.Node.Attr = append(e.Node.Attr, html.Attribute{Key: name, Val: value})
}

// RemoveAttribute removes an attribute if present.
func (e *Element) RemoveAttribute(name string) {
	if e == nil || e.Node == nil || e.Node.Type != html.ElementNode {
		return
	}

	name = e.attributeName(name)
	for i, attribute := range e.Node.Attr {
		if attribute.Namespace == "" && attribute.Key == name {
			e.Node.Attr = append(e.Node.Attr[:i], e.Node.Attr[i+1:]...)
			return
		}
	}
}

func (e *Element) attributeName(name string) string {
	if e.Node.Namespace == "" {
		return lowerASCII(name)
	}

	return name
}

// ClassName returns the class attribute as written.
func (e *Element) ClassName() string {
	value, _ := e.GetAttribute("class")
	return value
}

// SetClassName replaces the class attribute.
func (e *Element) SetClassName(value string) {
	e.SetAttribute("class", value)
}

// ClassList returns a view of the same attribute used by ClassName.
func (e *Element) ClassList() *classlist.ClassList {
	if e == nil {
		return &classlist.ClassList{}
	}

	return &classlist.ClassList{Node: e.Node}
}

// OuterHTML serializes this node and its descendants.
func (e *Element) OuterHTML() string {
	if e == nil {
		return ""
	}

	return RenderNode(e.Node)
}

// Render is a compatibility alias for OuterHTML.
func (e *Element) Render() string {
	return e.OuterHTML()
}

func RenderNode(node *html.Node) string {
	if node == nil {
		return ""
	}

	var markup strings.Builder
	_ = html.Render(&markup, node)

	return markup.String()
}

func clearChildren(node *html.Node) {
	for node.FirstChild != nil {
		node.RemoveChild(node.FirstChild)
	}
}

func lowerASCII(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}

		return character
	}, value)
}
