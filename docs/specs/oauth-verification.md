# OAuth verification record (phase 7 — §3 policy gate)

Retrieved: **2026-10-06** (every URL below was fetched live on this date; quoted text is verbatim from the
sources). Gate: DESIGN.md §3 — before any OAuth code is written for a provider, verify (a) that the vendor's
current terms permit subscription OAuth from third-party clients and (b) how a third-party developer registers
a client. `unclear` is treated as `not permitted`. No workarounds, no spoofed client identities.

Result summary:

| Provider | Terms | Own client id | Decision |
|---|---|---|---|
| `anthropic` | **not permitted** | none exists | **api_key only** — OAuth dropped, config rejects it with this reason |
| `openai` | **permitted** (Sign in with ChatGPT, open-source token sharing) | yes — dynamic registration, no secret | **ship oauth** (openai-responses route) |

---

## 1. anthropic — decision: `api_key only` (subscription OAuth not permitted)

### 1.1 Terms

Primary source: <https://code.claude.com/docs/en/legal-and-compliance> (retrieved 2026-10-06), section
"Usage policy → Authentication and credential use":

> **OAuth authentication** is intended exclusively for purchasers of Claude Free, Pro, Max, Team, and Enterprise
> subscription plans and is designed to support ordinary use of Claude Code and other native Anthropic applications.
> …
> **Developers** building products or services that interact with Claude's capabilities, including those using the
> Agent SDK, should use API key authentication through Claude Console or a supported cloud provider. **Anthropic
> does not permit third-party developers to offer Claude.ai login into their own applications, or to route requests
> through Free, Pro, or Max plan credentials on behalf of their users.** Moreover, developers may not collect,
> store, or intermediate Claude.ai credentials or session tokens — sign-in to a Claude account must complete
> through Anthropic's own flow.
> …
> Anthropic reserves the right to take measures to enforce these restrictions and may do so without prior notice.

**Conclusion: not permitted.** moca is neither Claude Code nor Claude.ai; offering a "sign in with Claude"
OAuth flow and routing requests through consumer-plan credentials is exactly what the clause forbids — including
for Anthropic's own Agent SDK, so no third-party tool is exempt.

Corroborating timeline (checked 2026-10-06):

- 2026-01-09: Anthropic deploys server-side checks blocking third-party tools from authenticating with
  subscription OAuth (`"This credential is only authorized for use with Claude Code and cannot be used for other
  API requests."`); account bans reported, later partially reversed.
- 2026-02-18/19: the clause above appears in the Claude Code legal docs; press confirms it as formal policy
  (The Register 2026-02-20; WinBuzzer 2026-02-19). OpenCode removes Claude subscription support citing
  "anthropic legal requests".
- 2026-04-04: Claude subscriptions stop covering third-party tool usage; affected users offered a one-time
  credit and optional discounted "extra usage bundles" (an Anthropic-side billing product).
