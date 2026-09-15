package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/yuin/goldmark"
)

func main() {
	input := flag.String("i", "", "input markdown file")
	output := flag.String("o", "", "output pdf file (default: <input>.pdf)")
	flag.Parse()

	if *input == "" {
		if flag.NArg() > 0 {
			*input = flag.Arg(0)
		} else {
			fmt.Fprintln(os.Stderr, "usage: markdown2pdf -i input.md [-o output.pdf]")
			os.Exit(1)
		}
	}

	if *output == "" {
		ext := filepath.Ext(*input)
		*output = strings.TrimSuffix(*input, ext) + ".pdf"
	}

	if err := convert(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("generated: %s\n", *output)
}

func convert(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	var buf strings.Builder
	if err := goldmark.Convert(data, &buf); err != nil {
		return err
	}

	html, err := renderHTML(filepath.Base(input), buf.String())
	if err != nil {
		return err
	}

	return htmlToPDF(html, output)
}

func renderHTML(title, body string) (string, error) {
	const tpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<style>
body { font-family: -apple-system, "Segoe UI", "Helvetica Neue", Arial, sans-serif; margin: 2.5rem auto; max-width: 48rem; padding: 0 1.5rem; line-height: 1.7; color: #24292e; }
h1, h2, h3, h4 { line-height: 1.3; margin-top: 1.5em; }
code { background: #f6f8fa; padding: 0.2em 0.4em; border-radius: 4px; font-family: "SF Mono", Menlo, monospace; font-size: 0.9em; }
pre { background: #f6f8fa; padding: 1em; border-radius: 6px; overflow-x: auto; }
pre code { background: none; padding: 0; }
blockquote { border-left: 4px solid #d0d7de; margin: 0; padding-left: 1em; color: #57606a; }
table { border-collapse: collapse; }
th, td { border: 1px solid #d0d7de; padding: 0.4em 0.8em; }
img { max-width: 100%; }
</style>
</head>
<body>
{{.Body}}
</body>
</html>`

	t, err := template.New("page").Parse(tpl)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if err := t.Execute(&out, map[string]any{"Title": title, "Body": template.HTML(body)}); err != nil {
		return "", err
	}
	return out.String(), nil
}

func htmlToPDF(html, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithLogf(func(string, ...interface{}) {}),
	)
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			tree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(tree.Frame.ID, html).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	return os.WriteFile(output, buf, 0o644)
}
