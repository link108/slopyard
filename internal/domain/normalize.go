package domain

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

var ErrInvalidHost = errors.New("invalid host")

type NormalizedHost struct {
	Host              string
	RegistrableDomain string
}

func NormalizeHost(input string) (NormalizedHost, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return NormalizedHost{}, ErrInvalidHost
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return NormalizedHost{}, ErrInvalidHost
	}

	host := parsed.Hostname()
	if host == "" {
		return NormalizedHost{}, ErrInvalidHost
	}

	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if net.ParseIP(host) != nil {
		return NormalizedHost{}, ErrInvalidHost
	}

	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return NormalizedHost{}, ErrInvalidHost
	}
	ascii = strings.TrimSuffix(strings.ToLower(ascii), ".")
	if ascii == "" || strings.ContainsAny(ascii, "/?#@:") {
		return NormalizedHost{}, ErrInvalidHost
	}

	if strings.HasPrefix(ascii, "www.") {
		ascii = strings.TrimPrefix(ascii, "www.")
	}

	registrable, err := publicsuffix.EffectiveTLDPlusOne(ascii)
	if err != nil {
		return NormalizedHost{}, ErrInvalidHost
	}

	return NormalizedHost{
		Host:              ascii,
		RegistrableDomain: registrable,
	}, nil
}
