# StatusCake MCP Server [![test](https://github.com/StatusCakeDev/statuscake-mcp/actions/workflows/test.yaml/badge.svg?event=push)](https://github.com/StatusCakeDev/statuscake-mcp/actions/workflows/test.yaml)

[![Support][support-badge]][support]

[support-badge]: https://img.shields.io/badge/support-statuscake--mcp-0B8FE6.svg?style=flat
[support]: https://developers.statuscake.com/guides/mcp/introduction

- Website: [statuscake.com](https://www.statuscake.com)
- Model Context Protocol: [modelcontextprotocol.io](https://modelcontextprotocol.io)
- Support: [developers.statuscake.com](https://developers.statuscake.com/guides/mcp/introduction)

A [Model Context Protocol](https://modelcontextprotocol.io) server that lets an
AI assistant look at your StatusCake monitoring. Ask "what is down?", "when did
the checkout API last fail?" or "why didn't that alert?" and the assistant reads
the answer from StatusCake.

The server is read-only. It cannot create, change, pause or delete anything, and
it never shows the addresses or numbers in a contact group.

## Prerequisites

You will need a StatusCake API token. You can view and manage your API tokens
from the [StatusCake account panel](https://app.statuscake.com/User.php).

## Installation

Grab the binary for your system from the [releases
page](https://github.com/StatusCakeDev/statuscake-mcp/releases).

Alternatively, install with Go:

```bash
go install github.com/StatusCakeDev/statuscake-mcp@latest
```

> [!NOTE]
> Installing from source using `go install` will build from the master branch
> and won't report version information via `statuscake-mcp --version`.

### Documentation

Full documentation is available on the [developer portal][support].

## License

This project is licensed under the [Mozilla Public License](LICENSE).
