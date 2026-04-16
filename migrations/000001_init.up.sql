create extension if not exists pgcrypto;

create table if not exists sites (
  id uuid primary key default gen_random_uuid(),
  host text unique not null,
  registrable_domain text not null,

  first_reported_at timestamptz not null default now(),
  last_reported_at timestamptz not null default now(),

  total_reports integer not null default 0 check (total_reports >= 0),
  slop_count integer not null default 0 check (slop_count >= 0),
  not_slop_count integer not null default 0 check (not_slop_count >= 0),

  hidden_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists reports (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,

  submitted_input text not null,
  normalized_host text not null,

  type text not null check (type in ('SLOP', 'NOT_SLOP')),
  category text,
  notes text,

  fingerprint_hash text not null,
  removed_at timestamptz,

  created_at timestamptz not null default now()
);

create index if not exists idx_reports_site_id on reports(site_id);
create index if not exists idx_reports_fingerprint on reports(fingerprint_hash);
create index if not exists idx_reports_created_at on reports(created_at desc);
create index if not exists idx_sites_host on sites(host);
create index if not exists idx_sites_last_reported_at on sites(last_reported_at desc);

create or replace function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

drop trigger if exists trg_sites_updated_at on sites;
create trigger trg_sites_updated_at
before update on sites
for each row
execute function set_updated_at();
