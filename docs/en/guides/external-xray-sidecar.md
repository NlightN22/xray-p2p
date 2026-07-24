# External Xray health with xp2pdiag

An external Xray or 3x-ui server can participate in client-side health
selection without installing the full xp2p server. Run the lightweight
`xp2p diag` sidecar next to Xray and import the endpoint with the default
external-subscription settings.

The client sends readiness and ping requests through that endpoint's Xray
outbound. The sidecar advertises `xp2p-diag`; a successful ping is therefore a
complete health check. A full xp2p server continues to use
`xp2p-heartbeat`, which requires both ping and an authenticated heartbeat
report.

## TLS certificate

Use the certificate and private key for the endpoint's public `server_name`.
The certificate must contain that DNS name in its SAN extension. Keep the key
read-only inside the sidecar container. A self-signed certificate for
`127.0.0.1` does not satisfy a client configured for strict system CA
verification and the public DNS name.

The following Compose stack pins both images and mounts the same certificate
material read-only into 3x-ui/Xray and the diagnostics sidecar:

```yaml
services:
  x-ui:
    image: ghcr.io/mhsanaei/3x-ui:v2.8.11@sha256:ce050d75791a4576c0a5b2fdd207909efa7f88bf6a0a45c5424b949d5fd53432
    container_name: x-ui
    network_mode: host
    restart: unless-stopped
    environment:
      XRAY_VMESS_AEAD_FORCED: "false"
      XUI_ENABLE_FAIL2BAN: "false"
    volumes:
      - ./x-ui-state:/etc/x-ui
      - /etc/letsencrypt/live/xray.example/fullchain.pem:/run/tls/fullchain.pem:ro
      - /etc/letsencrypt/live/xray.example/privkey.pem:/run/tls/privkey.pem:ro
    healthcheck:
      test: ["CMD", "curl", "--fail", "--silent", "http://127.0.0.1:2053/"]
      interval: 10s
      timeout: 3s
      retries: 12

  xp2pdiag:
    image: oncharterliz/xp2pdiag:0.2.9
    container_name: xp2pdiag
    network_mode: host
    restart: unless-stopped
    environment:
      XP2P_SERVER_CERTIFICATE: /run/tls/fullchain.pem
      XP2P_SERVER_KEY: /run/tls/privkey.pem
    volumes:
      - /etc/letsencrypt/live/xray.example/fullchain.pem:/run/tls/fullchain.pem:ro
      - /etc/letsencrypt/live/xray.example/privkey.pem:/run/tls/privkey.pem:ro
```

Replace `xray.example` with the endpoint DNS name. In every TLS inbound created
through 3x-ui, select `/run/tls/fullchain.pem` and `/run/tls/privkey.pem`.
The containers use host networking so Xray inbounds, the 3x-ui panel, and the
diagnostics listener bind directly to their configured host ports. Restrict the
panel and port `62022` with host firewall rules.

After certificate renewal, restart both containers so Xray and the sidecar
reload the same files:

```console
docker compose restart x-ui xp2pdiag
```

If either certificate variable is missing, the file is unreadable, or the key
does not match the certificate, `xp2p diag` exits with a TLS configuration
error instead of starting an insecure fallback listener.

## Expected state

With `heartbeat_mode = "auto"`, a new endpoint starts as `probing`. After three
failed discovery attempts it becomes `not-detected`. Once the sidecar responds,
the state is:

```text
MODE=auto
CHECK=xp2p-diag
STATUS=healthy
FAILURE_STAGE=-
```

The detected check persists across client restarts. Three consecutive probe
failures change the status to `unhealthy`; a later successful ping clears the
failure counter and restores `healthy`. A single failure does not change the
selection state, which avoids failover flapping.

Use the detailed state table and a tunnel-selected ping when troubleshooting:

```console
xp2p client state --health-details
xp2p ping xray.example --tunnel --endpoint proxy-xray-example
```

`not-detected` means no health capability has been established.
`FAILURE_STAGE=probe` means readiness, TLS, or ping failed.
`FAILURE_STAGE=report` applies to a full `xp2p-heartbeat` endpoint and must not
be hidden by treating it as a ping-only sidecar.
