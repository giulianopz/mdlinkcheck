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
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	file      string
	dir       string
	skipTLS   bool
	timeout   int64
	userAgent string
)

var markdownRegex = regexp.MustCompile(`\[[^][]+]\((https?://[^()]+)\)`)

var client = &http.Client{
	Timeout: time.Duration(timeout) * time.Second,
}

const mdExt = ".md"

func main() {

	flag.StringVar(&file, "file", "", "path of markdown file")
	flag.StringVar(&dir, "dir", "", "path of dir containing markdown files")
	flag.BoolVar(&skipTLS, "skip-tls", false, "ignore invalid TLS/SSL certificates (default false)")
	flag.Int64Var(&timeout, "timeout", 5, "timeout in seconds")
	flag.StringVar(&userAgent, "user-agent", "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0", "impersonate an agent")
	flag.Parse()

	if skipTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

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
	matches := markdownRegex.FindAllStringSubmatch(string(bs), -1)

	g := errgroup.Group{}
	g.SetLimit(-1)

	for len(matches) != 0 {

		link := matches[0][1]

		g.Go(func() error {

			req, err := http.NewRequest(http.MethodGet, link, nil)
			if err != nil {
				return err
			}

			req.Header.Add("User-Agent", userAgent)

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("[err]: %q\n", link)
			} else {
				resp.Body.Close()
				fmt.Printf("[%d]: %s\n", resp.StatusCode, link)
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
