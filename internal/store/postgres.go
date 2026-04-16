package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) EnsureSite(ctx context.Context, host string, registrableDomain string) (Site, error) {
	var site Site
	err := p.pool.QueryRow(ctx, `
		insert into sites (host, registrable_domain)
		values ($1, $2)
		on conflict (host) do update set
			registrable_domain = excluded.registrable_domain
		returning id, host, registrable_domain, first_reported_at, last_reported_at,
			total_reports, slop_count, not_slop_count, created_at, updated_at
	`, host, registrableDomain).Scan(
		&site.ID,
		&site.Host,
		&site.RegistrableDomain,
		&site.FirstReportedAt,
		&site.LastReportedAt,
		&site.TotalReports,
		&site.SlopCount,
		&site.NotSlopCount,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return Site{}, err
	}
	return site, nil
}

func (p *Postgres) CreateReport(ctx context.Context, input ReportInput) (Site, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Site{}, err
	}
	defer tx.Rollback(ctx)

	slopIncrement := 0
	notSlopIncrement := 0
	if input.Type == ReportSlop {
		slopIncrement = 1
	} else {
		notSlopIncrement = 1
	}

	var site Site
	err = tx.QueryRow(ctx, `
		insert into sites (
			host, registrable_domain, first_reported_at, last_reported_at,
			total_reports, slop_count, not_slop_count
		)
		values ($1, $2, now(), now(), 1, $3, $4)
		on conflict (host) do update set
			last_reported_at = now(),
			total_reports = sites.total_reports + 1,
			slop_count = sites.slop_count + excluded.slop_count,
			not_slop_count = sites.not_slop_count + excluded.not_slop_count,
			updated_at = now()
		returning id, host, registrable_domain, first_reported_at, last_reported_at,
			total_reports, slop_count, not_slop_count, created_at, updated_at
	`, input.SiteHost, input.RegistrableDomain, slopIncrement, notSlopIncrement).Scan(
		&site.ID,
		&site.Host,
		&site.RegistrableDomain,
		&site.FirstReportedAt,
		&site.LastReportedAt,
		&site.TotalReports,
		&site.SlopCount,
		&site.NotSlopCount,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return Site{}, err
	}

	_, err = tx.Exec(ctx, `
		insert into reports (
			site_id, submitted_input, normalized_host, type, category, notes, fingerprint_hash
		)
		values ($1, $2, $3, $4, nullif($5, ''), nullif($6, ''), $7)
	`, site.ID, input.SubmittedInput, input.SiteHost, input.Type, input.Category, input.Notes, input.FingerprintHash)
	if err != nil {
		return Site{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Site{}, err
	}
	return site, nil
}

func (p *Postgres) GetSiteDetail(ctx context.Context, host string) (SiteDetail, error) {
	var detail SiteDetail
	err := p.pool.QueryRow(ctx, `
		select id, host, registrable_domain, first_reported_at, last_reported_at,
			total_reports, slop_count, not_slop_count, created_at, updated_at
		from sites
		where host = $1 and hidden_at is null
	`, host).Scan(
		&detail.Site.ID,
		&detail.Site.Host,
		&detail.Site.RegistrableDomain,
		&detail.Site.FirstReportedAt,
		&detail.Site.LastReportedAt,
		&detail.Site.TotalReports,
		&detail.Site.SlopCount,
		&detail.Site.NotSlopCount,
		&detail.Site.CreatedAt,
		&detail.Site.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteDetail{}, ErrNotFound
	}
	if err != nil {
		return SiteDetail{}, err
	}

	detail.Last24Hours, err = p.countReportsSince(ctx, detail.Site.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return SiteDetail{}, err
	}
	detail.Last7Days, err = p.countReportsSince(ctx, detail.Site.ID, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return SiteDetail{}, err
	}
	detail.RecentReports, err = p.recentReports(ctx, detail.Site.ID, 10)
	if err != nil {
		return SiteDetail{}, err
	}
	detail.CategoryBreakdown, err = p.categoryBreakdown(ctx, detail.Site.ID)
	if err != nil {
		return SiteDetail{}, err
	}

	return detail, nil
}

func (p *Postgres) HomeData(ctx context.Context) (HomeData, error) {
	recent, err := p.recentReports(ctx, "", 12)
	if err != nil {
		return HomeData{}, err
	}
	trending24h, err := p.trendingSince(ctx, time.Now().Add(-24*time.Hour), 10)
	if err != nil {
		return HomeData{}, err
	}
	trending7d, err := p.trendingSince(ctx, time.Now().Add(-7*24*time.Hour), 10)
	if err != nil {
		return HomeData{}, err
	}
	return HomeData{
		RecentReports: recent,
		Trending24h:   trending24h,
		Trending7d:    trending7d,
	}, nil
}

func (p *Postgres) countReportsSince(ctx context.Context, siteID string, since time.Time) (int, error) {
	var count int
	err := p.pool.QueryRow(ctx, `
		select count(*)
		from reports
		where site_id = $1 and removed_at is null and created_at >= $2
	`, siteID, since).Scan(&count)
	return count, err
}

func (p *Postgres) recentReports(ctx context.Context, siteID string, limit int) ([]RecentReport, error) {
	args := []any{limit}
	where := "s.hidden_at is null and r.removed_at is null"
	if siteID != "" {
		where += " and r.site_id = $2"
		args = append(args, siteID)
	}

	rows, err := p.pool.Query(ctx, `
		select s.host, r.type, coalesce(r.category, ''), r.created_at
		from reports r
		join sites s on s.id = r.site_id
		where `+where+`
		order by r.created_at desc
		limit $1
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []RecentReport{}
	for rows.Next() {
		var report RecentReport
		if err := rows.Scan(&report.Host, &report.Type, &report.Category, &report.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (p *Postgres) categoryBreakdown(ctx context.Context, siteID string) ([]CategoryCount, error) {
	rows, err := p.pool.Query(ctx, `
		select category, count(*)
		from reports
		where site_id = $1 and removed_at is null and category is not null
		group by category
		order by count(*) desc, category asc
		limit 8
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []CategoryCount{}
	for rows.Next() {
		var category CategoryCount
		if err := rows.Scan(&category.Category, &category.Count); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (p *Postgres) trendingSince(ctx context.Context, since time.Time, limit int) ([]Site, error) {
	rows, err := p.pool.Query(ctx, `
		select s.id, s.host, s.registrable_domain, s.first_reported_at, s.last_reported_at,
			s.total_reports, s.slop_count, s.not_slop_count, s.created_at, s.updated_at
		from sites s
		join reports r on r.site_id = s.id
		where s.hidden_at is null and r.removed_at is null and r.created_at >= $1
		group by s.id
		order by count(r.id) desc, s.last_reported_at desc
		limit $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sites := []Site{}
	for rows.Next() {
		var site Site
		if err := rows.Scan(
			&site.ID,
			&site.Host,
			&site.RegistrableDomain,
			&site.FirstReportedAt,
			&site.LastReportedAt,
			&site.TotalReports,
			&site.SlopCount,
			&site.NotSlopCount,
			&site.CreatedAt,
			&site.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}
