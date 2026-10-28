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

### Docker Image

```bash
docker pull statuscake/statuscake-mcp
```

## Signing

The binaries on the [releases
page](https://github.com/StatusCakeDev/statuscake-mcp/releases) are signed with
the signing key `DDEA0314FAE6420CA54CB80D9F8705905758331F`.

### Public Key

```text
-----BEGIN PGP PUBLIC KEY BLOCK-----

mDMEauKIPxYJKwYBBAHaRw8BAQdATGYPDtbEk25PRHeOojl6bn11f8wP9Gm9NT+H
BwBY6B+0RVN0YXR1c0Nha2UgKENvZGUgc2lnbmluZyBmb3Igc3RhdHVzY2FrZS5j
b20pIDxzaWduaW5nQHN0YXR1c2Nha2UuY29tPoiZBBMWCgBBFiEE3eoDFPrmQgyl
TLgNn4cFkFdYMx8FAmriiD8CGwMFCQX6nYAFCwkIBwICIgIGFQoJCAsCBBYCAwEC
HgcCF4AACgkQn4cFkFdYMx+8iQEAr5lnryZE09h3MHhK+O1jqk83ThL4AJLcgCSG
FETIl5YA/j2YAUtP7TfnY+xP0g3UVagT9LY2j83G2S/TlRDfhpMIuDgEauKIPxIK
KwYBBAGXVQEFAQEHQDKjCcdzlwsDYLlAAGszcondKuVtWfv3QufPl0hen6RTAwEI
B4h+BBgWCgAmFiEE3eoDFPrmQgylTLgNn4cFkFdYMx8FAmriiD8CGwwFCQX6nYAA
CgkQn4cFkFdYMx+tmgEAgC+E6x61nxHsmenUcxsXA+Zia07NWSsX0xkBab+oPN8A
/j8UfoJyozzbxq6gUNrTKi1sFDZtGeevB540wkakm9IL
=Knl0
-----END PGP PUBLIC KEY BLOCK-----
```

### Verifying Binaries

1. Import the signing key:

   ```bash
   gpg --keyserver keyserver.ubuntu.com --recv-keys DDEA0314FAE6420CA54CB80D9F8705905758331F
   ```

1. Verify that the key has been imported:

   ```bash
   gpg --list-keys signing@statuscake.com
   ```

   Check the output is:

   ```bash
   pub   ed25519 2026-10-28 [SC] [expires: 2030-01-01]
         DDEA0314FAE6420CA54CB80D9F8705905758331F
   ```

1. Download the binary and its corresponding shasums and signature from the
   [releases page](https://github.com/StatusCakeDev/statuscake-mcp/releases).

1. Verify the signature of the shasums file:

   ```bash
   gpg --verify statuscake-mcp_x.y.z_SHA256SUMS.sig statuscake-mcp_x.y.z_SHA256SUMS
   ```

   This should give you a similar output to that below:

<!-- pyml disable md013 -->
   ```bash
   gpg: Signature made Sun Feb  8 15:30:24 2026 GMT
   gpg:                using EDDSA key DDEA0314FAE6420CA54CB80D9F8705905758331F
   gpg:                issuer "signing@statuscake.com"
   gpg: Good signature from "StatusCake (Code signing for statuscake.com) <signing@statuscake.com>" [ultimate]
   ```
<!-- pyml enable md013 -->

1. Verify the shasum of the binary matches the value in the shasums file:

   ```bash
   sha256sum -c --ignore-missing statuscake-mcp_x.y.z_SHA256SUMS
   ```

   This should give you a similar output to that below:

   ```bash
   statuscake-mcp_x.y.z_linux_amd64.tar.gz: OK
   ```

## License

This project is licensed under the [Mozilla Public License](LICENSE).
