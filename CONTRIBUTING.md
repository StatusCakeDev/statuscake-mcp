# How To Contribute

Contributions are welcome as we strive to make this application as useful as
possible for everyone. However time is not always on our side, and changes may
not be reviewed or merged in a timely manner.

If this application is found to be missing in functionality, please open an
issue describing the proposed change - discussing changes ahead of time reduces
friction within pull requests.

## Prerequisites

You will need the following things properly installed on your computer:

- [Git](https://git-scm.com/)
- [Go](https://go.dev/) (1.27+)
- [Make](https://www.gnu.org/software/make/) (4.0+)
- An MCP client, such as [Claude Code](https://claude.com/claude-code), to try
  the server with

## Installation

If you wish to work on the server, you'll first need [Go](https://go.dev/)
installed on your machine (the version named in `go.mod`). You'll also need to
correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as
adding `$GOPATH/bin` to your `$PATH`.

To compile the server, run `make build`. This will build the `statuscake-mcp`
binary, which can then be moved to the `$GOPATH/bin` directory.

```sh
git clone <repository-url>` this repository
cd statuscake-mcp
make build
mv statuscake-mcp $GOPATH/bin/statuscake-mcp
```

The server is started by an MCP client rather than by hand, so register the
compiled binary with your client, having replaced `<token>` with a StatusCake
API token. For Claude Code:

```sh
claude mcp add statuscake --env STATUSCAKE_API_TOKEN=<token> -- statuscake-mcp
```

Other clients take the same command and environment in their own format. Restart
the client after rebuilding the binary so that it starts the new one.

### Pointing the server at another API

`--custom-endpoint` (or `STATUSCAKE_CUSTOM_ENDPOINT`, or `custom_endpoint` in
the config file) exists only in development builds, made with `make build
TAGS=dev`. A release build has no such setting and ignores it if set. Tests do
not need the tag: they hand `adapter.Config` an `Endpoint`.

## Running Tests

- `make test` vets and tests both the release and the development build.

The tests talk to a local fake of the API, so they need no API token and make no
real requests. Run `make test TESTARGS="-run TestName"` to run a single test.

## SDK Boundary

Exactly one package, `internal/adapter`, imports the StatusCake SDK. Everything
else works with the models in `internal/domain`, and the adapter reports a
missing resource as `domain.ErrNotFound` and a rejected token as
`domain.ErrUnauthorized`. Please keep SDK types out of the other packages.

## Making Changes

For additional contributing guidelines visit
[devhandbook.org](https://devhandbook.org/contributing)
