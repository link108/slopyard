package server

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"slopyard/internal/domain"
	"slopyard/internal/store"
)

type Config struct {
	Addr              string
	FingerprintSecret string
	TrustProxyHeaders bool
	GlobalLimit       int
	GlobalWindow      time.Duration
	HostWindow        time.Duration
}

type Server struct {
	cfg       Config
	store     store.PostgresStore
	limiter   store.Limiter
	logger    *slog.Logger
	templates *template.Template
}

func New(cfg Config, pg store.PostgresStore, limiter store.Limiter, logger *slog.Logger) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"reportLabel":   reportLabel,
		"categoryLabel": categoryLabel,
		"formatTime":    formatTime,
		"sitePath":      sitePath,
	}).ParseGlob("web/templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:       cfg,
		store:     pg,
		limiter:   limiter,
		logger:    logger,
		templates: tmpl,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	if s.cfg.TrustProxyHeaders {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	r.Get("/", s.home)
	r.Get("/top-slops", s.topSlops)
	r.Get("/faqs", s.faqs)
	r.Post("/report", s.submitReport)
	r.Get("/lookup", s.lookup)
	r.Get("/site/{host}", s.site)
	r.Get("/healthz", s.healthz)

	return r
}

type pageData struct {
	Title string
	Error string
	Flash string
	Data  any
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "home.html", pageData{
		Title: "AI Slop Detector",
		Error: r.URL.Query().Get("error"),
		Flash: r.URL.Query().Get("flash"),
	})
}

func (s *Server) topSlops(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.HomeData(r.Context())
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Could not load top slops.")
		s.logger.Error("load top slops", "err", err)
		return
	}
	s.render(w, r, http.StatusOK, "top_slops.html", pageData{
		Title: "Top Slops",
		Data:  data,
	})
}

func (s *Server) faqs(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "faqs.html", pageData{
		Title: "FAQs",
	})
}

func (s *Server) submitReport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectAfterReportError(w, r, "", "Could not read the report form.")
		return
	}

	input := strings.TrimSpace(r.FormValue("input"))
	reportType := strings.TrimSpace(r.FormValue("type"))
	category := strings.TrimSpace(r.FormValue("category"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	if reportType != store.ReportSlop && reportType != store.ReportNotSlop {
		s.redirectAfterReportError(w, r, input, "Choose whether this site is AI slop or not slop.")
		return
	}

	normalized, err := domain.NormalizeHost(input)
	if err != nil {
		s.redirectAfterReportError(w, r, input, "Enter a valid public website host.")
		return
	}

	fingerprint := domain.Fingerprint(r, s.cfg.FingerprintSecret, s.cfg.TrustProxyHeaders, time.Now())
	if err := s.limiter.CheckGlobal(r.Context(), fingerprint); err != nil {
		s.handleLimitError(w, r, normalized.Host, err, "Too many reports. Try again in a minute.")
		return
	}

	release, err := s.limiter.ReserveHost(r.Context(), fingerprint, normalized.Host)
	if err != nil {
		s.handleLimitError(w, r, normalized.Host, err, "You already reported this host recently.")
		return
	}
	success := false
	defer func() {
		if !success {
			release(contextWithoutCancel(r.Context()))
		}
	}()

	_, err = s.store.CreateReport(r.Context(), store.ReportInput{
		SiteHost:          normalized.Host,
		RegistrableDomain: normalized.RegistrableDomain,
		SubmittedInput:    input,
		Type:              reportType,
		Category:          category,
		Notes:             notes,
		FingerprintHash:   fingerprint,
	})
	if err != nil {
		s.logger.Error("create report", "host", normalized.Host, "err", err)
		s.redirectAfterReportError(w, r, normalized.Host, "Could not save the report.")
		return
	}
	success = true

	http.Redirect(w, r, sitePath(normalized.Host)+"?flash="+url.QueryEscape("Report counted."), http.StatusSeeOther)
}

func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	input := strings.TrimSpace(r.URL.Query().Get("input"))
	normalized, err := domain.NormalizeHost(input)
	if err != nil {
		s.redirectHome(w, r, "Website not found.", "")
		return
	}

	_, err = s.store.GetSiteDetail(r.Context(), normalized.Host)
	if errors.Is(err, store.ErrNotFound) {
		if !hostResolves(r.Context(), normalized.Host) {
			s.redirectHome(w, r, "Website not found.", "")
			return
		}
		if _, err := s.store.EnsureSite(r.Context(), normalized.Host, normalized.RegistrableDomain); err != nil {
			s.logger.Error("create searched site", "host", normalized.Host, "err", err)
			s.redirectHome(w, r, "Could not save that website.", "")
			return
		}
	} else if err != nil {
		s.logger.Error("lookup site", "host", normalized.Host, "err", err)
		s.redirectHome(w, r, "Could not search websites.", "")
		return
	}

	http.Redirect(w, r, sitePath(normalized.Host), http.StatusSeeOther)
}

