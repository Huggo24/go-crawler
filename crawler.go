package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Crawler struct {
	q            *Queue
	visitedLinks sync.Map
	foundLinks   sync.Map
	maxDepth     int
	client       *http.Client
}

func StartCrawler(startLink string, depth, timeout, workerPool int) {
	if workerPool == 0 {
		return
	}

	u, err := url.Parse(startLink)
	if err != nil {
		panic(err)
	}

	q := &Queue{}
	q.cond = sync.NewCond(&q.mu)

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	crawler := Crawler{
		q:            q,
		maxDepth:     depth,
		client:       client,
		visitedLinks: sync.Map{},
	}
	crawler.visitedLinks.Store(u.String(), struct{}{})

	var workers sync.WaitGroup

	initWorkers(&crawler, workerPool, &workers)
	q.Push(Task{
		URL:   u,
		depth: depth,
	})
	workers.Wait()
	crawler.foundLinks.Range(func(key, value any) bool { fmt.Println(key); return true })
}

func initWorkers(cr *Crawler, amount int, wg *sync.WaitGroup) {
	wg.Add(amount)
	for i := 0; i < amount; i++ {
		go func() {
			Worker(cr)
			wg.Done()
		}()
	}
}

func Worker(crawler *Crawler) {
	for {
		task, ok := crawler.q.Pop()
		if !ok {
			return
		}
		Crawl(crawler, task.URL, task.depth)
	}
}

func Crawl(cr *Crawler, u *url.URL, depth int) {

	defer cr.q.Done()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", "Go_Crawler")

	resp, err := cr.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	urlHtml, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	doc, err := html.Parse(bytes.NewReader(urlHtml))
	if err != nil {
		return
	}

	links := FindLinks(doc)
	for _, link := range links {
		//На случай редиректа основной URL берется из запроса
		newUrl, err := resp.Request.URL.Parse(link)
		if err != nil {
			continue
		}

		//Чтобы избежать относительных ссылкок (#)
		newUrl.Fragment = ""

		if newUrl.Scheme != "http" && newUrl.Scheme != "https" {
			continue
		}

		if newUrl.Host != u.Host {
			continue
		}

		cr.foundLinks.Store(newUrl.String(), struct{}{})
		if depth <= 0 {
			continue
		}

		if _, ok := cr.visitedLinks.LoadOrStore(newUrl.String(), struct{}{}); !ok {
			cr.q.Push(Task{
				URL:   newUrl,
				depth: depth - 1,
			})
		}
	}

}

func FindLinks(n *html.Node) []string {
	var links []string

	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				links = append(links, attr.Val)
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		links = append(links, FindLinks(child)...)
	}

	return links
}
