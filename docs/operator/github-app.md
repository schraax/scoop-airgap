---
layout: default
title: GitHub App Setup
parent: Operator Guide
nav_order: 5
---

# GitHub App Setup
{: .no_toc }

GitHub Apps are the recommended authentication method for scoop-airgap in production. They issue short-lived tokens (maximum 1 hour), carry precisely scoped permissions, and produce an audit trail that identifies the app rather than a person. No long-lived credential is stored in plaintext.

**When to use a GitHub App instead of a PAT:**
- The bucket repositories are on GitHub.com or GitHub Enterprise Server
- You want to enforce the principle of least privilege
- Your organisation does not allow long-lived personal access tokens in CI/CD workloads
- `pull_request: true` is enabled and you want PRs attributed to the app, not a user

If the bucket repositories are on Gitea or Forgejo, use `auth_token` instead — GitHub Apps are not supported on those platforms.

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## 1. Create the GitHub App

### GitHub.com

1. Open **Settings → Developer settings → GitHub Apps** and click **New GitHub App**.
2. Fill in the required fields:

   | Field | Value |
   |---|---|
   | **GitHub App name** | e.g. `scoop-airgap-bot` (must be globally unique) |
   | **Homepage URL** | URL of your internal docs or this repo (required, not used by the tool) |
   | **Webhook — Active** | **Uncheck** — scoop-airgap does not use webhooks |

3. Under **Repository permissions**, set the permissions shown in [§ 3 Permissions](#3-permissions).
4. Under **Where can this GitHub App be installed?** select **Only on this account**.
5. Click **Create GitHub App**.

### GitHub Enterprise Server

Follow the same steps, replacing `github.com` with your GHES hostname throughout.
The app settings page lives at `https://github.corp.com/settings/apps/new`.

---

## 2. Note the App ID

The **App ID** is shown near the top of the app's settings page immediately after creation. Save this number — it is needed in `config.yaml`.

---

## 3. Permissions

Grant only what scoop-airgap actually uses. All other permissions must remain **No access**.

| Permission | Access required | When |
|---|---|---|
| **Contents** | **Read and write** | Always — needed to push commits or the PR branch |
| **Pull requests** | **Read and write** | Only when `pull_request: true` |

{: .highlight }
If you are using direct-push mode (`pull_request: false`, the default), leave **Pull requests** at **No access**.

---

## 4. Generate a private key

1. Scroll to the bottom of the app's settings page.
2. Click **Generate a private key**.
3. A `.pem` file downloads automatically. Store it securely — it cannot be retrieved again.

GitHub generates PKCS#1 RSA keys. scoop-airgap also accepts PKCS#8 keys if you have converted the key for other tooling.

---

## 5. Install the app

1. On the app's settings page click **Install App** in the left sidebar.
2. Choose the account or organisation that owns the bucket repositories.
3. Under **Repository access** select **Only select repositories**.
4. Add each repository that scoop-airgap pushes manifests into (one per bucket entry in your config).
5. Click **Install** (or **Save**).

Limiting the installation to specific repositories means a compromised token can only affect those repositories.

---

## 6. Find the Installation ID

After installation you are redirected to the installation's settings page. The **Installation ID** is the number at the end of the URL:

```
https://github.com/settings/installations/78901234
                                           ^^^^^^^^
                                           installation_id
```

On GHES:
```
https://github.corp.com/settings/installations/78901234
```

---

## 7. Configure scoop-airgap

### Option A — private key via environment variable

Recommended for containers, Kubernetes, and CI/CD pipelines.

Export the PEM contents as an environment variable, preserving newlines:

```bash
export GITHUB_APP_PRIVATE_KEY="$(cat /path/to/private-key.pem)"
```

`config.yaml`:

```yaml
git:
  github_app:
    app_id: 123456
    installation_id: 78901234
    private_key: "${GITHUB_APP_PRIVATE_KEY}"
```

### Option B — private key as a file

Recommended for VMs and Kubernetes secret volume mounts.

```yaml
git:
  github_app:
    app_id: 123456
    installation_id: 78901234
    private_key_path: /run/secrets/github-app.pem
```

In a Kubernetes deployment, mount the secret as a volume:

```yaml
volumes:
  - name: gh-app-key
    secret:
      secretName: scoop-airgap-github-app
      items:
        - key: private-key.pem
          path: private-key.pem
volumeMounts:
  - name: gh-app-key
    mountPath: /run/secrets
    readOnly: true
```

### GitHub Enterprise Server

Add `api_url` pointing to your GHES REST API root. This routes both the App token exchange and PR creation to the GHES endpoint instead of `api.github.com`.

```yaml
git:
  api_url: https://github.corp.com/api/v3
  github_app:
    app_id: 123456
    installation_id: 78901234
    private_key: "${GITHUB_APP_PRIVATE_KEY}"
```

### Pull request mode

If you also want scoop-airgap to open pull requests instead of pushing directly, add `pull_request: true`. The **Pull requests: Read and write** permission must be granted (see [§ 3](#3-permissions)).

```yaml
git:
  pull_request: true
  github_app:
    app_id: 123456
    installation_id: 78901234
    private_key: "${GITHUB_APP_PRIVATE_KEY}"
```

---

## 8. Verify

Run with `-dry-run` to confirm authentication works without making any changes:

```bash
scoop-airgap -config config.yaml -dry-run
```

A successful run prints the apps that *would* be synced and exits without error. If authentication fails you will see an error such as:

- `401 Unauthorized` — the App ID or private key is wrong
- `404 Not Found` — the installation ID is wrong, or the app is not installed on the target repository
- `403 Forbidden` — the required permission (`Contents: Read and write`) is missing

---

## Token lifecycle

scoop-airgap fetches a fresh installation token at the start of every run. The token is valid for up to one hour and is not cached to disk. For a typical sync run of a few minutes this means the token never approaches its expiry. No rotation or renewal logic is needed.

The private key itself is long-lived and must be rotated manually. Generate a new key in the app's settings, update the secret or environment variable, and delete the old key only after confirming the new key works.
