---
sidebar_position: 9
title: Intégrations
description: Contrats Headroom et MemPalace.
---

# Intégrations

Headroom et MemPalace sont des sidecars de production requis.

```bash
HEADROOM_URL=http://headroom:8787
HEADROOM_TIMEOUT_MS=5000
MEMPALACE_URL=http://mempalace:8788
MEMPALACE_TIMEOUT_MS=5000
```

Headroom réduit le contexte avant la review modèle. MemPalace sert au rappel,
à la prévisualisation des propositions mémoire, puis à l'écriture après
approbation finale.
