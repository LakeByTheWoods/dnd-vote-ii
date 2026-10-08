"""Run the supported backend and frontend checks."""
import pathlib
import shutil
import subprocess

npm = shutil.which("npm.cmd") or shutil.which("npm")
if not npm:
    raise SystemExit("Node.js and npm are required. Run npm ci first.")
subprocess.run([npm, "run", "verify"], cwd=pathlib.Path(__file__).resolve().parents[1], check=True)
