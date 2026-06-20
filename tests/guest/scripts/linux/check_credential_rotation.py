#!/usr/bin/env python3
import hashlib
import hmac
import http.client
import json
import secrets
import ssl
import sys
import time
import uuid


ROTATE = "/control/v1/credentials/rotate"
ACK = "/control/v1/credentials/ack"


def request(host, port, path, payload):
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    connection = http.client.HTTPSConnection(host, port, context=ssl._create_unverified_context(), timeout=5)
    connection.request("POST", path, body=body, headers={"Content-Type": "application/json"})
    response = connection.getresponse()
    raw = response.read()
    connection.close()
    return response.status, json.loads(raw.decode("utf-8")) if raw else {}


def challenge(host, port, user):
    status, payload = request(host, port, ROTATE, {"user_label": user, "action": "challenge"})
    expect(status, 200, "rotation challenge", payload)
    if not payload.get("nonce") or not payload.get("expires_at"):
        raise AssertionError(f"invalid challenge payload: {payload}")
    return payload["nonce"]


def proof(secret, nonce):
    return hmac.new(secret.encode("utf-8"), nonce.encode("utf-8"), hashlib.sha256).hexdigest()


def rotate(host, port, user, secret):
    nonce = challenge(host, port, user)
    return request(host, port, ROTATE, {"user_label": user, "nonce": nonce, "proof": proof(secret, nonce)})


def expect(actual, expected, label, payload):
    if actual != expected:
        raise AssertionError(f"{label}: status {actual}, expected {expected}, payload={payload}")


def signed_subscription(host, port, user, secret):
    nonce = secrets.token_hex(12)
    timestamp = str(int(time.time()))
    canonical = "\n".join(("GET", "/control/v1/subscription", "", hashlib.sha256(b"").hexdigest(), timestamp, nonce))
    signature = hmac.new(secret.encode("utf-8"), canonical.encode("utf-8"), hashlib.sha256).hexdigest()
    connection = http.client.HTTPSConnection(host, port, context=ssl._create_unverified_context(), timeout=5)
    connection.request("GET", "/control/v1/subscription", headers={
        "X-XP2P-User": user,
        "X-XP2P-Timestamp": timestamp,
        "X-XP2P-Nonce": nonce,
        "X-XP2P-Signature": signature,
    })
    response = connection.getresponse()
    payload = json.loads(response.read().decode("utf-8"))
    connection.close()
    return response.status, payload


def main():
    if len(sys.argv) != 5:
        raise SystemExit("usage: check_credential_rotation.py <host> <port> <user> <previous-credential>")
    host, port_raw, user, previous = sys.argv[1:]
    port = int(port_raw)

    status, payload = request(host, port, ROTATE, {"user_label": "unknown", "action": "challenge"})
    expect(status, 401, "unknown user challenge", payload)
    status, payload = request(host, port, ROTATE, {"user_label": user, "nonce": "invalid", "proof": "00"})
    expect(status, 401, "invalid rotation proof", payload)

    status, pending = rotate(host, port, user, previous)
    expect(status, 200, "previous credential rotation", pending)
    if pending.get("rotation_pending") is not True or not pending.get("active_credential"):
        raise AssertionError(f"previous credential did not receive pending rotation: {pending}")
    active = pending["active_credential"]
    try:
        parsed = uuid.UUID(active)
    except ValueError as error:
        raise AssertionError(f"active credential is not UUID: {active!r}") from error
    if str(parsed) != active.lower():
        raise AssertionError(f"active credential is not canonical UUID: {active!r}")

    status, subscription = signed_subscription(host, port, user, previous)
    expect(status, 403, "previous credential subscription", subscription)
    status, active_result = rotate(host, port, user, active)
    expect(status, 200, "active credential rotation", active_result)
    if active_result.get("rotation_pending") is not False or active_result.get("active_credential"):
        raise AssertionError(f"active credential exposed rotation state: {active_result}")

    nonce = challenge(host, port, user)
    status, ack = request(host, port, ACK, {"user_label": user, "nonce": nonce, "proof": proof(active, nonce)})
    expect(status, 200, "rotation acknowledgement", ack)
    if ack.get("ok") is not True:
        raise AssertionError(f"unexpected acknowledgement payload: {ack}")

    status, payload = rotate(host, port, user, previous)
    expect(status, 401, "previous credential after acknowledgement", payload)
    print(json.dumps({"credential_generation": pending.get("credential_generation"), "subscription_generation": pending.get("subscription_generation")}))


if __name__ == "__main__":
    main()
