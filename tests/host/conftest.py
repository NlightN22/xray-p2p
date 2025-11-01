import shutil
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable, Optional

import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant-win" / "windows10"
GUEST_REPO = Path(r"C:\xp2p")
DEFAULT_CLIENT = "win10-client"
DEFAULT_SERVER = "win10-server"
DEFAULT_WINRM_WAIT_TIMEOUT = 180  # seconds
DEFAULT_WINRM_POLL_INTERVAL = 5  # seconds
DEFAULT_WINRM_COMMAND_TIMEOUT = 60  # seconds


class GuestCommandError(RuntimeError):
    """Raised when executing a guest command fails."""


class MachineNotRunning(RuntimeError):
    """Raised when a target Vagrant VM is not in the running state."""


@dataclass
class CommandResult:
    command: str
    stdout: str
    stderr: str
    returncode: int


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("xp2p", "xp2p guest orchestration options")
    group.addoption(
        "--xp2p-target",
        action="store",
        default="10.0.10.1",
        help="Target address for xp2p guest ping probes.",
    )
    group.addoption(
        "--xp2p-port",
        action="store",
        default="62022",
        help="TCP port for xp2p guest ping probes.",
    )
    group.addoption(
        "--xp2p-attempts",
        action="store",
        type=int,
        default=3,
        help="Number of probe attempts the guest ping should perform.",
    )
    group.addoption(
        "--xp2p-machine",
        action="store",
        default=DEFAULT_CLIENT,
        help="Vagrant machine name that executes guest probes (default: win10-client).",
    )
    group.addoption(
        "--xp2p-winrm-wait",
        action="store",
        type=int,
        default=DEFAULT_WINRM_WAIT_TIMEOUT,
        help="Overall timeout (seconds) while waiting for WinRM to become available.",
    )
    group.addoption(
        "--xp2p-winrm-poll",
        action="store",
        type=int,
        default=DEFAULT_WINRM_POLL_INTERVAL,
        help="Polling interval (seconds) between WinRM availability checks.",
    )


@pytest.fixture(scope="session")
def xp2p_options(pytestconfig: pytest.Config) -> dict:
    return {
        "target": pytestconfig.getoption("xp2p_target"),
        "port": str(pytestconfig.getoption("xp2p_port")),
        "attempts": pytestconfig.getoption("xp2p_attempts"),
        "machine": pytestconfig.getoption("xp2p_machine"),
        "winrm_wait": pytestconfig.getoption("xp2p_winrm_wait"),
        "winrm_poll": pytestconfig.getoption("xp2p_winrm_poll"),
    }


class VagrantRunner:
    def __init__(self, vagrant_dir: Path):
        self.vagrant_dir = vagrant_dir

    def run(
        self,
        *args: str,
        capture_output: bool = True,
        check: bool = True,
        timeout: Optional[float] = None,
    ) -> subprocess.CompletedProcess[str]:
        stdout = subprocess.PIPE if capture_output else None
        stderr = subprocess.PIPE if capture_output else None
        proc = subprocess.run(
            ["vagrant", *args],
            cwd=self.vagrant_dir,
            stdout=stdout,
            stderr=stderr,
            text=True,
            timeout=timeout,
        )
        if check and proc.returncode != 0:
            output = proc.stdout or ""
            raise RuntimeError(
                f"Command 'vagrant {' '.join(args)}' failed with code {proc.returncode}:\n{output}"
            )
        return proc

    def get_state(self, machine: str) -> Optional[str]:
        result = self.run("status", machine, "--machine-readable")
        for line in result.stdout.splitlines():
            parts = line.split(",")
            if len(parts) >= 4 and parts[2] == "state":
                return parts[3]
        return None

    def ensure_running(
        self,
        machine: str,
        *,
        winrm_timeout: float = DEFAULT_WINRM_COMMAND_TIMEOUT,
        wait_timeout: float = DEFAULT_WINRM_WAIT_TIMEOUT,
        poll_interval: float = DEFAULT_WINRM_POLL_INTERVAL,
    ) -> None:
        if not machine:
            return
        state = self.get_state(machine)
        if state != "running":
            raise MachineNotRunning(
                f"Vagrant VM '{machine}' is not running (state={state!r})."
            )
        deadline = time.monotonic() + wait_timeout
        last_error: Optional[Exception] = None
        while time.monotonic() < deadline:
            try:
                self.run_winrm(
                    machine,
                    "hostname",
                    timeout=winrm_timeout,
                )
                return
            except (GuestCommandError, subprocess.TimeoutExpired) as exc:
                last_error = exc
                print(
                    f"[xp2p] Waiting for WinRM on {machine} "
                    f"(retry in {poll_interval}s)...",
                    flush=True,
                )
                time.sleep(poll_interval)
        raise RuntimeError(
            f"Timed out waiting for WinRM connectivity on {machine}"
        ) from last_error

    def run_winrm(
        self,
        machine: str,
        command: str,
        *,
        check: bool = True,
        timeout: Optional[float] = 300,
    ) -> CommandResult:
        proc = self.run("winrm", machine, "-c", command, timeout=timeout, check=False)
        result = CommandResult(
            command=command,
            stdout=proc.stdout or "",
            stderr=proc.stderr or "",
            returncode=proc.returncode,
        )
        if check and result.returncode != 0:
            raise GuestCommandError(
                f"Guest command failed on {machine} (exit {result.returncode}).\n"
                f"Command: {command}\n"
                f"STDOUT:\n{result.stdout}\n"
                f"STDERR:\n{result.stderr}"
            )
        return result


