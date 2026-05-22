from __future__ import annotations

import shutil
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
JS_FILES = ["shared.js", "host.js", "vote.js", "results.js"]


def run(command: list[str]) -> None:
    printable = " ".join(command)
    print(f"$ {printable}")
    subprocess.run(command, cwd=ROOT, check=True)


def main() -> int:
    go = shutil.which("go")
    if go is None:
        print("$ go test ./...")
        print("error: go is not installed or is not on PATH")
        return 1

    run([go, "test", "./..."])

    node = shutil.which("node")
    if node is None:
        print("$ node --check shared.js host.js vote.js results.js")
        print("skipped: node is not installed")
        return 0

    for js_file in JS_FILES:
        run([node, "--check", js_file])

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
