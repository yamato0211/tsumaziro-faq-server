package batch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Anchor struct {
	Text string
	Href string
}

type HTML struct {
	Body string
}

type BucketBasics struct {
	S3Client *s3.Client
}

func (basics BucketBasics) UploadFile(bucketName string, objectKey string, fileContent io.Reader) error {
	_, err := basics.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   fileContent,
	})
	if err != nil {
		return err
	}

	return nil
}

func newAnchor(node *html.Node) *Anchor {
	var buff bytes.Buffer
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			buff.WriteString(c.Data)
		}
	}

	// href属性の値を取得
	href := ""
	for _, v := range node.Attr {
		if v.Key == "href" {
			href = v.Val
			break
		}
	}

	return &Anchor{Text: buff.String(), Href: href}
}

func findAnchors(node *html.Node, collection *[]*Anchor) {
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			if c.DataAtom == atom.A {
				*collection = append(*collection, newAnchor(c))
			}
			findAnchors(c, collection)
		}
	}
}

func CrawlKnowledge(url, bucketName, objectKey string, s3Client *s3.Client) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(body)
	parsed := buf.String()
	r := strings.NewReader(parsed)

	htmls := make(map[string]*HTML)

	htmls[url] = &HTML{parsed}

	node, err := html.Parse(r)
	if err != nil {
		return err
	}

	var collection []*Anchor
	findAnchors(node, &collection)

	for _, a := range collection {
		fmt.Println(a.Text, ":", a.Href)
		if strings.Contains(a.Href, url) {
			if _, ok := htmls[a.Href]; !ok {
				resp, err := http.Get(a.Href)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				buf := bytes.NewBuffer(body)
				parsed := buf.String()
				htmls[a.Href] = &HTML{parsed}
			}
		}
	}

	content := ""
	for _, html := range htmls {
		content += html.Body + "¥n"
	}
	fileContent := strings.NewReader(content)
	bucketBasics := BucketBasics{S3Client: s3Client}
	if err := bucketBasics.UploadFile(bucketName, objectKey, fileContent); err != nil {
		return err
	}

	return nil
}
