package markdown

import (
	"bufio"
	"regexp"
	"strings"

	"github.com/mickael-menu/zk/core/note"
	"github.com/mickael-menu/zk/util/opt"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Parser parses the content of Markdown notes.
type Parser struct {
	md goldmark.Markdown
}

// NewParser creates a new Markdown Parser.
func NewParser() *Parser {
	return &Parser{
		md: goldmark.New(
			goldmark.WithExtensions(
				meta.Meta,
			),
		),
	}
}

// Parse implements note.Parse.
func (p *Parser) Parse(source string) (note.Content, error) {
	out := note.Content{}

	bytes := []byte(source)

	context := parser.NewContext()
	root := p.md.Parser().Parse(
		text.NewReader(bytes),
		parser.WithContext(context),
	)

	frontmatter, err := parseFrontmatter(context, bytes)
	if err != nil {
		return out, err
	}

	title, bodyStart, err := parseTitle(frontmatter, root, bytes)
	if err != nil {
		return out, err
	}

	out.Title = title
	out.Body = parseBody(bodyStart, bytes)
	out.Lead = parseLead(out.Body)

	return out, nil
}

// parseTitle extracts the note title with its node.
func parseTitle(frontmatter frontmatter, root ast.Node, source []byte) (title opt.String, bodyStart int, err error) {
	if title = frontmatter.getString("title", "Title"); !title.IsNull() {
		bodyStart = frontmatter.end
		return
	}

	var titleNode *ast.Heading
	err = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := n.(*ast.Heading); ok && entering &&
			(titleNode == nil || heading.Level < titleNode.Level) {

			titleNode = heading
			if heading.Level == 1 {
				return ast.WalkStop, nil
			}
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return
	}

	if titleNode != nil {
		title = opt.NewNotEmptyString(string(titleNode.Text(source)))

		if lines := titleNode.Lines(); lines.Len() > 0 {
			bodyStart = lines.At(lines.Len() - 1).Stop
		}
	}
	return
}

// parseBody extracts the whole content after the title.
func parseBody(startIndex int, source []byte) opt.String {
	return opt.NewNotEmptyString(
		strings.TrimSpace(
			string(source[startIndex:]),
		),
	)
}

// parseLead extracts the body content until the first blank line.
func parseLead(body opt.String) opt.String {
	lead := ""
	scanner := bufio.NewScanner(strings.NewReader(body.String()))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			break
		}
		lead += scanner.Text() + "\n"
	}

	return opt.NewNotEmptyString(strings.TrimSpace(lead))
}

// frontmatter contains metadata parsed from a YAML frontmatter.
type frontmatter struct {
	values map[string]interface{}
	start  int
	end    int
}

var frontmatterRegex = regexp.MustCompile(`(?ms)^\s*-+\s*$.*?^\s*-+\s*$`)

func parseFrontmatter(context parser.Context, source []byte) (front frontmatter, err error) {
	index := frontmatterRegex.FindIndex(source)
	if index != nil {
		front.start = index[0]
		front.end = index[1]
		front.values, err = meta.TryGet(context)
	}
	return
}

// getString returns the first string value found for any of the given keys.
func (m frontmatter) getString(keys ...string) opt.String {
	if m.values == nil {
		return opt.NullString
	}

	for _, key := range keys {
		if val, ok := m.values[key]; ok {
			if val, ok := val.(string); ok {
				return opt.NewNotEmptyString(val)
			}
		}
	}
	return opt.NullString
}