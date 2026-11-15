package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/signal"
	"sync"
	"syscall"
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

func StartCrawler(startLink string, depth, timeout, workerPool int) error {
	if workerPool <= 0 {
		return nil
	}

	u, err := url.Parse(startLink)
	if err != nil {
		return err
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

	//Контекст отмены по сигналу
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	//Канал для проверки завершения/шатдауна
	jobDone := make(chan struct{})

	go func() {
		defer close(jobDone)

		<-ctx.Done()
		crawler.q.ForceClose()
	}()

	initWorkers(ctx, &crawler, workerPool, &workers)
	q.Push(Task{
		URL:   u,
		depth: depth,
	})
	workers.Wait()

	cancel()
	//Гарантирует завершение управляющей горутины до выхода
	<-jobDone

	numberOfLinks := 0
	crawler.foundLinks.Range(func(key, value any) bool {
		fmt.Println(key)
		numberOfLinks++
		return true
	})
	fmt.Printf("Links found: %v", numberOfLinks)
	return nil
}

func initWorkers(ctx context.Context, cr *Crawler, amount int, wg *sync.WaitGroup) {
	wg.Add(amount)
	for i := 0; i < amount; i++ {
		go func() {
			Worker(ctx, cr)
			wg.Done()
		}()
	}
}

func Worker(ctx context.Context, crawler *Crawler) {
	for {
		task, ok := crawler.q.Pop()
		if !ok {
			return
		}
		Crawl(ctx, crawler, task.URL, task.depth)
	}
}

func Crawl(ctx context.Context, cr *Crawler, u *url.URL, depth int) {

	defer cr.q.Done()
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		u.String(),
		nil,
	)
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

		//При отмене контекста:
		select {
		case <-ctx.Done():
			return
		default:
		}

		//На случай редиректа, основной URL берется из запроса
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
