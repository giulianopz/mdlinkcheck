### mdlinkcheck

Simple cli tool to check links in a markdown file or in a directory of markdown files:
```bash
$ go install github.com/giulianopz/mdlinkcheck@latest

$ mdlinkcheck -h
Usage of mdlinkcheck:
  -dir string
        path of dir containing markdown files
  -file string
        path of markdown file
  -skip-tls
        ignore invalid TLS/SSL certificates (default false)
  -timeout int
        timeout in seconds (default 5)
  -user-agent string
        impersonate an agent (default "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0")

$ mdlinkcheck --file file.md | grep -v 200 
[err]: https://twitter.com
```
