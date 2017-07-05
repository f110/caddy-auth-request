package authrequest

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

const (
	HeaderName = "X-AUTH-REQUEST"
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

	res, err := ar.sendRequestToBackend(ctx, r)
	if err != nil {
		return 0, err
	}

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusAccepted {
		if v := res.Header.Get(HeaderName); v != "" {
			r.Header.Add(HeaderName, v)
		}
		return ar.Next.ServeHTTP(w, r)
	}

	// If backend doesn't return status ok.
	if c := res.Header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				res.Header.Del(f)
			}
		}
	}
	for _, h := range hopHeaders {
		res.Header.Del(h)
	}

	copyHeader(w.Header(), res.Header)

	if len(res.Trailer) > 0 {
		trailerKeys := make([]string, 0, len(res.Trailer))
		for k := range res.Trailer {
			trailerKeys = append(trailerKeys, k)
		}
		w.Header().Add("Trailer", strings.Join(trailerKeys, ", "))
	}
	w.WriteHeader(res.StatusCode)
	if len(res.Trailer) > 0 {
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
	copyResponse(w, res.Body)
	res.Body.Close()
	copyHeader(w.Header(), res.Trailer)

	return 0, nil
}

func (ar AuthRequestHandler) sendRequestToBackend(ctx context.Context, req *http.Request) (*http.Response, error) {
	backendReq := new(http.Request)
	*backendReq = *req
	backendReq = req.WithContext(ctx)
	backendReq.URL.Host = ar.Backend.Host
	backendReq.URL.Scheme = ar.Backend.Scheme
	backendReq.Close = false
	if req.ContentLength == 0 {
		backendReq.Body = nil
	}
	backendReq = backendReq.WithContext(req.Context())

	copiedHeaders := false
	if c := backendReq.Header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				if !copiedHeaders {
					backendReq.Header = make(http.Header)
					copyHeader(backendReq.Header, req.Header)
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
				copyHeader(backendReq.Header, req.Header)
				copiedHeaders = true
			}
			backendReq.Header.Del(h)
		}
	}

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if prior, ok := backendReq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		backendReq.Header.Set("X-Forwarded-For", clientIP)
	}

	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	backendReq.Header.Set("X-Forwarded-Proto", scheme)

	return http.DefaultTransport.RoundTrip(backendReq)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func copyResponse(dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				return written, werr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if rerr != nil {
			return written, rerr
		}
	}
}
