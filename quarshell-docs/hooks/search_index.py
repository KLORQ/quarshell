# SPDX-FileCopyrightText: Copyright (c) 2026 cautem
# SPDX-License-Identifier: Apache-2.0

"""Write one Material search index per documentation language."""

import json
from pathlib import Path

from mkdocs.plugins import event_priority


@event_priority(-200)
def on_post_build(config):
    """Split the combined i18n index after mkdocs-static-i18n finishes."""
    i18n = config.plugins.get("i18n")
    if getattr(i18n, "building", False):
        return

    index_path = Path(config.site_dir) / "search" / "search_index.json"
    index = json.loads(index_path.read_text(encoding="utf-8"))
    documents = {"en": [], "ru": []}
    for document in index["docs"]:
        locale = "ru" if document["location"].startswith("ru/") else "en"
        documents[locale].append(document)

    for locale, locale_documents in documents.items():
        localized_index = {
            **index,
            "config": {**index["config"], "lang": [locale]},
            "docs": locale_documents,
        }
        output_dir = (
            index_path.parent
            if locale == "en"
            else index_path.parent.parent / "ru" / "search"
        )
        output = output_dir / index_path.name
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(
            json.dumps(localized_index, ensure_ascii=False), encoding="utf-8"
        )

    # Material's search client requests a sitemap relative to each locale URL.
    # MkDocs emits one combined sitemap at the site root, so expose that same
    # index under /ru/ as well to avoid a missing sitemap request on Russian pages.
    sitemap = Path(config.site_dir) / "sitemap.xml"
    localized_sitemap = Path(config.site_dir) / "ru" / "sitemap.xml"
    if sitemap.is_file():
        localized_sitemap.parent.mkdir(parents=True, exist_ok=True)
        localized_sitemap.write_bytes(sitemap.read_bytes())
