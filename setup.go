package authrequest

import (
	"errors"
	"fmt"
	"os"

	"net/url"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

func init() {
	caddy.RegisterPlugin("authrequest", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	backend, err := authrequestParse(c)
	if err != nil {
		return err
	}
	u, err := url.Parse(backend)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, u.Host)
	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return AuthRequestHandler{Next: next, Backend: u}
	})

	return nil
}

func authrequestParse(c *caddy.Controller) (string, error) {
	c.Next()
	arg := c.RemainingArgs()
	if len(arg) > 0 {
		return arg[0], nil
	}

	return "", errors.New("authrequest requires backend")
}
