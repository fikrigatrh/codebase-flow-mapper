# Vendored libraries

Files are byte-identical to the npm tarballs. Each tarball was checked against the registry's
`dist.integrity` before extraction. Versions were at least two weeks old when pinned (2026-11-13).

| File | Package | Version | License | sha256 |
|---|---|---|---|---|
| `cytoscape.min.js` | cytoscape | 3.34.4 | MIT (`LICENSE-cytoscape`) | `081591ab735b2b6a3b31a3c60f8f3a3e2fee8c92939e0c7820ad1208571b6cd8` |
| `cytoscape-dagre.min.js` | cytoscape-dagre | 4.0.1 | MIT (`LICENSE-cytoscape-dagre`) | `ec671a41d2bfcac580352e2997797793949e3489cecaf347fc48670d4bd456b0` |

`cytoscape-dagre` 4.x bundles `@dagrejs/dagre` 3.0.0 (MIT, `LICENSE-dagre`) and its dependency
`@dagrejs/graphlib` 4.0.1 (MIT, `LICENSE-graphlib`), so there is no separate dagre file.

To upgrade: download the tarball from `https://registry.npmjs.org/<pkg>/-/<pkg>-<ver>.tgz`, compare
its sha512 with `dist.integrity` from `https://registry.npmjs.org/<pkg>/<ver>`, copy the dist file,
and update this table and `internal/report` tests.
