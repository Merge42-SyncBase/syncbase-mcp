# SyncBase MCP

공식 MCP Streamable HTTP transport, bearer 인증, host/origin 제한 및 `search_documents` 도구를 소유한다.

MCP는 WAS 내부 패키지를 import하지 않고 `github.com/Merge42-SyncBase/syncbase-was/searchruntime`만 사용한다.

```sh
go test ./...
go vet ./...
```
