Project: AI Slop Detector
Summary

AI Slop Detector is a lightweight, anonymous web application where users can report websites (by domain/host) as either:

“AI Slop”
“Not Slop”

The system aggregates reports per host and exposes simple, fast lookup pages showing community sentiment over time.

The goal is to build a fast, low-friction public utility — not a social network, not a moderation-heavy platform.

Core Principles
Keep it simple
No accounts (initially)
No comments
Minimal fields per submission
Fast + performant
Server-side normalization
Precomputed aggregates
Redis-backed rate limiting (optional but ready)
Abuse-resistant
Fingerprinting instead of raw IP usage
Rate limiting per fingerprint + host
Opinionated but neutral UI
Show “reported as slop by X users”
Avoid definitive claims
Core Concepts
1. Host (Primary Entity)

A normalized website host (domain or subdomain).

Examples:

example.com
blog.example.com

Normalization rules:

Lowercase
Strip path/query/fragment
Strip port
Normalize IDNs (punycode)
Collapse www. → apex domain (optional, enabled by default)
2. Report

A single user submission indicating sentiment about a host.

Types:

SLOP
NOT_SLOP

Optional metadata:

category (reason)
notes (optional, not displayed publicly in MVP)
3. Fingerprint

A hashed identifier used for abuse prevention.

Derived from:

IP
User-Agent
Server-side secret

Properties:

Not reversible
Rotatable (e.g., daily salt)
Used for rate limiting + uniqueness
Features (MVP)
1. Submit Report

User can:

Paste a URL or host
Select:
“AI Slop”
“Not Slop”
Optionally select a category

System:

Normalizes input → host
Creates/links to host
Stores report
Updates aggregates
2. Host Lookup Page

Route:

/site/:host

Displays:

Total reports
Slop vs Not Slop counts
Ratio / percentage
Recent activity (last 24h / 7d)
Category breakdown (optional)
3. Search / Lookup

Simple input:

Paste URL or host → redirect to normalized /site/:host
4. Trending / Recent (Basic)

Homepage:

Recently reported hosts
Most reported (last 24h / 7d)

(Leaderboard can evolve later)

5. Abuse Prevention
One report per fingerprint per host per 24h
Global rate limit per fingerprint
Optional CAPTCHA (Cloudflare Turnstile)
6. Admin (Minimal)
Remove report
Hide host
View basic stats

(No full moderation system initially)

Data Model (Postgres)
sites
id UUID PRIMARY KEY
host TEXT UNIQUE NOT NULL
registrable_domain TEXT NOT NULL

first_reported_at TIMESTAMP
last_reported_at TIMESTAMP

total_reports INT DEFAULT 0
slop_count INT DEFAULT 0
not_slop_count INT DEFAULT 0

created_at TIMESTAMP DEFAULT now()
updated_at TIMESTAMP DEFAULT now()
reports
id UUID PRIMARY KEY

site_id UUID REFERENCES sites(id)

submitted_input TEXT
normalized_host TEXT

type TEXT CHECK (type IN ('SLOP', 'NOT_SLOP'))

category TEXT
notes TEXT

fingerprint_hash TEXT

created_at TIMESTAMP DEFAULT now()
Indexes
CREATE INDEX idx_reports_site_id ON reports(site_id);
CREATE INDEX idx_reports_fingerprint ON reports(fingerprint_hash);
CREATE INDEX idx_sites_host ON sites(host);
Normalization Logic (Critical)

Single shared function:

normalize(input: string) -> {
  host: string,
  registrable_domain: string
}

Steps:

If no scheme → prepend http://
Parse URL
Extract hostname
Lowercase
Remove port
Normalize IDN → punycode
Collapse www. → apex domain
Extract registrable domain (using public suffix list)
Backend Design
API Endpoints
Submit Report
POST /api/report

Body:

{
  "input": "https://example.com/foo",
  "type": "SLOP",
  "category": "SEO_SPAM"
}
Get Site
GET /api/site/:host

Returns:

aggregated stats
recent activity
Search / Normalize
GET /api/lookup?input=...

Returns:

normalized host
redirect target
Rate Limiting (Redis)

Optional but recommended:

Keys:

rate:{fingerprint}
report:{fingerprint}:{host}

Rules:

Global: X requests / minute
Per host: 1 report / 24h
Aggregation Strategy

Update aggregates on write:

On new report:

increment:
total_reports
slop_count OR not_slop_count
update timestamps

Avoid heavy runtime queries.

Frontend (Simple)

Pages:

/
submit box
recent activity
trending sites
/site/:host
stats + sentiment
simple visual (bar or ratio)
Tech Stack
Backend
Node.js (Express or lightweight framework)
Postgres (primary DB)
Redis (rate limiting, optional)
Frontend
React (Next.js or simple SPA)
Minimal styling
Infra
Runs easily on your existing setup (k3s + Postgres + Redis)
Future Enhancements (Not MVP)
Leaderboards (top slop sites)
Time-series graphs
Browser extension (auto-report / quick report)
Trusted reporters (optional accounts later)
Domain-level aggregation
Public API
Open Questions (for later)
Should we weight reports over time? (decay older reports)
Should we collapse subdomains eventually?
Do we expose raw counts or normalized scores?
Do we allow report reversal (same user flipping vote)?
MVP Definition (Strict)

You are done when:

User can submit report
Host is normalized correctly
Host page shows aggregated stats
Rate limiting prevents spam
Homepage shows recent activity

Nothing more.