- 2026-09-13: coverage reaffirms the ban is current ("Anthropic Halts Third-Party OAuth Access Across Claude
  Subscriptions").

The April "extra usage bundles" do not create a third-party OAuth contract: they are a billing option on the
user's Anthropic account, and the credential rules quoted above still apply. moca therefore does not implement
OAuth against Anthropic on any basis.

### 1.2 Client registration

There is no registration path for third-party consumer-plan OAuth. The only working client identities belong to
Anthropic's own clients; using one would be spoofing — prohibited by this gate and by Anthropic's terms.

### 1.3 Decision

**`api_key only`.** Enforced in config validation: `providers.anthropic.auth: "oauth"` is rejected with the
recorded reason; the §3 table drops the OAuth option; §12/README updated (DESIGN rev 12).

---

## 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)

### 2.1 Terms

Primary source: <https://developers.openai.com/siwc/> and the open-source flow pages under
<https://developers.openai.com/siwc/token-sharing-open-source/> (retrieved 2026-10-06):

> **For open-source developers:** Let users run AI workloads in your tools with their ChatGPT plan, without
> requiring them to provide an API key. The open-source sign-in flow registers your client and issues OAuth
> credentials for eligible Responses API requests, without a client secret or partner API key.

Quickstart page, same source:

> Sign in with ChatGPT is currently offered to a select group of commercial partners. **ChatGPT plan usage is
> available to all open-source partners and selected private clients.**

OpenAI Help Center (<https://help.openai.com/en/articles/20001542-using-your-chatgpt-plan-in-other-apps-and-sites>,
retrieved 2026-10-06):

> **Sign in with ChatGPT** lets you sign in to participating apps and sites with your ChatGPT account. … you can
> also choose to use your ChatGPT plan for AI requests in participating commercial apps and sites that support
> this feature. … **Supported open-source tools remain available to all ChatGPT users.**

The partner directory (<https://learn.chatgpt.com/docs/sign-in-with-chatgpt>) lists OpenClaw, OpenCode, Pi and T3
under "Open-source integrations".

**Conclusion: permitted.** The flow moca implements is the documented open-source integration: the user
authorizes their own ChatGPT account on OpenAI's own consent screens; moca never sees credentials; OpenAI's
per-app usage limits govern consumption. Commercial-partner gating applies to commercial apps; moca is MIT
open source and uses the open-source flow, which the docs describe as self-service ("no client secret or partner
API key"). Note for the record: the flow is a preview with documented limitations (§2.4) — it may change; the
design keeps `api_key` as a first-class fallback (both remain available in config).

### 2.2 Client registration (per the docs)

- **First registration**: authorize with `client_id=dynamic_agent_client`, `agent_name_hint=moca` (consistent
  across installs), and a per-host `ext_agent_host_id`. The callback returns the **issued** client id
  (`oaiapp_…`); it is persisted with the account registration and reused for later sign-ins. The placeholder is
  never stored or used for token exchange.
- **No secret**: token-endpoint auth method `none` (confirmed in the live discovery document, §2.3).
- **Reauthorization**: reuse the issued client id (optionally with `id_token_hint` / `login_hint`); omit
  `agent_name_hint`. A callback returning a different client id must be rejected, not used to replace the
  registration.
- **Host identity**: a stable opaque `ext_agent_host_id` per host (`urn:uuid:…` supported; recommended
  `urn:ietf:params:oauth:jwk-thumbprint:…`). It is not a credential. One client id can serve multiple hosts;
  each host has its own id.

### 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`)

| Item | Value |
|---|---|
| authorize URL | `https://auth.openai.com/api/accounts/authorize` |
| token URL | `https://auth.openai.com/api/accounts/oauth/token` |
| revoke URL | `https://auth.openai.com/api/accounts/oauth/revoke` |
| JWKS | `https://auth.openai.com/.well-known/jwks.json` (RSA / RS256, `kid` present) |
| scopes | identity `openid profile email`; plan usage `offline_access resource.invoke chatgpt.tokens.use.direct` |
| resource | `https://api.openai.com/v1` |
| PKCE | S256 only (`code_challenge_methods_supported: ["S256"]`); `response_type=code`; `nonce` required |
| redirect URI | HTTP loopback on **`127.0.0.1`**, path fixed **`/auth/callback`** (e.g. `http://127.0.0.1:1455/auth/callback`); only the port may vary; `localhost` is rejected; identical URI in authorize + exchange |
| ID token | RS256; validate signature (JWKS), `iss=https://auth.openai.com`, `aud=issued client id`, `exp`, and the saved `nonce`; validated `sub` = account identity |
| token lifetimes | access token 3600 s; refresh token 30 days, **rotating** (each refresh returns a replacement; successive replacements unlimited while valid); response also carries `earliest_refresh_at` |
| refresh | POST token endpoint, form-encoded `grant_type=refresh_token`, issued `client_id`, saved `refresh_token`, `resource`; omit `scope` to retain the grant |
| logout | POST revoke with `token=<refresh_token>`, `token_type_hint=refresh_token`, issued `client_id`; empty 200 = success |

Live probes (2026-10-06):

- discovery + JWKS documents returned as above;
- token endpoint parses form bodies and answers `{"error":"invalid_client"}` for the unissued placeholder id —
  expected, since exchange must use the issued id (the docs' own contract);
- the authorize endpoint is live behind a Cloudflare bot challenge (HTTP 403 "Just a moment…" for curl), so the
  end-to-end interactive flow (browser consent → callback → exchange → first inference) is verified by the
  phase-7 live gate with a real ChatGPT account (Task 4 Step 5), not by a scripted probe.

### 2.4 Inference contract for subscription traffic (open-source flow docs)

- `POST https://api.openai.com/v1/responses`, `Authorization: Bearer <access token>`. Model slugs must be
  available to the signed-in account (`GET https://api.openai.com/v1/models`, entries with
  `visibility == "list"`; show `display_name`, pass `slug`). **Responses API only** — "do not point it at
  ChatGPT's `backend-api` endpoints".
- Required per request: `store: false`, `stream: true`; full history in `input` (no `previous_response_id`);
  system prompt via `instructions` (explicit `{"type":"message","role":"system"}` items are rejected).
- Unsupported fields (omit): `background`, `conversation`, `max_output_tokens`, `max_tool_calls`, `metadata`,
  `moderation`, `multi_agent`, `prompt`, `prompt_cache_retention`, `safety_identifier`, `temperature`,
  `top_logprobs`, `top_p`, `truncation`, `user`.
- Tools: function/custom tools are supplied **grouped in a namespace** (or via `additional_tools` input
  items). Unsupported: image generation, file search, code interpreter, native computer use, hosted MCP /
  connectors, `tool_search`, `programmatic_tool_calling` in top-level `tools`.
- Errors: usage limits arrive as `response.failed` with `subscription_sharing_usage_limit_exceeded` (429) /
  `subscription_sharing_usage_unavailable` (503) — stop, don't retry blindly; `subscription_sharing_user_not_eligible`
  (403) and `subscription_sharing_unsupported_capability` (400, inspect `error.param`) are terminal for the
  request; pre-stream admission can answer `{"detail":…}` with 401/403/503.
- Refresh errors that mean "clear and re-login with the saved client id": `invalid_grant`,
  `invalid_refresh_token`, `token_expired`, `refresh_token_expired`, `refresh_token_invalidated`,
  `refresh_token_reused`; `invalid_client` = client configuration problem.
- Credential storage (docs' example record): email, issuer, subject, issued `client_id`, `ext_agent_host_id`,
  retained `id_token`, access + refresh tokens, scopes, saved-at — owner-only permissions, atomic writes, never
  logged or committed. Rotating refresh tokens **must be serialized across processes** ("so two processes do
  not race a rotating token").

### 2.5 Decision

**`ship oauth`** for the `openai` provider, on the `openai-responses` protocol, implementing exactly the
open-source flow above. `api_key` remains supported and is the fallback if the preview changes.

---

## 3. Implementation deltas vs. the phase plan (recorded for the plan's Implementation notes)

The phase plan (`docs/plans/phase-7-oauth-release.md`) was drafted before this verification. Reality adjusts it:

1. **Anthropic**: no OAuth code; config validation + docs only (plan's "api_key only" branch — applied).
2. **OpenAI uses dynamic client registration** (plan assumed a fixed per-provider client id): the store keeps
   the issued client id per provider registration, and `moca login openai` uses
   `client_id=dynamic_agent_client` only when no registration exists.
3. **Host id**: persisted alongside the store (`ext_agent_host_id`, `urn:uuid:` form) and sent on every
   authorize attempt.
4. **Nonce + ID token validation**: the login flow validates state, nonce, `iss`, `aud`, `exp`, and the RS256
   signature against the JWKS (plan's flow had state only).
5. **Rotating refresh tokens**: the plan's cross-process lock across read → refresh → write is exactly what the
   SIWC docs require; refresh errors beyond `invalid_grant` (the SIWC list) map to the same terminal
   "run moca login openai" error.
6. **Responses request shaping on the OAuth route**: omit `max_output_tokens` (unsupported there), keep
   `store:false`/`stream:true` (already moca's shape), and wrap the seven function tools in one namespace
   (`moca`). The adapter applies this only when the credential is OAuth; API-key traffic is unchanged.
7. **Models on the subscription route** are the account-visible slugs; moca does not call `/v1/models` at
   runtime — the user declares models in config (catalog approach unchanged). Noted in README/SPECS.
8. **Single registration per provider (single-account simplification)**: moca keeps one openai registration in the store
   (no account picker). `moca login openai` while logged in = reauthorization with the saved client id;
   switching accounts = `moca logout openai` then `moca login openai` (fresh dynamic registration). Recorded in
   SPECS as a known limitation.
