# Independent workflow runtime architecture

This repository implements a small n8n-style workflow runtime from scratch. It uses LMScript for validation and orchestration, LMCode for typed graph execution, and a Go host for bounded HTTP transport. It contains no n8n implementation code.

## Repository boundary

The testfoss/n8n fork retains the complete upstream project, credential-storage research, examples, and integration tests. This independent repository does not import or sync that source tree.

The workflow JSON format uses nodes, connections, parameters, and credential references as data. The current implementation targets a narrow n8n-style subset. It does not claim general n8n export compatibility.

Connector specifications use public provider documentation. The Telegram specification covers sendMessage and getChat with JSON POST bodies and a bot token in the URL path. The n8n field mappings are data bindings, not copied implementation code.

## Execution path

1. src/validate.lms validates workflow and connector JSON against schema documents.
2. LMScript checks unique node identity, supported node types, operation mappings, required parameters, credential references, and linear main connections.
3. src/run.lms compiles the LMCode program against the exact host schema bytes.
4. LMCode follows the connection graph and stores each node response as JSON text.
5. Whole-field expressions read the previous node response through the $json path subset.
6. The connector specification maps n8n-style parameter names to provider field names.
7. The Go host reads credential values by ID and field. The workflow contains no credential value.
8. The Go host performs a bounded JSON POST and returns typed status and JSON values.
9. LMCode validates the HTTP status and provider success envelope.
10. The host returns the final node response to LMScript through the LMCode result contract.

## Supported expressions

The only expression form is a complete string in this shape:

```text
={{ $json.path.to.value }}
```

The runtime does not evaluate JavaScript. It does not support string interpolation, function calls, arithmetic, array indexes, or references to arbitrary named nodes. Values retain their scalar JSON types.

The integration workflow uses a numeric manual chat ID of 41. The sendMessage response changes the chat ID to 42. getChat must receive numeric 42. This proves that the second operation reads the previous response rather than the original input.

## Host contract

The host schema declares input, credential, base_url, json_put, http_post_json, and workflow_fail functions. HttpResponse exposes status and json methods. The result is a nonnullable Json value.

The transport uses a ten-second timeout, one MiB request and response limits, strict JSON parsing, and no redirects. Error messages do not include request URLs. This matters because Telegram puts the credential in the path.

The host uses the exact schema bytes for compiler binding, runtime loading, signature inspection, and result validation. The lmmock schema is also pinned by exact bytes.

## Intentional exclusions

- Branching, loops, disconnected graphs, and multiple outputs.
- General JavaScript expressions and arbitrary named-node references.
- OAuth, secret encryption, credential sharing, and persistent credential storage.
- Webhooks, schedules, retries, queues, execution history, and a graphical editor.
- Production connector coverage beyond the declared Telegram subset.

Add these capabilities through reviewed contracts. Do not describe the MVP as a complete n8n replacement.

## Validation record

On 2026-11-18, npm test passed locally. This included LMScript static checks, Go unit tests, host build, schema and registry parity, a missing-credential control, and the complete Telegram workflow through lmmock.

The integration path ran manual trigger, sendMessage, expression propagation, and getChat. No live provider or real credential was used.
