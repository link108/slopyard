package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"
)

func Fingerprint(r *http.Request, secret string, trustProxyHeaders bool, now time.Time) string {
	ip := clientIP(r, trustProxyHeaders)
	ua := r.UserAgent()
	day := now.UTC().Format("2006-01-02")

	sum := sha256.Sum256([]byte(ip + "\x00" + ua + "\x00" + secret + "\x00" + day))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if ip != "" {
				return ip
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
