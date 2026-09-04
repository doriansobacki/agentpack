# Microsoft Entra ID identity provider

The `entra` provider resolves who a user is and which groups they belong to
from Microsoft Entra ID (formerly Azure AD). Group membership comes from the
directory, so the org manifest no longer needs a `users:` map — adding someone
to a group in Entra is all it takes for their next `agentpack sync` to pick up
the right packs.

## How it works

- Users sign in via the OAuth 2.0 **device authorization grant**: `agentpack
  sync` (or a first interactive sync after `agentpack login --source ...`)
  prints a verification URL and a one-time code; the user completes sign-in in
  any browser, on any device.
- The resolved **email** comes from the ID token (`preferred_username`, or
  `upn` as fallback) — with this provider, `agentpack login` needs no email
  argument.
- **Groups** come from the ID token's `groups` claim (group object IDs). When
  the token cannot carry them — the claim is not configured, or the user is in
  more than ~200 groups and Entra signals an overage — the provider falls back
  to Microsoft Graph `GET /v1.0/me/memberOf`.
- Tokens are cached in `<agentpack home>/entra-token-cache.json` (0600), so
  subsequent syncs are silent until the refresh token expires. Encrypting the
  cache via the OS keychain/DPAPI is a planned follow-up.
- A **non-interactive** sync (e.g. from a scheduler) never starts a
  device-code prompt: when the cached sign-in has expired it fails with an
  instruction to run `agentpack sync` from a terminal (or `agentpack login`)
  instead of hanging on a prompt nobody sees.

## Creating the app registration

In the Entra admin center (Identity → Applications → App registrations):

1. **New registration.** Name it something like `agentpack`; supported account
   types: *Accounts in this organizational directory only*. No redirect URI is
   needed for the device-code flow.
2. Under **Authentication**, set **Allow public client flows** to **Yes**
   (this enables the device authorization grant).
3. Under **API permissions**, the default delegated `User.Read` (Microsoft
   Graph) is required. Optionally add delegated **`GroupMember.Read.All`** and
   grant admin consent — needed only if your tenant restricts `/me/memberOf`,
   which the provider calls for groups overage or display-name resolution.
4. Under **Token configuration → Add groups claim**, select **Security
   groups** (or the group set that matches your manifest) for the **ID**
   token. Without this claim every sync makes a Graph call, which still works
   but is slower.

Note the **Directory (tenant) ID** and the **Application (client) ID** from
the app's Overview page; the manifest needs both.

## Org manifest configuration

```yaml
# agentpack.yaml
identity:
  provider: entra
  tenant_id: 00000000-0000-0000-0000-000000000000   # Directory (tenant) ID
  client_id: 11111111-1111-1111-1111-111111111111   # Application (client) ID
  # resolve_display_names: true   # optional, see below

targets: [claude, agentsmd]

groups:
  "*": [org-baseline]
  # Group keys may be Entra display names ...
  Platform Engineering: [platform-core]
  # ... or group object IDs (work offline from token claims alone):
  22222222-2222-2222-2222-222222222222: [dotnet]
```

### Group naming: display names vs. object IDs

The resolved identity's groups are the **union** of group object IDs and
(when a Microsoft Graph lookup ran) group display names, so `groups:` keys may
use either form:

- **Display names** are the recommended, readable form. They require a Graph
  lookup: set `resolve_display_names: true` to force one on every sync (it
  otherwise happens only on groups overage or a missing claim).
- **Object IDs** always work from token claims alone — no Graph call, no extra
  permissions — and are immune to a group being renamed.

## Developer experience

```sh
# Once: point agentpack at the org config. No email — it comes from sign-in.
agentpack login --source https://github.com/example/org-config

# First sync prints the device-code instructions:
agentpack sync
#   To sign in, use a web browser to open the page
#   https://microsoft.com/devicelogin and enter the code ABCD-EFGH ...

# Subsequent syncs are silent (cached token, silent refresh).
agentpack sync
```
