#!/usr/bin/env python3
import hashlib
import hmac
import http.client
import json
import ssl
import sys


ROTATE = "/control/v1/credentials/rotate"


def request(host, port, path, payload):
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    connection = http.client.HTTPSConnection(host, port, context=ssl._create_unverified_context(), timeout=5)
    connection.request("POST", path, body=body, headers={"Content-Type": "application/json"})
    response = connection.getresponse()
    raw = response.read()
    connection.close()
    return response.status, json.loads(raw.decode("utf-8")) if raw else {}


def proof(secret, nonce):
    return hmac.new(secret.encode("utf-8"), nonce.encode("utf-8"), hashlib.sha256).hexdigest()


def main():
    if len(sys.argv) != 5:
        raise SystemExit("usage: check_credential_rotation_rejected.py <host> <port> <user> <credential>")
    host, port_raw, user, credential = sys.argv[1:]
    port = int(port_raw)

    status, challenge = request(host, port, ROTATE, {"user_label": user, "action": "challenge"})
    if status != 200:
        raise AssertionError(f"rotation challenge status {status}, expected 200, payload={challenge}")
    nonce = challenge.get("nonce")
    if not nonce:
        raise AssertionError(f"invalid challenge payload: {challenge}")

    status, payload = request(host, port, ROTATE, {"user_label": user, "nonce": nonce, "proof": proof(credential, nonce)})
    if status != 401:
        raise AssertionError(f"credential was accepted by rotation endpoint: status={status}, payload={payload}")
    print(json.dumps({"rejected": True}))


if __name__ == "__main__":
    main()
