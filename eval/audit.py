"""Lists what a reviewer's agents opened outside the directories they were given, among the places the sandbox leaves readable that are not the system or a toolchain."""

from __future__ import annotations

import json
import os
import re
from pathlib import Path

from confine import HIDDEN, SESSIONS, STATE

ABSOLUTE = re.compile(r"(?<![\w.])/(?:[\w.+@-]+/)*[\w.+@-]+")
TOOLCHAIN = ("/usr", "/bin", "/sbin", "/dev", "/etc", "/System", "/Library", "/opt", "/private/etc")


def outside(transcript: Path, allowed: tuple[Path, ...]) -> list[str]:
    home = Path.home()
    roots = tuple(os.path.realpath(a) for a in allowed)
    hidden = (str(home), *HIDDEN)
    found: list[str] = []
    for line in transcript.read_text(errors="replace").splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        message = event.get("message") or {}
        if event.get("type") != "message_end" or message.get("role") != "assistant":
            continue
        for block in message.get("content", []):
            if block.get("type") != "toolCall":
                continue
            for path in ABSOLUTE.findall(json.dumps(block.get("arguments") or {})):
                real = os.path.realpath(path)
                if not os.path.exists(real) or real.startswith(roots + TOOLCHAIN) or any(r.startswith(real) for r in roots + hidden):
                    continue
                if real.startswith(str(home / STATE)) and not real.startswith(str(home / SESSIONS)) or not real.startswith(hidden):
                    found.append(path)
    return sorted(set(found))