def _require_vagrant_environment() -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not VAGRANT_DIR.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{VAGRANT_DIR}' is missing; "
            "run `make vagrant-win10` before invoking host tests."
        )


@pytest.fixture(scope="session")
def vagrant_environment(xp2p_options: dict) -> VagrantRunner:
    _require_vagrant_environment()
    runner = VagrantRunner(VAGRANT_DIR)
    ensure_kwargs = {
        "wait_timeout": xp2p_options["winrm_wait"],
        "poll_interval": xp2p_options["winrm_poll"],
        "winrm_timeout": DEFAULT_WINRM_COMMAND_TIMEOUT,
    }
    try:
        runner.ensure_running(DEFAULT_SERVER, **ensure_kwargs)
        runner.ensure_running(DEFAULT_CLIENT, **ensure_kwargs)
    except MachineNotRunning as exc:
        pytest.skip(
            f"Guest environment unavailable: {exc}. Run `make vagrant-win10` and retry."
        )
    return runner


@pytest.fixture(scope="session")
def winrm_runner(vagrant_environment: VagrantRunner) -> Callable[[str, str], CommandResult]:
    return lambda machine, command, **kwargs: vagrant_environment.run_winrm(
        machine, command, **kwargs
    )


@pytest.fixture
def go_guest_runner(
    vagrant_environment: VagrantRunner, xp2p_options: dict
) -> Callable[..., CommandResult]:
    def _run(
        script: str,
        *,
        machine: Optional[str] = None,
        target: Optional[str] = None,
        port: Optional[str] = None,
        attempts: Optional[int] = None,
        extra_args: Optional[Iterable[str]] = None,
        check: bool = True,
        timeout: Optional[float] = 300,
    ) -> CommandResult:
        machine_name = machine or xp2p_options["machine"] or DEFAULT_CLIENT
        runner = vagrant_environment
        try:
            runner.ensure_running(
                machine_name,
                wait_timeout=xp2p_options["winrm_wait"],
                poll_interval=xp2p_options["winrm_poll"],
                winrm_timeout=DEFAULT_WINRM_COMMAND_TIMEOUT,
            )
        except MachineNotRunning as exc:
            pytest.skip(
                f"Guest '{machine_name}' unavailable: {exc}. Start the VM and retry."
            )

        script_path = script.replace("/", "\\")
        args = []
        target_value = target or xp2p_options["target"]
        port_value = port or xp2p_options["port"]
        attempts_value = attempts if attempts is not None else xp2p_options["attempts"]

        if target_value:
            args.extend(["--target", str(target_value)])
        if port_value:
            args.extend(["--port", str(port_value)])
        if attempts_value is not None:
            args.extend(["--attempts", str(attempts_value)])
        if extra_args:
            args.extend(list(extra_args))

        go_command = f"go run {script_path}"
        if args:
            go_command = f"{go_command} {' '.join(args)}"

        command = f"Set-Location {GUEST_REPO}; {go_command}"
        return runner.run_winrm(
            machine_name,
            command,
            check=check,
            timeout=timeout,
        )

    return _run