func (s *Server) site(w http.ResponseWriter, r *http.Request) {
	rawHost := chi.URLParam(r, "host")
	normalized, err := domain.NormalizeHost(rawHost)
	if err != nil {
		s.renderError(w, r, http.StatusBadRequest, "That host is not valid.")
		return
	}

	detail, err := s.store.GetSiteDetail(r.Context(), normalized.Host)
	if errors.Is(err, store.ErrNotFound) {
		s.renderError(w, r, http.StatusNotFound, "Website not found.")
		return
	}
	if err != nil {
		s.logger.Error("load site", "host", normalized.Host, "err", err)
		s.renderError(w, r, http.StatusInternalServerError, "Could not load this host.")
		return
	}

	s.render(w, r, http.StatusOK, "site.html", pageData{
		Title: detail.Site.Host,
		Error: r.URL.Query().Get("error"),
		Flash: r.URL.Query().Get("flash"),
		Data:  detail,
	})
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleLimitError(w http.ResponseWriter, r *http.Request, host string, err error, message string) {
	if errors.Is(err, store.ErrRateLimited) {
		s.redirectAfterReportError(w, r, host, message)
		return
	}
	s.logger.Error("rate limit", "err", err)
	s.redirectAfterReportError(w, r, host, "Could not check rate limits.")
}

func (s *Server) redirectHome(w http.ResponseWriter, r *http.Request, errorMessage string, flash string) {
	values := url.Values{}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	if flash != "" {
		values.Set("flash", flash)
	}
	target := "/"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) redirectAfterReportError(w http.ResponseWriter, r *http.Request, input string, message string) {
	normalized, err := domain.NormalizeHost(input)
	if err != nil {
		s.redirectHome(w, r, message, "")
		return
	}
	http.Redirect(w, r, sitePath(normalized.Host)+"?error="+url.QueryEscape(message), http.StatusSeeOther)
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	s.render(w, r, status, "error.html", pageData{Title: http.StatusText(status), Error: message})
}

func (s *Server) render(w http.ResponseWriter, _ *http.Request, status int, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("render template", "template", name, "err", err)
	}
}

func reportLabel(reportType string) string {
	switch reportType {
	case store.ReportSlop:
		return "AI Slop"
	case store.ReportNotSlop:
		return "Not Slop"
	default:
		return reportType
	}
}

func categoryLabel(category string) string {
	labels := map[string]string{
		"SEO_SPAM":         "SEO spam",
		"LOW_EFFORT":       "Low effort",
		"CONTENT_FARM":     "Content farm",
		"FAKE_REVIEWS":     "Fake reviews",
		"MISINFO":          "Misinformation",
		"USEFUL_AI":        "Useful AI",
		"HUMAN_AUTHORED":   "Human-authored",
		"CLEAR_DISCLOSURE": "Clear disclosure",
	}
	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("Jan 2, 2006 3:04 PM")
}

func sitePath(host string) string {
	return "/site/" + url.PathEscape(host)
}

func hostResolves(ctx context.Context, host string) bool {
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := net.DefaultResolver.LookupHost(lookupCtx, host)
	return err == nil
}

func contextWithoutCancel(ctx context.Context) context.Context {
	return withoutCancel{ctx}
}

type withoutCancel struct {
	context.Context
}

func (withoutCancel) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (withoutCancel) Done() <-chan struct{} {
	return nil
}

func (withoutCancel) Err() error {
	return nil
}

func (c withoutCancel) Value(key any) any {
	return c.Context.Value(key)
}
