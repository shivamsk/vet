---
sidebar_position: 2
title: 🚀 Quick Start
---

# 🚀 Quick Start

- Download the binary file for your operating system/architecture from the [Official GitHub Releases](https://github.com/safedep/vet/releases)

![vet Github Releases](/img/vet/vet-github-releases.png)

- Get an API key for the vet insights data access for performing the scan

```bash
vet auth trial --email john.doe@example.com
```

![vet register trial](/img/vet/vet-register-trial.png)

:::info

A time limited trial API key will be sent over email.

:::

- Configure `vet` to use API key to access the insights

```bash
vet auth configure
```

![vet configure](/img/vet/vet-configure.png)

:::tip

Insights API is used to enrich OSS packages with metadata for rich query and policy decisions. Alternatively, the API key can be passed through environment variable `VET_API_KEY`

:::

- You can verify the configured key is successful by running the following command

```bash
vet auth verify
```

- Run `vet` to identify risks

```bash
vet scan -D /path/to/repository
```

![vet scan directory](/img/vet/vet-scan-directory.png)

- You can also scan a specific (supported) package manifest

```bash
vet scan --lockfiles /path/to/pom.xml
vet scan --lockfiles /path/to/requirements.txt
vet scan --lockfiles /path/to/package-lock.json
```

![vet scan files](/img/vet/vet-scan-files.png)
