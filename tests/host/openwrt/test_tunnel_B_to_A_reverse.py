from __future__ import annotations

import pytest

from tests.host.openwrt.flows.tunnel_b_to_a_impl import test_reverse_redirect_via_server_portal, tunnel_environment

pytestmark = [pytest.mark.host, pytest.mark.linux]

