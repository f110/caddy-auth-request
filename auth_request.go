package authrequest

import (
	"net/http"

	"context"

	"net/url"

	"strings"

	"net"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

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

	copiedHeaders := false
	if c := backendReq.Header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				if !copiedHeaders {
					backendReq.Header = make(http.Header)
					copyHeader(backendReq.Header, r.Header)
					copiedHeaders = true
				}
				backendReq.Header.Del(f)
			}
		}
	}

	for _, h := range hopHeaders {
		if backendReq.Header.Get(h) != "" {
			if !copiedHeaders {
				backendReq.Header = make(http.Header)
				copyHeader(backendReq.Header, r.Header)
				copiedHeaders = true
			}
			backendReq.Header.Del(h)
		}
	}

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if prior, ok := backendReq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		backendReq.Header.Set("X-Forwarded-For", clientIP)
	}

	res, err := http.DefaultTransport.RoundTrip(backendReq)
	if err != nil {
		return ar.Next.ServeHTTP(w, r)
	}
	if res.StatusCode == http.StatusOK {
		return ar.Next.ServeHTTP(w, r)
	}

	w.WriteHeader(res.StatusCode)
	return 0, nil
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
