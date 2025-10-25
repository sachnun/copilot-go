# Copilot API (Go Port)

> [!WARNING]
> This project reverse-engineers private GitHub Copilot endpoints. Excessive or automated usage can trigger GitHub’s abuse detection and may violate the [Acceptable Use Policies](https://docs.github.com/site-policy/acceptable-use-policies/github-acceptable-use-policies) and [Copilot Terms](https://docs.github.com/site-policy/github-terms/github-terms-for-additional-products-and-features#github-copilot). Use at your own risk.

A minimal Golang port of the original Node/Bun-based `copilot-api`, exposing reverse-engineered GitHub Copilot endpoints through a single Go binary or container with OpenAI/Anthropic-compatible routes, streaming support, optional API key protection, and matching CLI commands.

## Quick Start

```bash
# build and run locally
go run ./cmd/copilot-api start --port 4141

# build docker image
docker build -t copilot-go .
docker run --rm -p 4141:4141 \
  -v $(pwd)/copilot-data:/root/.local/share/copilot-api \
  -e API_KEY=your_secret_key \
  copilot-go
```

Data directory stores GitHub/Copilot tokens; keep it safe. `API_KEY` is optional; if set, clients must supply either `Authorization: Bearer <API_KEY>` or `x-api-key: <API_KEY>`.

> **Warning:** This project accesses internal GitHub Copilot APIs. Use responsibly; you might violate GitHub’s terms.

## CLI

```bash
copilot-api start [flags]       # start proxy
copilot-api auth [flags]        # force GitHub auth flow
copilot-api check-usage         # print Copilot quota summary
```

Relevant flags: `--verbose`, `--manual`, `--rate-limit`, `--wait`, `--github-token`, `--proxy-env`, `--show-token`, `--account-type`.

## Configuration

- `API_KEY` (optional) → enforce Bearer / x-api-key authentication.
- `GH_TOKEN` (optional) → supply GitHub token instead of interactive auth.
- proxies/HTTP via system environment if `--proxy-env` is set.

## License

See [LICENSE](./LICENSE).
