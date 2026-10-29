<!--
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
-->

# raynet-cli

> [!IMPORTANT]
> `raynet` is an official RAYNET tool, but an early proof of concept. It comes with no support
> guarantee: we respond on a best-effort basis, and commands may change before 1.0.

`raynet` is a command-line client for the RAYNET CRM REST API v2. It has commands for every major
CRM entity and a raw `raynet api` passthrough for the rest of the API. The design borrows from
[GitLab's CLI](https://gitlab.com/gitlab-org/cli) (entity commands on top of an `api` escape hatch,
`-f`/`-F` body fields) and [Stripe's CLI](https://github.com/stripe/stripe-cli) (ad-hoc credential
flags, `--verbose` request logging).

The CLI documents itself: `raynet --help` gives an overview, `raynet <entity> create --help` lists an
entity's fields, and `raynet help filtering|fields|output` covers the shared syntax. This README is
the tour.

## Install

### Release archive

[GitHub Releases](https://github.com/raynetcrm/raynet-cli/releases) has an archive for linux, darwin
(macOS) and windows, each for amd64 and arm64, plus `checksums.txt`. On Linux or macOS, download
one, verify it (run the `sha256sum` line on Linux, the `shasum` one on macOS) and put `raynet` on
your `PATH`:

```sh
VERSION=0.1.0   # the release, without the leading v
OS=darwin       # or linux
ARCH=arm64      # or amd64
ARCHIVE="raynet_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/raynetcrm/raynet-cli/releases/download/v${VERSION}"

curl -fsSLO "$BASE/$ARCHIVE"
curl -fsSLO "$BASE/checksums.txt"
grep " $ARCHIVE\$" checksums.txt | sha256sum -c -        # Linux; prints "<archive>: OK"
grep " $ARCHIVE\$" checksums.txt | shasum -a 256 -c -    # macOS; prints "<archive>: OK"
tar -xzf "$ARCHIVE" raynet
sudo install raynet /usr/local/bin/
raynet --version
```

On Windows, download `raynet_<version>_windows_<arch>.zip` from the release page, check its SHA-256
against `checksums.txt` (`Get-FileHash`), and unzip `raynet.exe` to a directory on your `PATH`.

**macOS:** the binaries are not signed or notarized. A `curl` download isn't quarantined, so the
commands above just work. An archive downloaded in a browser is, and macOS refuses to run `raynet`
from it; clear the flag after unpacking:

```sh
xattr -d com.apple.quarantine raynet
```

### `go install`

With Go (the version in [go.mod](go.mod)) installed:

```sh
go install github.com/raynetcrm/raynet-cli/cmd/raynet@latest
```

This puts `raynet` in `$(go env GOPATH)/bin`. A `go install` build reports its version as `dev`.

### From source

The repo pins its Go toolchain and `golangci-lint` with [mise](https://mise.jdx.dev/):

```sh
git clone https://github.com/raynetcrm/raynet-cli.git
cd raynet-cli
mise install
make build      # -> bin/raynet
```

## Feedback

Report bugs and ideas in [GitHub Issues](https://github.com/raynetcrm/raynet-cli/issues). Pull
requests aren't accepted. Report security vulnerabilities privately, as described in
[SECURITY.md](SECURITY.md).

## Authenticate

```sh
raynet auth login                 # a wizard, in a terminal
raynet auth login --host app.raynet.cz --auth-type apikey \
  --username jan@acme.cz --api-key "$RAYNET_API_KEY" --instance-name acme
raynet auth login --host app.raynet.cz --auth-type apikey \
  --username jan@acme.cz --api-key "$RAYNET_API_KEY" --instance-id 1a2b3c
raynet auth login --host app.raynet.cz --auth-type bearer --token "$RAYNET_TOKEN"
raynet auth status                # list stored profiles
raynet user-account me            # who and which instance the credentials resolve to
```

There are two auth types: `apikey` (username + API key, plus an instance name or instance id) and
`bearer` (a bearer token, which already carries the instance, username and API key; stored and sent
exactly as given). `raynet auth token` mints a bearer token from API-key credentials.

In a terminal, `auth login` is a wizard: host (a known host or "Other…"), login method (bearer
token, instance id + username + API key, or instance name + username + API key) and that method's
fields, with the API key and bearer token hidden while typed. Every flag given skips its step; the
login method is picked only by `--auth-type`, `--instance-name` or `--instance-id`. Set
`RAYNET_CLI_ACCESSIBLE=1` for plain line-by-line prompts (for screen readers) instead of menus.

`auth login` checks the credentials with `GET /security/info` before it saves anything. If the
check fails, the wizard asks whether to save anyway; without a terminal nothing is saved and it
exits non-zero. Without a terminal it never prompts: a missing value is an error naming its flag
(`missing --api-key`). On bearer and instance-id profiles it also stores the instance name the API
reports, which `--web` links need. The first profile becomes the default; a later one with
`--default`, or when you answer yes to the wizard's "make default?".

Known hosts: `app.raynet.cz`, `app.raynetcrm.sk`, `app.raynetcrm.com`, `eu.raynetcrm.com`.

### Profiles

Credentials are stored as named profiles in `~/.config/raynet-cli/config.yml` (mode `0600`, honoring
`$XDG_CONFIG_HOME`, on macOS too); `raynet auth config` prints the path. A profile's name is
independent of its host, so two profiles can target two instances on the same host:

```sh
raynet auth login --profile acme --host app.raynet.cz --auth-type apikey \
  --username jan@acme.cz --api-key XXXX --instance-name acme --default
raynet auth login --profile widgets --host app.raynet.cz --auth-type apikey \
  --username jan@acme.cz --api-key YYYY --instance-name widgets

raynet --profile widgets company list
RAYNET_CLI_PROFILE=widgets raynet company list
raynet company list               # the default profile
```

Without `--profile`, `auth login` names the profile after the instance the credentials resolve to;
the wizard suggests that name and asks before replacing an existing profile. With `--profile`, or
without a terminal, an existing profile with the same name is replaced.

### Ad-hoc credentials

`--api-key` or `--token` run a single call without a stored profile and always win over it. `--host`
is required with them and selects nothing else:

```sh
raynet --host app.raynet.cz --username jan@acme.cz --api-key XXXX --instance-name acme company list
raynet --host app.raynet.cz --token XXXX api /company/
```

## Commands

| Command | Aliases | Actions | Nested collections |
|---|---|---|---|
| `company` | | lock, unlock, invalidate, revalidate, tag, untag | `address` (+ set-primary, set-contact), `relationship` |
| `person` | | lock, unlock, invalidate, revalidate, tag, untag | `relationship` (+ set-primary; no list) |
| `lead` | | lock, unlock, convert, tag, untag | |
| `business-case` | `businessCase`, `bc` | lock, unlock, invalidate, revalidate, tag, untag | `item` |
| `offer` | | as business-case, plus sync, unsync | `item` |
| `sales-order` | `salesOrder`, `so` | as offer | `item` |
| `project` | | lock, unlock, invalidate, revalidate, tag, untag | |
| `invoice` | `invoiceLight` | lock, unlock, tag, untag | |
| `product` | | invalidate, revalidate, tag, untag | |
| `price-list` | `priceList`, `pl` | lock, unlock, tag, untag | |
| `task`, `meeting`, `event`, `email`, `phone-call` (`phoneCall`, `pc`), `letter` | | lock, unlock, tag, untag | |

Each of these has `list`, `view`, `create`, `update` and `delete`. In addition:

- `activity list`: every activity type in one list, with a `type` column naming the command to act
  on a row with (`raynet phone-call view 42`).
- `user-account` (`userAccount`) `list|view|me`: find the user whose `person.id` goes into `owner`
  fields; `me` shows who the credentials resolve to.
- `codelist`: list codelists and manage their entries (see [Codelists](#codelists)).
- `auth login|status|config|token`: profiles and bearer tokens.
- `api`: any REST API endpoint (see [Raw API access](#raw-api-access)).

Every record argument takes a numeric id or `ext:<extId>`, an external id; for anything but `view`,
the CLI first resolves it with `GET /<path>/ext/<extId>/`.

### List

```sh
raynet company list --fulltext acme --sort -name
raynet person list --filter 'contactInfo.email[LIKE]=%@example.com' --offset 30
raynet company list --filter 'rowInfo.createdAt[GE]=2026-01-01 00:00' --filter rating=A
raynet company list --all --columns id,name,owner.fullName,rowInfo.createdAt
raynet activity list --sort -scheduledFrom
```

`list` fetches 30 records; `--limit N` fetches up to 1000, `--offset N` pages and `--all` fetches
every page (1000 records per request). In a terminal it reports the total on stderr, e.g. `30 of 319`. `--filter attr[OP]=value`
passes any REST API filter through, `--sort column` or `--sort -column` sorts, and `--fulltext`
searches like the CRM search box. `<entity> list --help` names useful filter attributes;
`raynet help filtering` lists every operator and the date formats.

`--columns` takes dotted paths into nested fields (`owner.fullName`, `company.name`); a flat API key
containing a dot (`rowInfo.createdAt`) wins over nesting.

### View, create, update, delete

```sh
raynet company view 123
raynet company view ext:erp-17
raynet company create -f name=Acme -f state=A_POTENTIAL -f rating=A -f role=A_SUBSCRIBER -F owner=7 --jq .data.id
raynet company create -f name=Acme -f state=A_POTENTIAL -f rating=A -f role=A_SUBSCRIBER \
  -f 'addresses[][address][name]=HQ' -f 'addresses[][address][city]=Brno'
raynet task create -f title='Send offer' -f priority=DEFAULT -F owner=7 -f 'deadline=2026-11-02 09:00'
raynet person update ext:crm-9 --input person.json
raynet company update 123 -F category=null
raynet company delete 123          # asks "Delete company 123? [y/N]" in a terminal
raynet company delete 123 --yes    # required outside a terminal
```

`create` and `update` build the JSON body from flags, the same way as `raynet api`:

- `-f key=value` adds a string field: text, enum values, dates (`yyyy-MM-dd`) and date-times
  (`yyyy-MM-dd HH:mm`).
- `-F key=value` adds a typed field: a number, `true`/`false`, `null`, or `@file` (the file's
  contents). Send ids, numbers and booleans with `-F`: the API stores `false` for the string
  `"true"`.
- Keys nest with brackets: `owner[id]`, `tags[]` appends, `items[][count]` sets a field of the last
  array element (or starts a new one).
- `--input file.json` (or `-` for stdin) is the base body; `-f`/`-F` apply on top.

`<entity> create --help` and `update --help` list the entity's key fields with type, required marker,
allowed values and the codelist or entity an id points to. The list is help only: the API, not the
CLI, validates the body. On a business document (`business-case`, `offer`, `sales-order`), `create`
switches to the API's create-with-items endpoint when the body has `items`, so a document and its
line items are one call:

```sh
raynet offer create -f name=Q1 -F company=7 -F businessCase=9 -f 'items[][productCode]=002' -F 'items[][count]=7'
```

`create`, `update`, `delete` and actions print the API's JSON response on stdout (`--jq .data.id`
gives a new record's id) and, in a terminal only, a confirmation such as `Created company 123` on
stderr. `delete` asks for confirmation in a terminal and refuses without `--yes` otherwise.

### Actions

```sh
raynet company lock 123            # also unlock, invalidate, revalidate
raynet person tag ext:crm-9 vip hot
raynet person untag 456 hot
raynet lead convert 12 -F company=101 -F person=205
raynet offer sync 31               # link the offer's line items with its business case; also unsync
```

Each entity offers only the actions its API supports (see the table above). `tag`/`untag` take tag
names after the record argument. On activities, the `tags` field is an array on `create` but a
comma-separated string on `update`.

### Nested collections

```sh
raynet offer item list 31                      # also business-case, sales-order
raynet offer item add 31 -f productCode=A001 -F count=2
raynet offer item update 31 5 -F discountPercent=10
raynet offer item delete 31 5 6 7 --yes        # several ids: one bulk request
raynet company address list 123
raynet company address add 123 -f 'address[name]=HQ' -f 'address[city]=Brno' -f 'address[country]=CZ'
raynet company address set-primary 123 5       # also set-contact
raynet company relationship add 123 -F companyJoin=456 -f companyJoinType=parent
raynet person relationship add 789 -F company=123 -f type=CEO
raynet person relationship set-primary 789 5
```

Nested collections live under their record: `<entity> <collection> list|add|update|delete <id> ...`.
`add` and `update` take the body flags and list the fields in `--help`; `delete` follows the same
confirmation rules as a record `delete`.

- Line items (`item`) and addresses (`address`) are read from the record's `view` response, as the
  API has no list endpoint for them.
- A company's links to other companies (`relationship`) have their own list endpoint; `update`
  changes only the `notice`.
- A person's companies (`person relationship`) have no `list` (see `person view`), and `update` must
  resend `company` and `type`.

### Codelists

```sh
raynet codelist                                   # every codelist name
raynet codelist companyCategory                   # its entries (same as `... list`)
raynet codelist companyCategory create --name VIP --color B780FF --jq .data
raynet codelist currency create --name Euro -f code02=EUR
raynet codelist territory update 3 --name South
raynet codelist territory delete 3 --yes
```

Fields such as `category` or `territory` point to codelist entries by id. The API has no catalog
endpoint, so `raynet codelist` prints a static list. `codelist <name>` lists all entries at once (no
paging). `--name` sets `code01` and `--color` sets `code02` on codelists with a color; `-f`/`-F`/
`--input` work too, with `--name`/`--color` winning. Codelists offer only the verbs the API supports:
`businessCaseType` is read-only, and `taxRate` and the phase and status codelists have no `update`.
A create returns the new entry's id as `data`.

### Open in the web app

```sh
raynet company view 123 --web
raynet company view ext:erp-17 --web
raynet person list --web          # the list view
```

`--web` opens `{web base}/{instance}/redir?en=<Entity>&ei=<id>` (or the list view) with `$BROWSER`,
else the OS default browser. The web base is the profile's `--web-url`, else the API's scheme + host.
The instance name is the profile's, else the one `auth login` stored, else it is looked up with
`GET /security/info` on each call.

### Raw API access

Every REST API endpoint is reachable through `raynet api`, with `<path>` relative to `/api/v2` and
the same body flags. The API creates with `PUT` and updates with `POST`. For `GET` and `DELETE`, the
fields become query parameters and `--input` is the body. `--paginate` walks every page of a `GET`
list and returns one JSON array.

```sh
raynet api /company/ -f fulltext=acme --paginate
raynet api /company/ -X PUT -f name=Acme -f rating=A -f state=A_POTENTIAL -f role=A_SUBSCRIBER
raynet api /company/123/ -X POST --input body.json -F owner=7
```

## Output and errors

- `list` prints a table: styled in a terminal (unless `NO_COLOR` is set), plain tab-separated columns
  when piped. Output never switches to JSON on its own; pass `--json`.
- `view`, mutations and `api` print JSON, colored through `jq` in a terminal when `jq` is on `PATH`.
- `--jq '<expr>'` filters JSON output through `jq` (installed separately).
- Notes (`30 of 319`, `Created company 123`) go to stderr, and only in a terminal.

API errors go to stderr as the status and a message, with one indented line per validation error
(`field: message`, or just the message when it names no field), and the command exits non-zero.
The message is in your language when the API sends a usable translation, else the technical one:

```
Error: 400 Invalid record
  name: Name is required
```

With `--json` the error is JSON on stderr instead: `error`, `status`, `type`, `message`,
`translatedMessage` and `fieldErrors` (`[{field, message}]`, without `field` when there is none).
`-v`/`--verbose` prints each request line and response status to stderr, and the raw error body on
failure:

```sh
raynet -v company view 123
# > GET https://app.raynet.cz/api/v2/company/123/
# < 200 OK
```

## Development

`make check` runs fmt-check, vet, lint, test and build. Other targets: `make fmt`, `make vet`, `make
lint`, `make test`; see the [Makefile](Makefile). Domain terms are defined in
[GLOSSARY.md](GLOSSARY.md).

`raynet --version` prints the version embedded at build time via `-ldflags`; a local `make build`
reads `.version` if present, else `git describe`, else `dev`.
