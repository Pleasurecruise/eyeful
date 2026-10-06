# 登录与凭据

云端服务通过 GitHub App 让用户登录，也可以选配 Gitee OAuth；脚本用 API key 认证。eyeful 不保存任何密码。本地模式没有 eyeful 账号，桌面端和 `eyeful review` 都不需要登录（见[本地运行](LOCAL.md)）。实现遵循 [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/) 以及 OWASP 关于会话和 OAuth 的速查表。相关路由见 [API 约定](API.md#routes)。

| 谁       | 凭据            | 怎样发送                      |
| -------- | --------------- | ----------------------------- |
| 人       | GitHub 或 Gitee | 会话 cookie `eyeful_session`  |
| CI、脚本 | API key `eyf_…` | `Authorization: Bearer eyf_…` |

## 登录

GitHub 是必需的，需要配置 GitHub App 的 client ID 和 secret，缺少时服务端不会启动。Gitee 是可选的，它的 client ID 和 secret 必须同时配置。控制台通过 `GET /oauth/providers` 得知该显示哪些登录方式。

登录使用带 PKCE 的授权码流程，state cookie 的作用路径限定在对应提供方的路径下。账号必须有已验证的主邮箱：GitHub 上为 `verified`，Gitee 上为 `confirmed`，并需要 `primary` 权限。邮箱相同的账号会关联到同一个用户，所以同一个人可以用任意一种方式登录。

GitHub App 没有 OAuth scope 的概念。它申请仓库 Contents 和 Metadata 的只读权限，以及账号 Email addresses 的只读权限，用户安装时自己选择允许读取哪些仓库。Gitee 申请 `user_info emails`。每个账号都会记录提供方实际授予的权限。

## 保存的凭据

登录时拿到的 access token 和 refresh token 用 AES-256-GCM 加密保存，密钥为 `EYEFUL_TOKEN_KEY`。它们不写日志，也不发给控制台。GitHub 的 refresh token 只能用一次，所以刷新过期的 access token 时会加行锁。refresh token 也过期时，用户需要重新登录。

会话 token 和 API key 用 `crypto/rand.Text` 生成，只显示一次，保存的是 SHA-256 哈希。会话 cookie 设置了 `HttpOnly` 和 `SameSite=Lax`，除非设置了 `--insecure-cookies`，否则还带 `Secure`；有效期为 `--session-ttl`。

每个审查都属于创建它的用户，其他人看不到也改不了。

## 跨域访问 {#cross-origin-access}

控制台通常和服务端同源（内嵌在服务端里，或开发时经过 Vite 代理），不需要 CORS。控制台和服务端部署在不同源时：

- `EYEFUL_CORS_ORIGINS` 精确列出其他控制台的源，这些源可以携带 cookie（`Access-Control-Allow-Credentials`）。配置为 `*` 时启动报错，其他源不会收到 CORS 响应头。
- 用会话 cookie 认证的 `POST`、`PATCH`、`DELETE` 请求，`Origin` 必须等于 `EYEFUL_PUBLIC_URL` 的源或 `EYEFUL_CORS_ORIGINS` 中的某一个，否则返回 `403 origin_forbidden`。这用来挡住来自同级子域名的跨站请求伪造，`SameSite=Lax` 挡不住这种情况。用 API key 认证的请求不做这项检查，因为浏览器不会自动附带 API key。
- `SameSite=Lax` 的 cookie 只在同一站点的源之间发送，所以单独部署的控制台必须和服务端在同一个可注册域名下，比如 `console.example.com` 和 `api.example.com`。为其他站点上的控制台提供 `SameSite=None` 选项，计划中。

## 仓库访问 {#repository-access}

控制台只能读取当前用户授权给 GitHub App 的仓库，即用户能访问的每个安装中、用户能看到的每个仓库。访问其他仓库一律返回 404，公开仓库也一样。读取时使用用户自己的 token，所以 GitHub 会在应用权限之外再检查用户本人的权限。`GET /repositories` 会返回每个安装在 GitHub 上的设置页地址，用户在那里增删仓库。

计划中：工作进程在沙箱外用安装 token 克隆仓库，这需要应用的 ID 和私钥；模型密钥留在工作进程上的 pi 中。Gitee 仓库需要用户重新授权 `projects` 权限后读取，token 同样加密保存。本地模式下，用户的编码工具使用它们自己的登录（见 [agent](LOCAL.md#agents)）。

同样计划中：组织，以及清理过期的会话记录。
