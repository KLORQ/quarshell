# Agent instructions

## Scope

This repository builds the shared English and Russian product site with
Material for MkDocs. The landing page is implemented in `overrides/home.html`;
content lives under `docs/en/` and `docs/ru/`.

## Working rules

- Keep the English and Russian page trees aligned and update both translations
  when product behavior changes.
- Add pages to `nav` in `mkdocs.yml` and add Russian navigation labels where
  needed. Use site-relative asset URLs so GitHub Pages subpath hosting works.
- Keep metadata, generated asset paths, and the assertions in
  `.github/workflows/docs.yml` synchronized.
- Do not commit the generated `site/` directory or local virtual environments.
- Verify product and compatibility claims against the owning module before
  publishing them.

## Checks

- Run `mkdocs build --strict` from this checkout.
- From the parent workspace, `task docs:strict` runs the pinned container build
  and site-output checks.
