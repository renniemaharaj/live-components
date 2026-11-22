package element

import (
	"github.com/renniemaharaj/live-components/internal/classlist"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// QuerySelector returns the first matching descendant, excluding this element.
// No match or invalid CSS returns nil.
func (e *Element) QuerySelector(selector string) *Element {
	if e == nil {
		return nil
	}

	return SelectOne(e.Node, selector)
}

// QuerySelectorAll returns matching descendants in document order as a snapshot.
// This element is excluded. No matches or invalid CSS returns nil.
func (e *Element) QuerySelectorAll(selector string) []*Element {
	if e == nil {
		return nil
	}

	return SelectAll(e.Node, selector)
}

// GetElementById returns the first descendant with this exact, nonempty ID.
func (e *Element) GetElementById(id string) *Element {
	if e == nil || e.Node == nil || id == "" {
		return nil
	}

	for node := range e.Node.Descendants() {
		match := &Element{Node: node}
		if value, exists := match.GetAttribute("id"); exists && value == id {
			return match
		}
	}

	return nil
}

// GetElementsByTagName returns a snapshot of descendants with the literal tag name.
// HTML names are case-insensitive; "*" matches every element.
func (e *Element) GetElementsByTagName(tag string) []*Element {
	if e == nil || e.Node == nil {
		return nil
	}

	htmlTag := lowerASCII(tag)
	var matches []*Element
	for node := range e.Node.Descendants() {
		name := tag
		if node.Namespace == "" {
			name = htmlTag
		}
		if node.Type == html.ElementNode && (tag == "*" || node.Data == name) {
			matches = append(matches, &Element{Node: node})
		}
	}

	return matches
}

// GetElementsByClassName returns a snapshot of descendants containing all names.
// Names are case-sensitive and separated by HTML whitespace.
func (e *Element) GetElementsByClassName(names string) []*Element {
	if e == nil || e.Node == nil {
		return nil
	}

	requested := classlist.Tokens(names)
	if len(requested) == 0 {
		return nil
	}

	var matches []*Element
	for node := range e.Node.Descendants() {
		if node.Type != html.ElementNode {
			continue
		}

		candidate := &Element{Node: node}
		classes := candidate.ClassList()
		matchesAll := true
		for _, name := range requested {
			if !classes.Contains(name) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			matches = append(matches, candidate)
		}
	}

	return matches
}

func SelectOne(node *html.Node, selector string) *Element {
	if node == nil {
		return nil
	}

	matcher, err := cascadia.Compile(selector)
	if err != nil {
		return nil
	}

	for descendant := range node.Descendants() {
		if descendant.Type == html.ElementNode && matcher.Match(descendant) {
			return &Element{Node: descendant}
		}
	}

	return nil
}

func SelectAll(node *html.Node, selector string) []*Element {
	if node == nil {
		return nil
	}

	matcher, err := cascadia.Compile(selector)
	if err != nil {
		return nil
	}

	var matches []*Element
	for descendant := range node.Descendants() {
		if descendant.Type == html.ElementNode && matcher.Match(descendant) {
			matches = append(matches, &Element{Node: descendant})
		}
	}

	return matches
}
