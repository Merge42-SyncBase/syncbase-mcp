# Third-party notices

SyncBase MCP is licensed under Apache-2.0. The following direct runtime
dependencies are not relicensed by SyncBase.

| Component | Version | Use | Upstream license | Source |
| --- | --- | --- | --- | --- |
| `github.com/Merge42-SyncBase/syncbase-was` | Pseudo-version pinned in `go.mod` | Public `searchruntime` facade | Apache-2.0, first-party sibling project | <https://github.com/Merge42-SyncBase/syncbase-was> |
| `github.com/google/uuid` | v1.6.0 | UUID values | BSD-3-Clause | <https://github.com/google/uuid/tree/v1.6.0> |
| `github.com/modelcontextprotocol/go-sdk` | v1.7.0 | MCP Streamable HTTP protocol implementation | Upstream transition license: Apache-2.0 for new/relicensed code, MIT for legacy contributions without relicensing consent, and CC-BY-4.0 for non-specification documentation | <https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/LICENSE> |

The MCP process also executes the first-party WAS search facade and local
query-embedding chain. Therefore its release binary inherits the third-party
runtime obligations documented by `syncbase-was` and `syncbase-embedding`,
including E5 (MIT), ONNX Runtime (MIT), and their Go dependencies.

`go.mod` and `go.sum` are the authoritative module pins. This file highlights
direct and cross-repository runtime dependencies; the final CycloneDX SBOM
must include all resolved transitive modules and preserve the MCP SDK's mixed
upstream license notice without simplifying it to a single license.
