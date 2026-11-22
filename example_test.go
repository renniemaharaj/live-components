package live_test

import (
	"fmt"

	live "github.com/renniemaharaj/live-component"
)

func Example() {
	page, err := live.Parse(`<h1 id="title">Old title</h1><div class="banner"></div>`)
	if err != nil {
		panic(err)
	}

	document := page.Document()
	document.GetElementById("title").SetTextContent("Hello World")
	banner := document.QuerySelector(".banner")
	if err := banner.ClassList().Add("active"); err != nil {
		panic(err)
	}

	button := document.CreateElement("button")
	button.SetTextContent("Click me")
	if err := banner.AppendChild(button); err != nil {
		panic(err)
	}

	fmt.Println(page.Render())
	// Output:
	// <html><head></head><body><h1 id="title">Hello World</h1><div class="banner active"><button>Click me</button></div></body></html>
}
