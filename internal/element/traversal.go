package element

import "golang.org/x/net/html"

// ParentElement returns the immediate parent if it is an element.
func (e *Element) ParentElement() *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	parent := e.Node.Parent
	if parent == nil || parent.Type != html.ElementNode {
		return nil
	}

	return &Element{Node: parent}
}

// Children returns a snapshot of direct element children, excluding text and comments.
func (e *Element) Children() []*Element {
	if e == nil || e.Node == nil {
		return nil
	}

	var children []*Element
	for node := e.Node.FirstChild; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode {
			children = append(children, &Element{Node: node})
		}
	}

	return children
}

// FirstElementChild returns the first direct child that is an element.
func (e *Element) FirstElementChild() *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	for node := e.Node.FirstChild; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode {
			return &Element{Node: node}
		}
	}

	return nil
}

// LastElementChild returns the last direct child that is an element.
func (e *Element) LastElementChild() *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	for node := e.Node.LastChild; node != nil; node = node.PrevSibling {
		if node.Type == html.ElementNode {
			return &Element{Node: node}
		}
	}

	return nil
}

// NextElementSibling skips text and comments to find the next sibling element.
func (e *Element) NextElementSibling() *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	for node := e.Node.NextSibling; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode {
			return &Element{Node: node}
		}
	}

	return nil
}

// PreviousElementSibling skips text and comments to find the previous sibling element.
func (e *Element) PreviousElementSibling() *Element {
	if e == nil || e.Node == nil {
		return nil
	}

	for node := e.Node.PrevSibling; node != nil; node = node.PrevSibling {
		if node.Type == html.ElementNode {
			return &Element{Node: node}
		}
	}

	return nil
}

// Contains reports whether other is this node or any descendant of it.
func (e *Element) Contains(other *Element) bool {
	if e == nil || e.Node == nil || other == nil {
		return false
	}

	for node := other.Node; node != nil; node = node.Parent {
		if node == e.Node {
			return true
		}
	}

	return false
}
