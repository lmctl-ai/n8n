# Independent n8n-style workflows in LMScript and LMCode

[![LMScript, LMCode, Go, and lmmock CI](https://github.com/lmctl-ai/n8n/actions/workflows/ci.yml/badge.svg)](https://github.com/lmctl-ai/n8n/actions/workflows/ci.yml)

This repository is an independent, from-scratch workflow runtime. It uses [LMScript](https://www.lmscript.io/lmscript/) for validation and orchestration. It uses [LMCode](https://www.lmscript.io/lmcode/) for typed graph execution. A small Go host built with [lmcode-go](https://github.com/lmctl-ai/lmcode-go) supplies bounded JSON HTTP transport.

## Documentation

- [Quick reference](https://www.lmctl.com/tools/n8n/)
- [LLM skill](https://www.lmctl.com/tools/n8n/skill.html)
- [Guide](https://www.lmctl.com/tools/n8n/guide.html)
- [Reference](https://www.lmctl.com/tools/n8n/reference.html)

This is not an n8n fork. It contains no n8n implementation code. Neutral connector specifications use public provider contracts. Workflow JSON uses n8n-style nodes, connections, parameters, and credential references as data.

For the upstream n8n source, credential-storage research, and upstream-node integration tests, use [testfoss/n8n](https://github.com/testfoss/n8n). That repository intentionally retains the complete upstream project.

## Current scope

The first executable vertical slice runs this workflow:

```text
Manual Trigger -> Telegram sendMessage -> Telegram getChat
```

The getChat node reads the chat ID from the sendMessage response. The integration test changes the ID between those operations to prove that expression propagation works and preserves numeric JSON types.

The runtime supports:

- One manual trigger at the start of a linear workflow.
- One main output and one outgoing connection per node.
- n8n-style node IDs, names, types, parameters, and connections.
- Credential IDs in workflow JSON, with values in a separate credentials file.
- Whole-field expressions in the form `={{ $json.path.to.value }}`.
- Literal string, number, and Boolean connector inputs.
- Connector specifications with operation selection, node-field mapping, and credential placement.
- Bounded JSON POST requests through the custom LMCode host.
- Deterministic integration tests through [lmmock](https://github.com/lmctl-ai/lmmock).

This is an MVP, not a complete n8n replacement. It does not support branching, workflow loops, JavaScript expressions, OAuth, webhooks, schedules, retries, persistent execution history, or the full n8n node catalog.

## Architecture

```text
Workflow JSON + connector JSON + separate credentials JSON
                         |
                         v
             LMScript structural validation
             LMScript graph and input checks
                         |
                         v
                  run_lmcode(...)
                         |
                         v
       LMCode graph traversal and expression resolution
       LMCode operation mapping and response validation
                         |
                         v
          Go host: bounded outbound JSON transport
                         |
                         v
              Provider API or lmmock in tests
```

The workflow engine does not import n8n packages. Connector behavior comes from files such as [connectors/telegram.json](connectors/telegram.json), authored from the [public Telegram Bot API contract](https://core.telegram.org/bots/api).

Read [durable-memory/index.md](durable-memory/index.md) for the implementation decisions and validation record.

## Repository layout

| Path | Purpose |
| --- | --- |
| `src/validate.lms` | LMScript structural, connector, credential, and linear-graph validation |
| `src/run.lms` | CLI orchestration and the typed LMCode workflow program |
| `connectors/` | Neutral connector specifications and public documentation references |
| `examples/telegram.workflow.json` | Executable n8n-style workflow example |
| `schema/` | LMScript validation schemas for workflows and connector specifications |
| `internal/host/` | Host schema, credential/input adapters, JSON updates, and bounded HTTP |
| `cmd/lmflow-host/` | LMCode host CLI and invocation lifecycle |
| `tests/` | lmmock handlers, schema pins, fixtures, and integration assertions |
| `test.lms` | LMScript schema-parity, negative-validation, and end-to-end test driver |
| `durable-memory/` | Committed architecture and validation knowledge |

## Prerequisites

- Node.js 24.15 or later
- Go 1.26 or later
- Linux or macOS for the current build script

The npm lockfile pins the LMScript compiler and lmmock. `go.mod` pins the LMCode Go runtime. No global LMScript installation is required.

## Install and test

Install the pinned Node tools without lifecycle scripts:

```sh
npm ci --ignore-scripts --no-audit --no-fund
```

Run all static checks, Go unit tests, schema checks, and the lmmock workflow test:

```sh
npm test
```

The test suite needs no real credentials and no live provider access. lmmock chooses an ephemeral loopback port and closes after the test.

## Integration coverage

The main test command runs 63 LMScript-driven integration scenarios, in addition to the Go unit tests.

| Suite | Scenarios | Coverage |
| --- | ---: | --- |
| `tests/validation-cases.lms` | 32 | Workflow and connector schemas, node identity, credential references, operations, expressions, graph shape, and malformed JSON |
| `tests/runtime-cases.lms` | 26 | Literal and expression inputs, scalar types, credentials, provider responses, transport failures, graph cycles, and a nine-connector-node chain |
| `test.lms` | 5 | Host and lmmock schema parity, missing credentials, CLI usage, missing base-URL overrides, and the original workflow smoke test |

The reusable runtime cases include 98 explicit assertions. Each lmmock handler also checks the request path, content type, exact body fields, JSON scalar types, and values.

## CI

The public GitHub Actions workflow runs the complete test suite on both Ubuntu and macOS. It also runs Go vet and durable-memory validation. The workflow needs no repository secrets or live provider credentials.

## Run the example

```sh
npm run demo
```

The demo builds the host and executes [the Telegram example](examples/telegram.workflow.json) through the local mock. Its final response is the mocked getChat result:

```json
{
  "ok": true,
  "result": {
    "id": 42,
    "type": "private",
    "title": "Fixture Chat"
  }
}
```

The test credential in `tests/fixtures/credentials.json` is synthetic. It cannot authenticate a real Telegram bot.

## Add a connector operation

1. Read the provider's public API documentation or canonical OpenAPI specification.
2. Add or extend a JSON file under `connectors/`.
3. Declare the provider operation name, n8n-style resource and operation bindings, HTTP method, path, and parameter mappings.
4. Keep secret values out of the specification. Declare only the credential field name and placement.
5. Extend [schema/connector-v1.schema.json](schema/connector-v1.schema.json) only when the neutral contract needs a new capability.
6. Add a deterministic lmmock handler and end-to-end assertions.
7. Run `npm test`.

The current connector schema supports JSON POST operations with a credential placeholder in the path. Add a neutral authentication or transport contract before implementing another authentication shape.

## Credentials and transport safety

Workflow files contain credential IDs only. Supply credential values through a separate, untracked file:

```json
{
  "telegram-bot": {
    "token": "read-from-a-private-local-file"
  }
}
```

Use `credentials.local.json` for local development. That path is ignored by Git. Do not commit real credentials.

The HTTP host applies these limits:

- POST with `application/json` only.
- 10-second client timeout.
- 1 MiB request and response limits.
- No redirects.
- Strict JSON response parsing.
- Sanitized transport errors that do not include the request URL.

Telegram places the bot token in the URL path. Do not log resolved request URLs. Base URL overrides are trusted configuration; this host is not an SSRF sandbox.

For tests, `LMMOCK_URL` overrides the connector base URL. `LMFLOW_BASE_URL` supplies an explicit override for other controlled environments. `LMFLOW_HOST` changes the host executable path.

## Validation and static checks

```sh
npm run check
go vet ./...
go test ./...
npm run check:memory
```

The host exposes the standard LMCode inspection commands:

```sh
./build/lmflow-host --lmcode-schema --json
./build/lmflow-host --lmcode-signatures
```

The integration test compares the schema bytes and registry signatures with the compiler's view. It also compares the lmmock schema bytes with the committed pin.

## Toolchain references

| Component | Pin |
| --- | --- |
| LMScript and LMCode compiler | `@lmctl-ai/lmctl` 0.2.454 |
| lmmock | `@lmctl-ai/lmmock` 0.1.117 |
| LMCode Go runtime | `lmcode-go` commit `06cd9ae991d90fea9ad38f3b071da646c76832f4` |
| LMCode host schema | `dsl.schema` 2 |
| LMCode bytecode | DSLB v6 |

Useful references:

- [LMScript language guide](https://www.lmscript.io/lmscript/guide.html)
- [LMCode author guide](https://www.lmscript.io/lmcode/tutorial.html)
- [LMCode host SDK guide](https://www.lmscript.io/lmcode/hosts.html)
- [LMCode specification](https://github.com/lmctl-ai/lmcode)
- [lmcode-go runtime](https://github.com/lmctl-ai/lmcode-go)
- [lmcode-cpp runtime](https://github.com/lmctl-ai/lmcode-cpp)

## License

This independent implementation is available under the [MIT License](LICENSE). External tools and dependencies retain their own licenses. No n8n source code or upstream license-covered source is copied into this repository.
