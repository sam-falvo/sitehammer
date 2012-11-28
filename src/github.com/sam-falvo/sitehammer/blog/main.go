/*
The blog command renders static HTML for one or more blog articles.

USAGE: blog descs.json

WHERE: descs.json - a file containing a JSON array of article descriptors.

Blog articles are rendered in an output directory called ./article.
Each article rendered exists in a subdirectory named after the numeric article ID.
For example, ./article/1024/index.html.
This allows easy linking to the articles.

The source material for each article appears in a source directory named ./src.
Traditionally, descs.json appears inside ./src.
When looking for abstracts or bodies for each article, the blog command looks in a directory named for the article ID.
E.g., ./src/1024/abstract or ./src/1024/body.
*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
)

// The default place for SiteHammer to look for the template used to generate a blog article.
const blogArticleFilename = "templates/blog-article.html"

// The default place for SiteHammer to look for the template used to generate the blog's front matter/home page.
const blogIndexFilename = "templates/blog-index.html"

// The default place for SiteHammer to place blog article output.
const articleDirName = "./article"

// When creating a new index file, there's the possibility that something will break.
// To prevent damage to the old index file, the blog command will create the new index
// in a temporary file first.
const indexFileCreated = "./index.html.inprogress"

// After the new index has been successfully created, the blog command promotes the new index to replace the old.
const outputIndexFile = "./index.html"

// The number of articles to show on the index page.
// TODO(sfalvo): Make this a user-configurable setting.
const numberOfArticlesOnIndexPage = 5

// descriptor describes a single article in the blog.
// When running the blog generator, the article descriptors file contains an array of these structures, encoded in JSON format.
//
// The Id field uniquely identifies the blog article amongst others as far as the blog generator and external hyperlinks are concerned.
// No two articles share the same Id.
// The Id must be greater than or equal to zero.
// Title identifies to the human reader the name of the article.
// Author identifies who wrote the article.
// Published tells when the article was published, in the date format of the author's choosing.
//
// Note that neither Title, Author, nor Published hold any significance to the blog generator, except their use in filling out an HTML template.
type descriptor struct {
	Id        uint
	Title     string
	Author    string
	Published string
}

// articleData describes a full article, like a descriptor; unlike a descriptor,
// however, the abstract and body data are included.
// Observe that the body is optional (can be nil).
type articleData struct {
	descriptor
	Abstract template.HTML
	Body     *template.HTML
}

// abend abnormally ends the program, usually as a result of some blocking error.
// The specified diagnostic is printed before terminating the program.
// The program stops with shell result code 1.
func abend(reason error) {
	if reason != nil {
		fmt.Println(reason)
		os.Exit(1)
	}
}

// validateDescriptors performs a sanity check over the set of descriptors.
// An error is returned if at least one of the following conditions exists:
// (1) Greater than one article descriptor shares a common Id.
// (2) Title, author, or published fields have zero length.
func validateDescriptors(ds []descriptor) error {
	for i, d := range ds {
		if len(d.Title) == 0 {
			return fmt.Errorf("Article ID %d has zero-length title.", d.Id)
		}
		if len(d.Author) == 0 {
			return fmt.Errorf("Article ID %d has zero-length author.", d.Id)
		}
		if len(d.Published) == 0 {
			return fmt.Errorf("Article ID %d has zero-length publication timestamp.", d.Id)
		}

		for _, e := range ds[i+1 : len(ds)] {
			if d.Id == e.Id {
				return fmt.Errorf("More than one article with ID %d", d.Id)
			}
		}
	}
	return nil
}

func main() {
	var descriptors []descriptor

	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		abend(fmt.Errorf("You need to specify an article descriptor file."))
	}

	raw, err := ioutil.ReadFile(args[0])
	abend(err)
	err = json.Unmarshal(raw, &descriptors)
	abend(err)
	err = validateDescriptors(descriptors)
	abend(err)

	for _, descriptor := range descriptors {
		err = emitStaticHTMLForArticle(descriptor)
		abend(err)
	}

	err = emitStaticHTMLForFrontMatter(descriptors)
	abend(err)
	err = os.Rename(indexFileCreated, outputIndexFile)
	abend(err)
}

// emitStaticHTMLForFrontMatter creates the index.html file for the blog's initial landing page.
func emitStaticHTMLForFrontMatter(ds []descriptor) error {
	finish := len(ds)
	start := finish - numberOfArticlesOnIndexPage
	if start < 0 {
		start = 0
	}
	mostRecentDescriptors := ds[start:finish]
	templateFileContents, err := blogIndexTemplate()
	if err != nil {
		return err
	}
	tmpl, err := template.New("SiteHammer Blog Index").Parse(templateFileContents)
	if err != nil {
		return err
	}
	outputWriter := new(bytes.Buffer)
	err = tmpl.Execute(outputWriter, mostRecentDescriptors)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(indexFileCreated, outputWriter.Bytes(), 0644)
}

// emitStaticHTMLForArticle does as its name suggests.
// It will also attempt to create the relevant directories it needs, including article/ and article/{{id}}.
// If any error occurs while creating the final HTML, all resources related to the article will be removed.
// This leaves the filesystem in a consistent state.
func emitStaticHTMLForArticle(d descriptor) error {
	abstract, err := abstractFor(d.Id)
	if err != nil {
		return err
	}
	body := bodyFor(d.Id)
	templateFileContents, err := blogArticleTemplate()
	if err != nil {
		return err
	}
	tmpl, err := template.New("SiteHammer Blog Article").Parse(templateFileContents)
	if err != nil {
		return err
	}
	keys := &articleData{
		descriptor: descriptor{
			Id:        d.Id,
			Title:     d.Title,
			Author:    d.Author,
			Published: d.Published,
		},
		Abstract: abstract,
		Body:     body,
	}
	outputWriter := new(bytes.Buffer)
	err = tmpl.Execute(outputWriter, keys)
	if err != nil {
		return err
	}
	err = ensureExistanceOfOutputDirectories(d.Id)
	if err != nil {
		return err
	}

	err = generateIndexFile(d.Id, outputWriter.Bytes())
	if err != nil {
		err2 := unlinkHtmlAndDir(d.Id)
		if err2 != nil {
			return err2
		}
		return err
	}

	return nil
}

// Writes out text to an article's index.html file.
// Returns nil if everything went OK; otherwise, an error is returned.
func generateIndexFile(id uint, content []byte) error {
	return ioutil.WriteFile(outputFilenameFor(id, "index.html"), content, 0644)
}

// unlinkHtmlAndDir attempts to remove the index.html file and the directory it sits in.
// It does not attempt, however, to remove the articles directory.
func unlinkHtmlAndDir(id uint) error {
	return os.RemoveAll(outputFilenameFor(id, ""))
}

// inputFilenameFor derives a filename in source data filesystem space.
func inputFilenameFor(id uint, kind string) string {
	return fmt.Sprintf("src/%d/%s", id, kind)
}

// abstractFor attempts to locate the abstract for an article.
// For an article with ID 1234, SiteHammer's blog command expects the abstract to appear in the ./src/1234/abstract file.
// If not found, it returns a relevant error.
// Otherwise, it returns the raw text contained in the abstract.
func abstractFor(id uint) (text template.HTML, err error) {
	content, err := ioutil.ReadFile(inputFilenameFor(id, "abstract"))
	if err != nil {
		text = ""
		return
	}
	text = template.HTML(*bytesAsString(content))
	return
}

// bodyFor attempts to locate the body for an article.
// This procedure cannot fail.
// If, for some reason, a body file cannot be found, nil is returned.
// Otherwise, a slice containing the entirety of the body results.
func bodyFor(id uint) *template.HTML {
	text, err := ioutil.ReadFile(inputFilenameFor(id, "body"))
	if err != nil {
		return nil
	}
	h := template.HTML(*bytesAsString(text))
	return &h
}

// blogTemplateFor retrieves a blog template file, or an error if unsuccessful.
func blogTemplateFor(filename string) (s string, err error) {
	s = ""
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	s = *bytesAsString(contents)
	return
}

// blogIndexTemplate retrieves the blog index.html template, or an error if unsuccessful.
func blogIndexTemplate() (s string, err error) {
	return blogTemplateFor(blogIndexFilename)
}

// blogArticleTemplate retrieves the blog article template, or an error if unsuccessful.
// BUG(sam-falvo) Instead of reading and parsing the template every time, I should do this once at program startup.
// For now, however, it's not a big deal.
func blogArticleTemplate() (s string, err error) {
	return blogTemplateFor(blogArticleFilename)
}

// bytesAsString converts []byte to a string pointer.
// If you want just a regular string, use *bytesAsString().
func bytesAsString(bs []byte) *string {
	buf := bytes.NewBuffer(bs)
	s := buf.String()
	return &s
}

// ensureExistanceOfOutputDirectories does as its name suggests.
// If the article directory doesn't exist, attempt to create it.
// Assuming that's successful, attempt to create the directory for the article ID as well.
// Return success only if both directories were created OR if they already existed.
// Otherwise, return a meaningful error.
func ensureExistanceOfOutputDirectories(id uint) error {
	err := ensureIsDir(articleDirName)
	if err != nil {
		return err
	}
	return ensureIsDir(outputFilenameFor(id, ""))
}

// ensureIsDir checks to see if the given pathname already exists as a directory.
// If the given pathname already is a directory or it can be created as one,
// nil is returned.  Otherwise, a relevant error is returned.
func ensureIsDir(pathname string) error {
	fi, err := os.Stat(pathname)

	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(pathname, os.ModeDir|0755)
		}

		return err
	}

	if (fi.Mode() & os.ModeDir) == 0 {
		return fmt.Errorf("Path %s exists, but isn't a directory", pathname)
	}

	return nil
}

// outputFilenameFor derives a filename in output data filesystem space.
func outputFilenameFor(id uint, kind string) string {
	if len(kind) > 0 {
		return fmt.Sprintf("%s/%d/%s", articleDirName, id, kind)
	}

	return fmt.Sprintf("%s/%d", articleDirName, id)
}
