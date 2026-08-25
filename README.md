# SyncBase MCP

공식 MCP Streamable HTTP transport, bearer 인증, host/origin 제한 및 `search_documents` 도구를 소유한다.

MCP는 WAS 내부 패키지를 import하지 않고 `github.com/Merge42-SyncBase/syncbase-was/searchruntime`만 사용한다.

```sh
go test ./...
go vet ./...
```

## License

SyncBase MCP의 자체 소스는 [Apache License 2.0](LICENSE)
(`Apache-2.0`)으로 배포합니다.
MCP Go SDK와 query-embedding runtime을 포함한 외부 구성요소는 각자의
라이선스를 따르며 세부 출처는
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)에 기록합니다.
