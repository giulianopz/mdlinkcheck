package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/rs/dnscache"
)

var (
	file      string
	dir       string
	skipTLS   bool
	timeout   int64
	userAgent string
	method    string
)

var markdownRegex = regexp.MustCompile(`\[[^][]+]\((https?://[^()]+)\)`)

const mdExt = ".md"

var client = &http.Client{
	Timeout: time.Duration(timeout) * time.Second,
	// do not follow redirects
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func main() {

	flag.StringVar(&file, "file", "", "path of markdown file")
	flag.StringVar(&dir, "dir", "", "path of dir containing markdown files")
	flag.BoolVar(&skipTLS, "skip-tls", false, "ignore invalid TLS/SSL certificates (default false)")
	flag.Int64Var(&timeout, "timeout", 5, "timeout in seconds")
	flag.StringVar(&userAgent, "user-agent", "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0", "impersonate an agent")
	flag.StringVar(&method, "http-method", http.MethodGet, "http verb: HEAD (faster, less accurate) or GET (slower, trustworthy)")
	flag.Parse()

	r := &dnscache.Resolver{}

	t := &http.Transport{
		// connection pooling params
		MaxIdleConns:    100,
		MaxConnsPerHost: 10,
		IdleConnTimeout: 10 * time.Second,
		// in-memory dns caching
		DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := r.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				var dialer net.Dialer
				conn, err = dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					break
				}
			}
			return
		},
	}

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()

		for range t.C {
			options := dnscache.ResolverRefreshOptions{
				ClearUnused:      true,
				PersistOnFailure: true,
			}
			r.RefreshWithOptions(options)
		}
	}()

	if skipTLS {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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

	if strings.ToUpper(method) != http.MethodHead && strings.ToUpper(method) != http.MethodGet {
		return fmt.Errorf("http method must be HEAD or GET")
	}

	bs, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	matches := markdownRegex.FindAllStringSubmatch(string(bs), -1)

	g := errgroup.Group{}
	g.SetLimit(-1)

	for len(matches) != 0 {

		link := matches[0][1]

		g.Go(func() error {

			req, err := http.NewRequest(strings.ToUpper(method), link, nil)
			if err != nil {
				return err
			}

			req.Header.Add("User-Agent", userAgent)
			req.Header.Add("Accept", "*/*")

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("[err]: %q\n", link)
			} else {
				fmt.Printf("[%d]: %s\n", resp.StatusCode, link)
				defer resp.Body.Close()
			}

			return nil
		})

		matches = matches[1:]
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func checkDir(dir string) error {
	return filepath.Walk(dir, visit)
}

func visit(p string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if path.Ext(p) == mdExt {
		return checkFile(p)
	}
	return nil
}
