<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# cautem documentation

Shared documentation for all cautem modules, built with
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/) (EN / RU).

**Site:** https://klorq.github.io/sandbox.dev/

## Layout

```text
mkdocs.yml            theme, navigation, RU nav titles
overrides/home.html   landing page hero (text comes from index.md front matter)
docs/
├── en/               English (default, served at /)
├── ru/               Russian (same tree, served at /ru/)
└── assets/           shared CSS and images
```

Keep `docs/en/` and `docs/ru/` in lockstep: same file names, same paths.
New pages also need an entry in `nav` in `mkdocs.yml` (and a RU title in
`nav_translations` if the label is new). The sidebar tree is the table of
contents — don't add pages that only list links to other pages.

## Local preview

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
mkdocs serve
```

Pushes to `main` build with `mkdocs build --strict` and deploy to GitHub Pages.
