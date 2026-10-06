# Sign-in and credentials

The cloud server signs people in through a GitHub App, or optionally through Gitee OAuth, and
authenticates scripts with API keys. eyeful stores no passwords. Local mode has no eyeful account:
the desktop app and `eyeful review` never sign in ([Running locally](LOCAL.md)). The implementation
follows [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/) and
the OWASP cheat sheets on sessions and OAuth. The routes are listed in [API](API.md#routes).

| Who         | Credential      | Sent as                         |
| ----------- | --------------- | ------------------------------- |
| Person      | GitHub or Gitee | Session cookie `eyeful_session` |
| CI, scripts | API key `eyf_…` | `Authorization: Bearer eyf_…`   |

## Sign-in

GitHub is required, configured as a GitHub App's client ID and secret, and the server does not start
without them. Gitee is optional; its client ID and secret must be set together.
`GET /oauth/providers` tells the console which providers to offer.

Sign-in uses the authorization code flow with PKCE, and a state cookie scoped to the provider's
path. The account must have a verified primary email: `verified` on GitHub, or `confirmed` with the
`primary` scope on Gitee. Accounts with the same email are linked, so a person can sign in with
either provider.

A GitHub App has no OAuth scopes. It asks for read-only access to repository Contents and Metadata
and to the account's Email addresses, and each user chooses which repositories it may read when
installing it. Gitee asks for `user_info emails`. Each account records the scopes its provider
granted.

## Stored credentials

The access and refresh tokens received at sign-in are encrypted with AES-256-GCM under
`EYEFUL_TOKEN_KEY`. They are not logged and not sent to the console. GitHub refresh tokens can be
used only once, so an expired access token is refreshed while holding a row lock. If the refresh
token has expired as well, the user signs in again.

Session tokens and API keys are generated with `crypto/rand.Text`, shown once, and stored as
SHA-256 hashes. The session cookie is `HttpOnly` and `SameSite=Lax`, is `Secure` unless
`--insecure-cookies` is set, and lasts for `--session-ttl`.

Every review belongs to the user who created it, and no one else can see or change it.

## Cross-origin access {#cross-origin-access}

The console normally shares the server's origin, either embedded or behind the Vite proxy, and needs
no CORS. When a console is deployed on a different origin from the server:

- `EYEFUL_CORS_ORIGINS` lists the other console origins exactly, and those origins may send cookies
  (`Access-Control-Allow-Credentials`). `*` is rejected at start-up, and no other origin receives
  CORS headers.
- A `POST`, `PATCH` or `DELETE` authenticated by the session cookie must carry an `Origin` equal to
  the origin of `EYEFUL_PUBLIC_URL` or one of `EYEFUL_CORS_ORIGINS`. Otherwise it gets
  `403 origin_forbidden`. This blocks cross-site request forgery from sibling subdomains, which
  `SameSite=Lax` would let through. Requests authenticated by an API key are not checked, since a
  browser cannot attach a key by itself.
- `SameSite=Lax` cookies are sent only between origins of the same site, so a separate console has
  to be on the same registrable domain, for example `console.example.com` with `api.example.com`. A
  `SameSite=None` option for consoles on another site is planned.

## Repository access {#repository-access}

The console reads only repositories that the signed-in user has given the GitHub App: every
installation the user can access, and every repository in it that the user can see. Any other
repository returns 404, even if it is public. Reads use the user's own token, so GitHub applies the
user's permissions on top of the app's. `GET /repositories` returns the GitHub settings page of each
installation, where the user adds or removes repositories.

Planned: the worker will clone with installation tokens outside the sandbox, which requires the
app's ID and private key, and model keys stay with pi on the worker. Gitee repositories will be read
after the user re-authorizes with the `projects` scope, with the token stored the same way. Locally,
the user's coding tools use their own sign-in ([Agents](LOCAL.md#agents)).

Also planned: organizations, and cleanup of expired session rows.
