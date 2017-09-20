package renderer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/russross/blackfriday"
	"github.com/sourcegraph/syntaxhighlight"
)

// Renderer support conversion from markdown file into html file
type Renderer struct {
	// whether image file being base64 encoded and included in html
	ImageInline bool
	// html template
	Template string
	// css style to be included in html
	Style string
	// base directory where markdown files are located
	BaseDir string
	// output directory
	OutDir string
}

// Render converts markdown to html and write it to file
func (r *Renderer) Render(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	markdowned := blackfriday.MarkdownCommon(data)

	outPath, err := outPath(path, r.OutDir, r.BaseDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(outPath), os.ModeDir)
	if err != nil {
		return err
	}

	// we need document reader to modify markdowned html text, for example,
	// syntax highlight.
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(markdowned))
	if err != nil {
		return err
	}
	r.highlightCode(doc)
	r.handleImage(doc, filepath.Dir(path))

	content, _ := doc.Html()
	content = strings.Replace(content, "<html><head></head><body>", "", 1)
	content = strings.Replace(content, "</body></html>", "", 1)

	output := r.Template
	output = strings.Replace(output, "{{{style}}}", r.Style, -1)
	output = strings.Replace(output, "{{{content}}}", content, -1)

	err = ioutil.WriteFile(outPath, []byte(output), os.ModeAppend)
	if err != nil {
		return err
	}

	return nil
}

// highlight inside of code tag
func (r *Renderer) highlightCode(doc *goquery.Document) {
	doc.Find("code[class*=\"language-\"]").Each(func(i int, s *goquery.Selection) {
		oldCode := s.Text()
		formatted, err := syntaxhighlight.AsHTML([]byte(oldCode))
		if err != nil {
			log.Fatal(err)
			return
		}
		s.SetHtml(string(formatted))
	})
}

// include image to html document
func (r *Renderer) handleImage(doc *goquery.Document, dirPath string) {
	if r.ImageInline {
		// include image into html document
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			src, _ := s.Attr("src")
			if strings.HasPrefix(src, "http") {
				return
			}

			path := filepath.Join(dirPath, src)
			mime := mime.TypeByExtension(filepath.Ext(path))
			base64, _ := imageToBase64(path)
			srcEnced := fmt.Sprintf("data:%s;base64,%s", mime, base64)
			s.SetAttr("src", srcEnced)
		})
	} else {
		// move image files to out directory
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			src, _ := s.Attr("src")
			if strings.HasPrefix(src, "http") {
				return
			}

			fromPath := filepath.Join(dirPath, src)
			toPath := filepath.Join(r.OutDir, dirPath[utf8.RuneCountInString(r.BaseDir):], src)
			err := os.MkdirAll(filepath.Dir(toPath), os.ModeDir)
			if err != nil {
				log.Fatal(err)
				return
			}
			err = copyFile(fromPath, toPath)
			if err != nil {
				log.Fatal(err)
				return
			}
		})
	}
}

// get output file name
func outPath(input string, outDir string, baseDir string) (string, error) {
	out := filepath.Join(outDir, input[utf8.RuneCountInString(baseDir):])
	return changeExtension(out, "html"), nil
}

// change extension
func changeExtension(path string, toExt string) string {
	return omitExtension(path) + "." + toExt
}

// drop extension from path
func omitExtension(path string) string {
	extension := filepath.Ext(path)
	return path[:utf8.RuneCountInString(path)-utf8.RuneCountInString(extension)]
}

// base64 encoding of image file
func imageToBase64(path string) (string, error) {
	imageFile, err := os.Open(path)
	if err != nil {
		return "", err
	}

	image, err := ioutil.ReadAll(imageFile)

	if err != nil {
		return "", err
	}

	imgEnc := base64.StdEncoding.EncodeToString(image)
	return imgEnc, nil
}

// copy file
func copyFile(srcPath string, destPath string) error {
	if srcPath == destPath {
		return nil
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}