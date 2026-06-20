#!/usr/bin/env python3
import hashlib
import hmac
import http.client
import json
import secrets
import ssl
import sys
import time


def request(host, port, method, path, body=None, user=None, secret=None):
    payload = b"" if body is None else json.dumps(body, separators=(",", ":")).encode("utf-8")
    headers = {}
    nonce = ""
    if user and secret:
        timestamp = int(time.time())
        nonce = secrets.token_hex(12)
        body_hash = hashlib.sha256(payload).hexdigest()
        canonical = "\n".join((method.upper(), path, "", body_hash, str(timestamp), nonce))
        signature = hmac.new(secret.encode("utf-8"), canonical.encode("utf-8"), hashlib.sha256).hexdigest()
        headers.update(
            {
                "X-XP2P-User": user,
                "X-XP2P-Timestamp": str(timestamp),
                "X-XP2P-Nonce": nonce,
                "X-XP2P-Signature": signature,
            }
        )
    if payload:
        headers["Content-Type"] = "application/json"
    connection = http.client.HTTPSConnection(host, port, context=ssl._create_unverified_context(), timeout=5)
    connection.request(method, path, body=payload, headers=headers)
    response = connection.getresponse()
    raw = response.read()
    connection.close()
    decoded = json.loads(raw.decode("utf-8")) if raw else {}
    return response.status, decoded, nonce


def expect_status(actual, expected, label, payload):
    if actual != expected:
        raise AssertionError(f"{label}: status {actual}, expected {expected}, payload={payload}")


def wait_ready(host, port):
    deadline = time.monotonic() + 45
    last_error = None
    while time.monotonic() < deadline:
        try:
            status, payload, _ = request(host, port, "GET", "/control/v1/ready")
            if status == 200 and payload.get("ready") is True:
                return
            last_error = f"status={status} payload={payload}"
        except OSError as error:
            last_error = str(error)
        time.sleep(1)
    raise AssertionError(f"control endpoint did not become ready: {last_error}")


def main():
    if len(sys.argv) not in (7, 10):
        raise SystemExit(
            "usage: check_subscription_control_plane.py <host> <port> <user> <secret> <subscription-host> <tunnel-port> [<profile> <protocol> <flow>]"
        )
    host, port_raw, user, secret, subscription_host, trojan_port_raw = sys.argv[1:7]
    expected_profile = "trojan-tls"
    expected_protocol = "trojan"
    expected_flow = ""
    if len(sys.argv) == 10:
        expected_profile, expected_protocol, expected_flow = sys.argv[7:]
    port = int(port_raw)
    trojan_port = int(trojan_port_raw)

    wait_ready(host, port)

    status, payload, _ = request(host, port, "GET", "/control/v1/subscription")
    expect_status(status, 401, "unsigned subscription", payload)

    status, payload, _ = request(
        host, port, "GET", "/control/v1/subscription", user=user, secret="wrong-secret"
    )
    expect_status(status, 403, "invalid subscription signature", payload)

    status, subscription, _ = request(host, port, "GET", "/control/v1/subscription", user=user, secret=secret)
    expect_status(status, 200, "signed subscription", subscription)
    if not subscription.get("generation"):
        raise AssertionError(f"subscription generation is missing: {subscription}")
    expected = {
        "profile": expected_profile,
        "protocol": expected_protocol,
        "transport": "tcp",
        "security": "tls",
        "host": subscription_host,
        "port": trojan_port,
    }
    for key, value in expected.items():
        if subscription.get(key) != value:
            raise AssertionError(f"subscription {key}={subscription.get(key)!r}, expected {value!r}")
    tls = subscription.get("tls") or {}
    if tls.get("server_name") != subscription_host or not tls.get("pinned_peer_cert_sha256"):
        raise AssertionError(f"subscription TLS metadata is incomplete: {tls}")
    parameters = subscription.get("parameters") or {}
    if expected_flow and parameters.get("flow") != expected_flow:
        raise AssertionError(f"subscription flow={parameters.get('flow')!r}, expected {expected_flow!r}")

    status, payload, _ = request(host, port, "POST", "/control/v1/ping", {"nonce": "unsigned"})
    expect_status(status, 401, "unsigned ping", payload)

    status, payload, nonce = request(host, port, "POST", "/control/v1/ping", {"nonce": "signed"}, user, secret)
    expect_status(status, 200, "signed ping", payload)
    if payload.get("nonce") != "signed" or not nonce:
        raise AssertionError(f"unexpected signed ping response: {payload}")

    heartbeat = {"tag": "subscription-control", "host": subscription_host, "user": user, "rtt_ms": 7}
    status, payload, _ = request(host, port, "POST", "/control/v1/heartbeat", heartbeat, user, secret)
    expect_status(status, 200, "signed heartbeat", payload)
    if payload.get("ok") is not True:
        raise AssertionError(f"unexpected heartbeat response: {payload}")

    print(json.dumps({"generation": subscription["generation"], "heartbeat_tag": heartbeat["tag"], "profile": subscription.get("profile")}))


if __name__ == "__main__":
    main()
