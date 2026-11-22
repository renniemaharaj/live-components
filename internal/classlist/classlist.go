package classlist

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// ClassList reads the current class attribute on each operation.
type ClassList struct {
	// Node is retained for compatibility. Prefer Element.ClassList().
	Node *html.Node
}

func (cl *ClassList) className() string {
	if cl == nil || cl.Node == nil || cl.Node.Type != html.ElementNode {
		return ""
	}

	for _, attribute := range cl.Node.Attr {
		if attribute.Namespace == "" && attribute.Key == "class" {
			return attribute.Val
		}
	}

	return ""
}

func (cl *ClassList) set(value string) {
	if cl == nil || cl.Node == nil || cl.Node.Type != html.ElementNode {
		return
	}

	for i := range cl.Node.Attr {
		if cl.Node.Attr[i].Namespace == "" && cl.Node.Attr[i].Key == "class" {
			cl.Node.Attr[i].Val = value
			return
		}
	}

	cl.Node.Attr = append(cl.Node.Attr, html.Attribute{Key: "class", Val: value})
}

// Contains reports whether name is present as a single, case-sensitive class token.
func (cl *ClassList) Contains(name string) bool {
	for _, value := range Tokens(cl.className()) {
		if value == name {
			return true
		}
	}

	return false
}

// Add appends unique class tokens. Invalid tokens return an error before any change.
func (cl *ClassList) Add(names ...string) error {
	if err := validate(names); err != nil {
		return err
	}

	values := Tokens(cl.className())
	seen := make(map[string]bool, len(values))
	for _, name := range values {
		seen[name] = true
	}

	for _, name := range names {
		if !seen[name] {
			values = append(values, name)
			seen[name] = true
		}
	}

	cl.set(strings.Join(values, " "))
	return nil
}

// Remove deletes class tokens. Invalid tokens return an error before any change.
func (cl *ClassList) Remove(names ...string) error {
	if err := validate(names); err != nil {
		return err
	}

	remove := make(map[string]bool, len(names))
	for _, name := range names {
		remove[name] = true
	}

	var kept []string
	for _, name := range Tokens(cl.className()) {
		if !remove[name] {
			kept = append(kept, name)
		}
	}

	cl.set(strings.Join(kept, " "))
	return nil
}

// Toggle returns whether the token is present after toggling, or an invalid-token error.
func (cl *ClassList) Toggle(name string) (bool, error) {
	if err := validate([]string{name}); err != nil {
		return false, err
	}
	if cl.Contains(name) {
		return false, cl.Remove(name)
	}
	if err := cl.Add(name); err != nil {
		return false, err
	}

	return cl.Contains(name), nil
}

func validate(names []string) error {
	for _, name := range names {
		if name == "" || strings.IndexFunc(name, isWhitespace) >= 0 {
			return fmt.Errorf("class token %q must be nonempty and contain no HTML whitespace", name)
		}
	}

	return nil
}

// Tokens splits on HTML whitespace and preserves the first occurrence of each token.
func Tokens(value string) []string {
	var tokens []string
	seen := make(map[string]bool)
	for _, token := range strings.FieldsFunc(value, isWhitespace) {
		if !seen[token] {
			tokens = append(tokens, token)
			seen[token] = true
		}
	}

	return tokens
}

func isWhitespace(character rune) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r' || character == '\f'
}
