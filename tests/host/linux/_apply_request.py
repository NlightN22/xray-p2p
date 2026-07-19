from __future__ import annotations


def role_request(document: dict, role: str) -> dict:
    if document.get("version") == 2:
        request = (document.get("requests") or {}).get(role)
        if not isinstance(request, dict):
            raise AssertionError(f"{role} apply request is missing: {document}")
        if request.get("role") != role:
            raise AssertionError(f"{role} apply request has invalid role: {request}")
        return request

    request_role = document.get("role")
    if request_role not in {None, "", "any", role}:
        raise AssertionError(f"legacy apply request targets {request_role}, expected {role}")
    return document
