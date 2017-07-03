package authrequest

import (
	"fmt"
	"net/http"

	"context"

	"os"

	"net/url"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type AuthRequestHandler struct {
	Next    httpserver.Handler
	Backend *url.URL
}

func (ar AuthRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if cn, ok := w.(http.CloseNotifier); ok {
		notify := cn.CloseNotify()
		go func() {
			select {
			case <-notify:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	backendReq := new(http.Request)
	*backendReq = *r
	backendReq = r.WithContext(ctx)
	backendReq.URL.Host = ar.Backend.Host
	backendReq.URL.Scheme = ar.Backend.Scheme
	backendReq.Close = false
	if r.ContentLength == 0 {
		backendReq.Body = nil
	}
	backendReq = backendReq.WithContext(r.Context())

	res, err := http.DefaultTransport.RoundTrip(backendReq)
	fmt.Fprintln(os.Stderr, res)
	fmt.Fprintln(os.Stderr, err)
	return ar.Next.ServeHTTP(w, r)
}
