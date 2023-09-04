package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"golang.org/x/sync/errgroup"
)

type CheckResult struct {
	HTTPCode int
	Referrer string
	Error    error
	Body     string
	Recursed bool
}

var (
	linksChecked map[string]*CheckResult
	path         string
	skipTLS      bool
	timeout      int
)

var markdownRegex = regexp.MustCompile(`\[[^][]+]\((https?://[^()]+)\)`)

func handle(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {

	flag.StringVar(&path, "path", "", "path of markdown file")
	flag.BoolVar(&skipTLS, "skiptls", false, "To try site with invalid certificate, default: false")
	flag.IntVar(&timeout, "timeout", 5, "Timeout in seconds.")
	flag.Parse()

	bs, err := os.ReadFile(path)
	handle(err)

	matches := markdownRegex.FindAllStringSubmatch(string(bs), -1)
	g := errgroup.Group{}
	g.SetLimit(-1)
	for _, m := range matches {

		link := m[1]

		g.Go(func() error {
			resp, err := http.Get(link)
			if err != nil {
				fmt.Println("broken link:", link)
			}
			defer resp.Body.Close()
			fmt.Println("reachable link:", link)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		panic(err)
	}
}
