ACP JSON schema (protocol v1), vendored for conformance tests (data only).
Source: https://github.com/agentclientprotocol/agent-client-protocol, tag schema-v1.25.0,
files schema/v1/schema.json, schema/v1/schema.unstable.json, schema/v1/meta.json.
Pick the newest `schema-v1.x.y` tag (the repo also tags crate releases `vX.Y.Z`
and v2 alphas `schema-v2.*`). Refresh: re-run Task 2 Step 1 of
docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md and fix whatever `go test ./internal/acp` reports.

## Feature gates

- `messageId` on ContentChunk: present (optional, `MessageId` ref)
- `usage_update` (UsageUpdate: used, size, cost{amount,currency}): present in stable schema.json
- def names used by the plan (SessionNotification, InitializeResponse, PromptResponse, RequestPermissionRequest, ToolCall, ToolCallUpdate, ContentChunk, SessionConfigOption): present, unrenamed
- ToolKind / ToolCallStatus / StopReason / PermissionOptionKind: contain every value moca emits
