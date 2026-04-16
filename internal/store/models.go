package store

import (
	"context"
	"time"
)

const (
	ReportSlop    = "SLOP"
	ReportNotSlop = "NOT_SLOP"
)

type Site struct {
	ID                string
	Host              string
	RegistrableDomain string
	FirstReportedAt   time.Time
	LastReportedAt    time.Time
	TotalReports      int
	SlopCount         int
	NotSlopCount      int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (s Site) SlopPercent() int {
	if s.TotalReports == 0 {
		return 0
	}
	return int(float64(s.SlopCount)/float64(s.TotalReports)*100 + 0.5)
}

type ReportInput struct {
	SiteHost          string
	RegistrableDomain string
	SubmittedInput    string
	Type              string
	Category          string
	Notes             string
	FingerprintHash   string
}

type RecentReport struct {
	Host      string
	Type      string
	Category  string
	CreatedAt time.Time
}

type CategoryCount struct {
	Category string
	Count    int
}

type SiteDetail struct {
	Site              Site
	RecentReports     []RecentReport
	CategoryBreakdown []CategoryCount
	Last24Hours       int
	Last7Days         int
}

type HomeData struct {
	RecentReports []RecentReport
	Trending24h   []Site
	Trending7d    []Site
}

type PostgresStore interface {
	EnsureSite(ctx context.Context, host string, registrableDomain string) (Site, error)
	CreateReport(ctx context.Context, input ReportInput) (Site, error)
	GetSiteDetail(ctx context.Context, host string) (SiteDetail, error)
	HomeData(ctx context.Context) (HomeData, error)
	Close()
}
