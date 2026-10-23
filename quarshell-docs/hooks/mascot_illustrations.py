# SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
# SPDX-License-Identifier: Apache-2.0

"""Choose a transparent mascot illustration that matches each topic."""

from pathlib import PurePosixPath
from mkdocs.utils import normalize_url


def _illustration(source: str) -> str:
    path = source.lower()
    name = PurePosixPath(path).stem

    if "/providers/docker/" in f"/{path}" or name in {"first-sandbox", "images", "recreate-sandbox"}:
        return "docker"
    if "/providers/podman/" in f"/{path}":
        return "podman"
    if name in {"kubernetes", "microvm"}:
        return "worker"
    if name in {
        "security", "policy", "credentials", "environment", "settings",
        "development", "provider-profiles",
    }:
        return "security"
    if name in {"architecture", "gateway", "inference", "remote", "workspace", "workspaces"}:
        return "gateway"
    if name in {"fast", "performance"}:
        return "fast"
    if name in {"welcome", "about"}:
        return "happy"
    return "clean"


def on_page_content(html, page, config, files):
    """Place the matching, background-free mascot beside the title."""
    if page is None:
        return html

    source = page.file.src_path
    parts = PurePosixPath(source).parts
    if getattr(page, "is_homepage", False) or (parts[-1:] == ("index.md",) and len(parts) <= 2):
        return html
    if (getattr(page, "meta", None) or {}).get("mascot") == "hide":
        return html

    title_end = html.find("</h1>")
    if title_end < 0:
        return html

    variant = _illustration(source)
    # Every cutout now uses the same transparent gopher artwork.
    width, height = (720, 658)
    image_url = normalize_url(f"assets/images/mascot-cutouts/{variant}.png", page=page)
    illustration = (
        f'<figure class="ws-doc-mascot ws-doc-mascot--{variant}" aria-hidden="true">'
        f'<img class="ws-doc-mascot__art" src="{image_url}" '
        f'alt="" width="{width}" height="{height}" loading="lazy" draggable="false">'
        "</figure>"
    )
    title_end += len("</h1>")
    return html[:title_end] + illustration + html[title_end:]
