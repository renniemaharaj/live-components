package element

import (
	"fmt"

	"golang.org/x/net/html"
)

// AppendChild moves child to the end of this element's children.
func (e *Element) AppendChild(child *Element) error {
	return insert(e, child, nil)
}

// Prepend moves child before all existing children, including text and comments.
func (e *Element) Prepend(child *Element) error {
	if e == nil || e.Node == nil {
		return fmt.Errorf("nil parent")
	}

	var reference *Element
	if e.Node.FirstChild != nil {
		reference = &Element{Node: e.Node.FirstChild}
	}

	return insert(e, child, reference)
}

// InsertBefore moves child before reference. A nil reference appends instead.
func (e *Element) InsertBefore(child, reference *Element) error {
	return insert(e, child, reference)
}

func insert(parent, child, reference *Element) error {
	if parent == nil || child == nil || parent.Node == nil || child.Node == nil {
		return fmt.Errorf("nil node")
	}
	if parent.Node.Type != html.ElementNode {
		return fmt.Errorf("parent is not an element")
	}
	if child.Node.Type != html.ElementNode && child.Node.Type != html.TextNode && child.Node.Type != html.CommentNode {
		return fmt.Errorf("child must be an element, text, or comment node")
	}
	if reference != nil && (reference.Node == nil || reference.Node.Parent != parent.Node) {
		return fmt.Errorf("reference is not a child of parent")
	}
	if child.Contains(parent) {
		return fmt.Errorf("cannot insert an ancestor into its descendant")
	}
	if reference != nil && reference.Node == child.Node {
		return nil
	}

	// Validate before detaching so a failed move leaves the original tree intact.
	if child.Node.Parent != nil {
		child.Node.Parent.RemoveChild(child.Node)
	}
	if reference == nil {
		parent.Node.AppendChild(child.Node)
	} else {
		parent.Node.InsertBefore(child.Node, reference.Node)
	}

	return nil
}

// RemoveChild detaches a direct child, or returns an error without changing the tree.
func (e *Element) RemoveChild(child *Element) error {
	if e == nil || e.Node == nil || child == nil || child.Node == nil || child.Node.Parent != e.Node {
		return fmt.Errorf("not a child")
	}

	e.Node.RemoveChild(child.Node)
	return nil
}

// Remove detaches this node. Removing an already detached node does nothing.
func (e *Element) Remove() {
	if e != nil && e.Node != nil && e.Node.Parent != nil {
		e.Node.Parent.RemoveChild(e.Node)
	}
}

// ReplaceChild moves newChild into oldChild's position and detaches oldChild.
func (e *Element) ReplaceChild(newChild, oldChild *Element) error {
	if e == nil || e.Node == nil || oldChild == nil || oldChild.Node == nil || oldChild.Node.Parent != e.Node {
		return fmt.Errorf("old child is not a child")
	}
	if err := e.InsertBefore(newChild, oldChild); err != nil {
		return err
	}
	if newChild.Node == oldChild.Node {
		return nil
	}

	return e.RemoveChild(oldChild)
}

// CloneNode returns a detached copy, including descendants when deep is true.
func (e *Element) CloneNode(deep bool) *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	return &Element{Node: cloneNode(e.Node, deep)}
}

func cloneNode(node *html.Node, deep bool) *html.Node {
	cloned := &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
		Attr:      append([]html.Attribute(nil), node.Attr...),
	}
	if deep {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			cloned.AppendChild(cloneNode(child, true))
		}
	}

	return cloned
}
