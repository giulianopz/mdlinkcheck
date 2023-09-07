package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	file      string
	dir       string
	skipTLS   bool
	timeout   int64
	userAgent string
	method    string
	follow    bool
)

var markdownLinkRgx = regexp.MustCompile(`\[[^][]+]\((https?://[^()]+)\)`)

var client = &http.Client{
	Timeout: time.Duration(timeout) * time.Second,
}

const mdExt = ".md"

func main() {

	flag.StringVar(&file, "file", "", "path of markdown file")
	flag.StringVar(&dir, "dir", "", "path of dir containing markdown files")
	flag.BoolVar(&skipTLS, "skip-tls", false, "ignore invalid TLS/SSL certificates (default false)")
	flag.Int64Var(&timeout, "timeout", 5, "timeout in seconds")
	flag.StringVar(&method, "http-method", http.MethodGet, "http method: HEAD (faster, less accurate) or GET (slower, trustworthy)")
	flag.StringVar(&userAgent, "user-agent", "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0", "impersonate an agent")
	flag.BoolVar(&follow, "follow", true, "follow redirects")
	flag.Parse()

	method = strings.ToUpper(method)
	if method != http.MethodGet && method != http.MethodHead {
		fmt.Println("err: --http-method must be HEAD or GET")
		os.Exit(1)
	}

	t := &http.Transport{
		// connection pooling params
		MaxIdleConns:    100,
		MaxConnsPerHost: 10,
		IdleConnTimeout: 10 * time.Second,
	}

	if !follow {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	if skipTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client.Transport = t

	var e error
	if file != "" {
		if err := checkFile(file); err != nil {
			e = err
		}
	} else if dir != "" {
		if err := checkDir(dir); err != nil {
			e = err
		}
	} else {
		flag.Usage()
		e = fmt.Errorf("missing mandatory flags: use -file or -dir")
	}

	if e != nil {
		fmt.Println("err:", e)
		os.Exit(-1)
	}
}

func checkFile(file string) error {

	if path.Ext(file) != mdExt {
		return fmt.Errorf("not a markdown file")
	}

	bs, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	links := extract(string(bs))

	g := errgroup.Group{}
	g.SetLimit(-1)

	for len(links) != 0 {

		link := links[0]
		g.Go(func() error {
			return check(link)
		})
		link = link[1:]
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func checkDir(dir string) error {

	done := make(chan struct{})
	defer close(done)

	linksC, errC := getLinks(done, dir)

	g := errgroup.Group{}
	g.SetLimit(-1)

	for link := range linksC {
		link := link
		g.Go(func() error {
			return check(link)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := <-errC; err != nil {
		return err
	}

	return nil
}

func getLinks(done <-chan struct{}, root string) (<-chan string, <-chan error) {

	linksC := make(chan string)
	errC := make(chan error, 1)
	go func() {
		var wg sync.WaitGroup
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path.Ext(p) != mdExt {
				return nil
			}
			wg.Add(1)
			go func() {

				bs, err := os.ReadFile(p)
				if err != nil {
					fmt.Println(err)
				} else {
					for _, l := range extract(string(bs)) {
						select {
						case linksC <- l:
						case <-done:
						}
					}
				}
				wg.Done()
			}()

			select {
			case <-done:
				return fmt.Errorf("done")
			default:
				return nil
			}
		})

		go func() {
			wg.Wait()
			close(linksC)
		}()
		errC <- err
	}()
	return linksC, errC
}

func extract(mdText string) []string {
	links := make([]string, 0)
	matches := markdownLinkRgx.FindAllStringSubmatch(mdText, -1)
	for _, m := range matches {
		links = append(links, m[1])
	}
	return links
}

func check(link string) error {
	req, err := http.NewRequest(method, link, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[err]: %q\n", link)
	} else {
		resp.Body.Close()
		fmt.Printf("[%d]: %s\n", resp.StatusCode, link)
	}
	return nil
}
