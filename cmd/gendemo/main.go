// gendemo generates a demo migrations directory in one of four layouts.
//
// Usage: gendemo [-layout flat|release|schema|env] [-dir ./migrations]
//
// Every layout produces the same logical database using all four file extensions:
//
//	.up.sql    - versioned, transactional
//	.r.sql     - repeatable, transactional
//	.notx.sql  - versioned, non-transactional
//	.rnotx.sql - repeatable, non-transactional
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	layout := flag.String("layout", "flat", "layout: flat | release | schema | env")
	dir := flag.String("dir", "./migrations", "output directory")
	flag.Parse()

	if err := os.RemoveAll(*dir); err != nil {
		log.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	fmt.Printf("layout: %s -> %s\n\n", *layout, *dir)

	g := &gen{dir: *dir}
	switch *layout {
	case "flat":
		g.flat()
	case "release":
		g.release()
	case "schema":
		g.schema()
	case "env":
		g.env()
	default:
		log.Fatalf("unknown layout %q (choose: flat | release | schema | env)", *layout)
	}

	fmt.Printf("\n%d files written.\n", g.n)
}

type gen struct {
	dir string
	rev int
	n   int
}

func (g *gen) up(sub, name, sql string)    { g.write(sub, name, "up.sql", sql) }
func (g *gen) rep(sub, name, sql string)   { g.write(sub, name, "r.sql", sql) }
func (g *gen) notx(sub, name, sql string)  { g.write(sub, name, "notx.sql", sql) }
func (g *gen) rnotx(sub, name, sql string) { g.write(sub, name, "rnotx.sql", sql) } //nolint:unparam

func (g *gen) write(sub, name, kind, sql string) {
	g.rev++
	filename := fmt.Sprintf("%07d-%s.%s", g.rev, name, kind)
	path := filepath.Join(g.dir, sub, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(sql+"\n"), 0o644); err != nil { //nolint:gosec
		log.Fatalf("write %s: %v", path, err)
	}
	fmt.Printf("  %s\n", filepath.ToSlash(filepath.Join(sub, filename)))
	g.n++
}

func (g *gen) flat() {
	g.up("", "schemas", sqlSchemas)
	g.up("", "ref-countries", sqlCountries)
	g.up("", "ref-statuses", sqlStatuses)
	g.up("", "app-projects", sqlProjects)
	g.up("", "app-members", sqlMembers)
	g.rep("", "v-project-summary", sqlView)
	g.rep("", "fn-project-members", sqlFunc)
	g.notx("", "indexes", sqlIndexes)
	g.rnotx("", "vacuum", sqlVacuum)
}

func (g *gen) release() {
	g.up("rel-1.0", "schemas", sqlSchemas)
	g.up("rel-1.0", "ref-countries", sqlCountries)
	g.up("rel-1.0", "ref-statuses", sqlStatuses)
	g.up("rel-1.0", "app-projects", sqlProjects)
	g.notx("rel-1.0", "indexes", sqlIndexesProjects)
	g.up("rel-2.0", "app-members", sqlMembers)
	g.rep("rel-2.0", "v-project-summary", sqlView)
	g.rep("rel-2.0", "fn-project-members", sqlFunc)
	g.notx("rel-2.0", "indexes", sqlIndexesMembers)
	g.rnotx("rel-2.0", "vacuum", sqlVacuum)
}

func (g *gen) schema() {
	g.up("ref", "schemas", sqlSchemas)
	g.up("ref", "countries", sqlCountries)
	g.up("ref", "statuses", sqlStatuses)
	g.up("app", "projects", sqlProjects)
	g.up("app", "members", sqlMembers)
	g.rep("views", "v-project-summary", sqlView)
	g.rep("views", "fn-project-members", sqlFunc)
	g.notx("indexes", "app", sqlIndexes)
	g.rnotx("indexes", "vacuum", sqlVacuum)
}

func (g *gen) env() {
	g.up("base", "schemas", sqlSchemas)
	g.up("base", "ref-countries", sqlCountries)
	g.up("base", "ref-statuses", sqlStatuses)
	g.up("base", "app-projects", sqlProjects)
	g.up("base", "app-members", sqlMembers)
	g.rep("base", "v-project-summary", sqlView)
	g.rep("base", "fn-project-members", sqlFunc)
	g.notx("base", "indexes", sqlIndexes)
	g.rnotx("base", "vacuum", sqlVacuum)
	g.up("dev", "seed", sqlSeedDev)
}

const sqlSchemas = `
create schema if not exists app;
create schema if not exists ref;`

const sqlCountries = `
create table ref.countries (
    code char(2) primary key,
    name text    not null
);

insert into ref.countries (code, name) values
    ('US', 'United States'),
    ('DE', 'Germany'),
    ('JP', 'Japan'),
    ('GB', 'United Kingdom');`

const sqlStatuses = `
create table ref.statuses (
    id    int  primary key,
    label text not null unique
);

insert into ref.statuses values
    (1, 'active'),
    (2, 'inactive'),
    (3, 'pending');`

const sqlProjects = `
create table app.projects (
    id         bigint      generated always as identity primary key,
    name       text        not null,
    status_id  int         not null references ref.statuses(id) default 1,
    country    char(2)     references ref.countries(code),
    created_at timestamptz not null default now()
);`

const sqlMembers = `
create table app.members (
    id         bigint      generated always as identity primary key,
    project_id bigint      not null references app.projects(id) on delete cascade,
    email      text        not null,
    role       text        not null default 'member',
    joined_at  timestamptz not null default now(),
    unique (project_id, email)
);`

const sqlView = `
create or replace view app.v_project_summary as
select
    p.id,
    p.name,
    s.label       as status,
    c.name        as country,
    count(m.id)   as member_count
from app.projects p
join      ref.statuses  s on s.id   = p.status_id
left join ref.countries c on c.code = p.country
left join app.members   m on m.project_id = p.id
group by p.id, p.name, s.label, c.name;`

const sqlFunc = `
create or replace function app.fn_project_members(p_project_id bigint)
    returns setof app.members language sql stable as $$
    select * from app.members where project_id = p_project_id;
$$;`

const sqlIndexesProjects = `
create index concurrently if not exists idx_app_projects_status  on app.projects (status_id);
create index concurrently if not exists idx_app_projects_country on app.projects (country);`

const sqlIndexesMembers = `
create index concurrently if not exists idx_app_members_email   on app.members (email);
create index concurrently if not exists idx_app_members_project on app.members (project_id);`

const sqlIndexes = sqlIndexesProjects + "\n" + sqlIndexesMembers

const sqlVacuum = `
vacuum analyze app.projects;
vacuum analyze app.members;`

const sqlSeedDev = `
insert into app.projects (name, country) values
    ('Alpha', 'US'),
    ('Beta',  'DE'),
    ('Gamma', 'JP');`
