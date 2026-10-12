"""Runs pi in a macOS sandbox that can read the repository under review and none of the user's other files, cannot change the repository and cannot start a program that downloads."""

from __future__ import annotations

import os
import shlex
import shutil
import tempfile
from pathlib import Path

HIDDEN = ("/private/tmp", "/private/var/folders", "/Volumes")
STATE = ".pi"
SESSIONS = ".pi/agent/sessions"
TOOLS = (".local/share/mise", ".config/mise")


def environment(repo: Path, also: tuple[Path, ...], flags: list[str]) -> dict[str, str]:
    home = Path.home()
    open_ = [Path(os.path.realpath(p)) for p in (repo, *also)]
    above = sorted({str(a) for p in open_ for a in p.parents if str(a) != "/"} | {str(home)})
    box = Path(os.path.realpath(tempfile.mkdtemp()))
    scratch = box / "tmp"
    scratch.mkdir()
    (box / "pi.sb").write_text("\n".join([
        "(version 1)",
        "(allow default)",
        "(deny file-read* " + " ".join(f'(subpath "{p}")' for p in (str(home), *HIDDEN)) + ")",
        "(deny file-write* " + " ".join(f'(subpath "{p}")' for p in (str(home), "/private/tmp", open_[0])) + ")",
        "(allow file-read* "
        + " ".join(f'(subpath "{p}")' for p in (*open_, scratch, home / STATE, *(home / t for t in TOOLS)))
        + ' (regex #"/eyeful-agent-[^/]*" #"/T/xcrun_db[^/]*$") '
        + " ".join(f'(literal "{a}")' for a in above)
        + ")",
        f'(deny file-read* (subpath "{home / SESSIONS}") (regex #"/(site|dist)-packages(/|$)"))',
        f'(allow file-write* (subpath "{home / STATE}") (subpath "{scratch}") (regex #"/T/xcrun_db[^/]*$"))',
        '(deny process-exec (regex #"/(curl|wget|ssh|nc|pip[0-9.]*|python[0-9.]*|git-remote-https?)$"))',
        "(allow file-read-metadata)",
        "",
    ]))
    wrapper = box / "pi"
    wrapper.write_text(
        f"#!/bin/sh\nexport TMPDIR={shlex.quote(str(scratch))} GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 "
        "GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile GIT_CONFIG_VALUE_0=/dev/null\n"
        f"exec sandbox-exec -f {shlex.quote(str(box / 'pi.sb'))} {shlex.quote(shutil.which('pi') or 'pi')} "
        f"{' '.join(shlex.quote(f) for f in flags)} \"$@\"\n"
    )
    wrapper.chmod(0o755)
    return os.environ | {"PATH": f"{box}{os.pathsep}{os.environ['PATH']}"}
