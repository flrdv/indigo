<p align="center">
<img src="https://gist.githubusercontent.com/flrdv/08395567a602992be0c72f445589cb90/raw/248c01ff18936b36629e520b2164ef6d78f0f57f/indigo-logo.svg" alt="This is just a logo" title="What are you looking for?" width="40%" height="40%"/>
</p>

Indigo is a web-framework focusing at performance, elegancy and robustness.

# Documentation

Documentation is available [here](https://floordiv.gitbook.io/indigo/). It might be incomplete. In this case, feel free to open an issue.

# Hello, world!

```golang
package main

import (
	"log"

	"github.com/indigo-web/indigo"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/router/inbuilt"
)

func HelloWorld(request *http.Request) *http.Response {
	return http.String(request, "Hello, world!")
}

func Log(request *http.Request) *http.Response {
	text, err := request.Body.String()
	if err != nil {
		return http.Error(request, err)
	}

	log.Printf("%s says: %s", request.Remote, text)
	return http.String(request, text)
}

func main() {
	r := inbuilt.New()
	r.Resource("/").
		Get(HelloWorld).
		Post(Log)

	err := indigo.New(":8080").Serve(r)
	if err != nil {
		log.Fatal(err)
	}
}

```

You can find more examples in [examples/](https://github.com/indigo-web/indigo/tree/master/examples).
