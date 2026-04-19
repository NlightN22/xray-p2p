from __future__ import annotations

import pytest

from tests.host.openwrt.flows.tunnel_b_to_a_impl import test_client_redirect_through_server, tunnel_environment

pytestmark = [pytest.mark.host, pytest.mark.linux]

