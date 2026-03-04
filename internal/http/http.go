// Package http
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/henrywhitaker3/windowframe/cache"
)

type Server struct {
	srv *http.Server
}

type Options struct {
	Port       int
	URL        *url.URL
	RedirectTo string

	CacheEnabled  bool
	CacheDuration time.Duration
}

func New(opts Options) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/ext-authz", extauthHandler(opts))
	mux.HandleFunc("/auth/ext-authz/", extauthHandler(opts))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("unhandled request", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", opts.Port),
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	if err := s.srv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("start http server: %w", err)
		}
	}
	return nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	return s.srv.Shutdown(ctx)
}

type User struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    struct {
			User           string `json:"user"`
			Group          int    `json:"group"`
			Email          string `json:"email"`
			UserIP         string `json:"user_ip"`
			RequestedGroup int    `json:"requested_group"`
			UUID           string `json:"uuid"`
		} `json:"data"`
	} `json:"response"`
}

func extauthHandler(opts Options) func(w http.ResponseWriter, r *http.Request) {
	client := buildClient()
	cache := cache.NewExpiringCache[string, User]()

	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("handling ext-authz request")
		start := time.Now()

		req, err := buildRequest(r, opts)
		if err != nil {
			slog.Error("could not build auth request", "error", err)
			redirect(r, w, opts)
			return
		}
		req = req.WithContext(r.Context())

		if opts.CacheEnabled {
			if token := tokenCookie(req); token != nil {
				if user, ok := cache.Get(r.Context(), token.Value); ok {
					slog.Info(
						"using cached authentication",
						"user",
						user,
						"latency",
						time.Since(start).String(),
					)
					success(w, user)
					return
				}
			}
		}

		user, err := authorize(client, req)
		if err != nil {
			slog.Debug("authorize failure", "error", err)
			redirect(r, w, opts)
			return
		}

		if opts.CacheEnabled {
			if token := tokenCookie(req); token != nil {
				cache.Put(r.Context(), token.Value, user, opts.CacheDuration)
			}
		}

		slog.Info("user authenticated", "user", user, "latency", time.Since(start).String())
		success(w, user)
	}
}

func authorize(client *http.Client, req *http.Request) (User, error) {
	resp, err := client.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("call api: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("got response", "code", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("non-200 status code: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return User{}, fmt.Errorf("ready response body: %w", err)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return User{}, fmt.Errorf("unmarshal response body: %w", err)
	}

	return user, nil
}

func tokenCookie(r *http.Request) *http.Cookie {
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "organizr_token_") {
			return c
		}
	}
	return nil
}

func success(w http.ResponseWriter, user User) {
	w.Header().Set("X-Organizr-User", user.Response.Data.User)
	w.Header().Set("X-Organizr-Email", user.Response.Data.Email)
	w.WriteHeader(http.StatusOK)
}

func redirect(incoming *http.Request, w http.ResponseWriter, opts Options) {
	to := opts.RedirectTo
	if to == "" {
		to = fmt.Sprintf("%s://%s", opts.URL.Scheme, opts.URL.Host)
	}
	if referer := incoming.Header.Get("Referer"); referer != "" {
		to = fmt.Sprintf("%s?return=%s", url.QueryEscape(referer))
	}
	w.Header().Set("location", to)
	w.WriteHeader(http.StatusFound)
}

func buildRequest(incoming *http.Request, opts Options) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, opts.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if cookie := tokenCookie(incoming); cookie != nil {
		req.AddCookie(&http.Cookie{
			Name:        cookie.Name,
			Path:        cookie.Path,
			Value:       cookie.Value,
			Quoted:      cookie.Quoted,
			Domain:      cookie.Domain,
			Expires:     cookie.Expires,
			RawExpires:  cookie.RawExpires,
			MaxAge:      cookie.MaxAge,
			Secure:      cookie.Secure,
			HttpOnly:    cookie.HttpOnly,
			SameSite:    cookie.SameSite,
			Partitioned: cookie.Partitioned,
			Raw:         cookie.Raw,
			Unparsed:    cookie.Unparsed,
		})
	}
	if cookie, err := incoming.Cookie("organizr_user_uuid"); err == nil {
		req.AddCookie(&http.Cookie{
			Name:        cookie.Name,
			Path:        cookie.Path,
			Value:       cookie.Value,
			Quoted:      cookie.Quoted,
			Domain:      cookie.Domain,
			Expires:     cookie.Expires,
			RawExpires:  cookie.RawExpires,
			MaxAge:      cookie.MaxAge,
			Secure:      cookie.Secure,
			HttpOnly:    cookie.HttpOnly,
			SameSite:    cookie.SameSite,
			Partitioned: cookie.Partitioned,
			Raw:         cookie.Raw,
			Unparsed:    cookie.Unparsed,
		})
	}
	return req, nil
}

func buildClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 5,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
