#!/usr/bin/env python3
"""Render the presentation SVG groups (hero, architecture, process,
integrations) plus the static scene and demo-step cards from authored facts
and the renderer palette. Light/dark × desktop/mobile variants carry the
palette id in data-palette so stale exports are detectable."""

import argparse
import json
import math
import random
import shutil
import textwrap
from html import escape
from pathlib import Path

CSS = (
    "text{font-family:Arial,'Noto Sans','PingFang SC',sans-serif}"
    ".flow{stroke-dasharray:8 96;animation:flow 5.5s linear infinite}"
    ".pulse{animation:pulse 4.5s ease-in-out infinite}"
    ".bob{animation:bob 7s ease-in-out infinite}"
    "@keyframes flow{to{stroke-dashoffset:-312}}"
    "@keyframes pulse{0%,100%{opacity:.35}50%{opacity:1}}"
    "@keyframes bob{0%,100%{transform:translateY(0)}50%{transform:translateY(-5px)}}"
    "@media(prefers-reduced-motion:reduce){*{animation:none!important;transform:none!important}.flow{display:none}}"
)
STATIC_CSS = CSS + "*{animation:none!important;transform:none!important}.flow{display:none}"


def render(repo: Path):
    facts = json.loads((repo / "docs/presentation.json").read_text())
    palette = json.loads((repo / "web/palette.json").read_text())
    record = json.loads((repo / "docs/demo-results.json").read_text())
    site = json.loads((repo / "web/site.json").read_text())

    name = facts["name"]
    tagline = facts["tagline"]
    nodes = facts["architecture"]["nodes"]
    edges = facts["architecture"]["edges"]
    steps = facts["process"]["steps"]
    items = facts["integrations"]["items"]
    assert len(nodes) == 6 and len(edges) == 5 and len(steps) == 4 and 3 <= len(items) <= 6

    out = repo / "assets/presentation"
    web = repo / "web/assets/presentation"
    out.mkdir(parents=True, exist_ok=True)
    web.mkdir(parents=True, exist_ok=True)

    def lang(value):
        return str(value.get("en") or value.get("zh") or "") if isinstance(value, dict) else str(value or "")

    for mode in ("dark", "light"):
        p = palette[mode]

        def txt(x, y, s, size=18, color="ink", anchor="start", weight=400, opacity=None):
            o = f' opacity="{opacity}"' if opacity is not None else ""
            return (
                f'<text x="{x}" y="{y}" fill="{p[color]}" font-size="{size}" '
                f'text-anchor="{anchor}" font-weight="{weight}"{o}>{escape(s)}</text>'
            )

        def para(x, y, s, width, size=17, color="muted", lines=3, anchor="start", weight=400):
            wrapped = textwrap.wrap(s, max(8, int(width / (size * 0.55)))) or [""]
            if len(wrapped) > lines:
                wrapped = wrapped[:lines]
                wrapped[-1] = wrapped[-1].rstrip(".,") + "…"
            return "".join(
                txt(x, y + i * size * 1.42, t, size, color, anchor, weight) for i, t in enumerate(wrapped)
            )

        def wire(d, active=True):
            stroke = p["primary"] if active else p["line"]
            path = f'<path d="{d}" fill="none" stroke="{stroke}" stroke-width="1.4" marker-end="url(#arr)"/>'
            if active:
                path += f'<path d="{d}" class="flow" fill="none" stroke="{p["highlight"]}" stroke-width="2"/>'
            return path

        def chip(x, y, w, title, detail, h=112, accent=False):
            stroke = p["primary"] if accent else p["line"]
            inner = w - 32
            tl = len(textwrap.wrap(title, max(8, int(inner / (19 * 0.55)))))
            detail_y = y + 30 + (tl - 1) * 27 + 18
            return (
                f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{p["panel"]}" stroke="{stroke}" stroke-width="{1.6 if accent else 1}"/>'
                + para(x + 16, y + 30, title, inner, 19, "ink", 2, weight=600)
                + para(x + 16, detail_y, detail, inner, 15, "muted", 2)
            )

        def particles(cx, cy, rx, ry, count, seed):
            rng = random.Random(seed)
            body = ""
            for i in range(count):
                a = rng.random() * math.tau
                z = rng.uniform(-1, 1)
                r = math.sqrt(1 - z * z)
                x, y = cx + rx * r * math.cos(a), cy + ry * z
                body += (
                    f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{rng.uniform(.5, 1.6):.2f}" '
                    f'fill="{p["primary"] if i % 3 else p["secondary"]}" opacity="{rng.uniform(.15, .7):.2f}"/>'
                )
            return body

        def save(kind, w, h, title, desc, body, mobile=False, static=False):
            slug = f"{kind}{'-mobile' if mobile else ''}-{mode}"
            svg = (
                f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
                f'data-palette="{palette["id"]}" role="img" aria-labelledby="{slug}-t {slug}-d">'
                f'<title id="{slug}-t">{escape(title)}</title><desc id="{slug}-d">{escape(desc)}</desc>'
                "<defs>"
                f'<linearGradient id="brand" gradientUnits="userSpaceOnUse" x2="{w}" y2="{h}">'
                f'<stop stop-color="{p["primary"]}"/><stop offset=".55" stop-color="{p["secondary"]}"/>'
                f'<stop offset="1" stop-color="{p["highlight"]}"/></linearGradient>'
                f'<radialGradient id="halo"><stop stop-color="{p["primary"]}" stop-opacity=".2"/>'
                f'<stop offset="1" stop-color="{p["bg"]}" stop-opacity="0"/></radialGradient>'
                '<filter id="glow"><feGaussianBlur stdDeviation="4"/></filter>'
                f'<pattern id="grid" width="44" height="44" patternUnits="userSpaceOnUse">'
                f'<path d="M44 0H0V44" fill="none" stroke="{p["line"]}" stroke-opacity=".3" stroke-width=".6"/></pattern>'
                '<marker id="arr" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto">'
                f'<path d="m1 1 7 4-7 4" fill="none" stroke="{p["primary"]}" stroke-width="1.4"/></marker>'
                f'</defs><style>{STATIC_CSS if static else CSS}</style>'
                f'<rect width="{w}" height="{h}" rx="16" fill="{p["bg"]}"/>'
                f'<rect width="{w}" height="{h}" rx="16" fill="url(#grid)"/>'
                f'<ellipse cx="{w * .62}" cy="{h * .45}" rx="{w * .5}" ry="{h * .72}" fill="url(#halo)"/>'
                f"{body}</svg>\n"
            )
            dest = out / f"{slug}.svg"
            dest.write_text(svg)
            shutil.copy2(dest, web / dest.name)

        # ---------------- hero: ledger spine + converging particles ----------------
        for mobile in (False, True):
            if mobile:
                w, h = 400, 540
                b = txt(28, 46, "TOMBSTONE LEDGER FOR CODING AGENTS", 12, "muted")
                b += f'<text x="28" y="104" font-size="44" fill="url(#brand)" font-weight="700">deadend-ledger</text>'
                b += para(28, 146, tagline, 344, 19, "ink", 4)
                b += txt(28, 250, facts["advantage"], 17, "highlight", weight=600)
                b += particles(200, 420, 185, 105, 130, "hero-m")
                b += f'<path d="M28 340H372" stroke="{p["line"]}"/>'
                for i in range(3):
                    y = 366 + i * 52
                    b += f'<rect x="28" y="{y}" width="344" height="40" rx="8" fill="{p["panel"]}" stroke="{p["line"]}"/>'
                    b += f'<circle cx="52" cy="{y + 20}" r="5" fill="{p["secondary"]}"/>'
                    b += txt(68, y + 25, f"tombstone #{3 - i} · do-not-retry", 15, "ink", weight=600)
                save("hero", w, h, name, tagline, b, mobile=True)
            else:
                w, h = 1000, 390
                b = particles(836, 208, 175, 160, 210, "hero")
                b += txt(48, 56, "TOMBSTONE LEDGER FOR CODING AGENTS", 13, "muted")
                b += f'<text x="48" y="138" font-size="64" fill="url(#brand)" font-weight="700">deadend-ledger</text>'
                b += para(48, 196, tagline, 560, 22, "ink", 3)
                b += txt(48, 300, facts["advantage"], 20, "highlight", weight=600)
                b += f'<path d="M700 78V356" stroke="{p["line"]}"/><path d="M700 78V356" stroke="{p["primary"]}" stroke-opacity=".35"/>'
                labels = ["client-side token refresh", "on-disk token cache", "jwt v9 bump"]
                for i in range(3):
                    y = 84 + i * 92
                    b += f'<g class="bob" style="animation-delay:{i * 1.2}s">'
                    b += f'<rect x="726" y="{y}" width="226" height="64" rx="10" fill="{p["panel"]}" stroke="{p["line"]}"/>'
                    b += f'<circle cx="750" cy="{y + 32}" r="6" fill="{p["secondary"]}" filter="url(#glow)"/>'
                    b += f'<circle class="pulse" cx="750" cy="{y + 32}" r="3" fill="{p["highlight"]}"/>'
                    b += txt(766, y + 26, f"tombstone #{3 - i}", 14, "ink", weight=600)
                    b += txt(766, y + 46, labels[i], 12, "muted")
                    b += "</g>"
                    b += wire(f"M726 {y + 32}H700", active=(i == 0))
                save("hero", w, h, name, tagline, b)

        # ---------------- architecture: two lanes, one ledger ----------------
        for mobile in (False, True):
            kicker = txt(44 if not mobile else 28, 44, "ARCHITECTURE / SESSION + RECORD PATHS", 13, "muted")
            head = para(44 if not mobile else 28, 84, facts["architecture"]["title"], 900 if not mobile else 344, 26 if not mobile else 23, "ink", 2, weight=600)
            if mobile:
                h = 118 + len(nodes) * 118 + 56
                b = kicker + head
                coords = [(28, 128 + i * 118, 344) for i in range(len(nodes))]
                for k, e in enumerate(edges):
                    a, t = e
                    ax, ay, aw = coords[a]
                    bx, by, bw = coords[t]
                    if t == a + 1 and by > ay:
                        d = f"M{ax + aw / 2} {ay + 100}V{by}"
                    else:
                        d = f"M{ax + aw} {ay + 55}H386V{by + 55}H{bx + bw}"
                    b += wire(d, active=True)
                for n, (x, y, cw) in zip(nodes, coords):
                    b += chip(x, y, cw, n["title"], n["detail"], 100)
                b += txt(28, h - 22, "one JSONL line per tombstone · nothing else is written", 14, "highlight")
                save("architecture", 400, h, name + " architecture", facts["architecture"]["title"], b, mobile=True)
            else:
                h = 570
                b = kicker + head
                coords = {
                    0: (24, 140, 220), 1: (268, 140, 200), 2: (492, 140, 200), 3: (716, 140, 200),
                    4: (268, 392, 200), 5: (492, 392, 220),
                }
                routes = {
                    (0, 1): "M244 196H268", (1, 2): "M468 196H492", (2, 3): "M692 196H716",
                    (4, 5): "M468 448H492",
                    (5, 1): "M602 392V318H368V252",
                }
                b += txt(24, 128, "SESSION PATH — runs on every prompt, under 5 s", 12, "muted")
                b += txt(268, 380, "RECORD PATH — the only writer", 12, "muted")
                for e, d in routes.items():
                    b += wire(d, active=True)
                for i, n in enumerate(nodes):
                    x, y, cw = coords[i]
                    b += chip(x, y, cw, n["title"], n["detail"], 112, accent=(i == 5))
                b += txt(492, 540, "inject reads the ledger · record is the only writer", 12, "highlight")
                save("architecture", 1000, h, name + " architecture", facts["architecture"]["title"], b)

        # ---------------- process: staggered timeline ----------------
        for mobile in (False, True):
            kicker = txt(44 if not mobile else 28, 44, "PROCESS / FROM FAILURE TO SKIP", 13, "muted")
            head = para(44 if not mobile else 28, 84, facts["process"]["title"], 900 if not mobile else 344, 26 if not mobile else 23, "ink", 2, weight=600)
            if mobile:
                h = 130 + len(steps) * 152 + 74
                b = kicker + head
                for i, s in enumerate(steps):
                    y = 128 + i * 152
                    if i < len(steps) - 1:
                        b += wire(f"M200 {y + 100}V{y + 152}")
                    b += chip(28, y, 344, f"{i + 1:02}  {s['title']}", s["detail"], 100)
                b += txt(28, h - 40, facts["process"]["result"], 17, "highlight", weight=600)
                b += txt(28, h - 16, "unrelated task: zero bytes out", 13, "muted")
                save("process", 400, h, name + " process", facts["process"]["result"], b, mobile=True)
            else:
                h = 470
                b = kicker + head
                spine_y = 250
                b += f'<path d="M70 {spine_y}H930" stroke="{p["line"]}"/>'
                centers = [160, 387, 614, 841]
                for i, (cx, s) in enumerate(zip(centers, steps)):
                    b += f'<circle cx="{cx}" cy="{spine_y}" r="15" fill="{p["panel"]}" stroke="{p["primary"]}"/>'
                    b += txt(cx, spine_y + 5, f"{i + 1:02}", 13, "primary", "middle", 700)
                    b += f'<circle class="pulse" cx="{cx}" cy="{spine_y}" r="4" fill="{p["highlight"]}"/>'
                    above = i % 2 == 0
                    cy = spine_y - 148 if above else spine_y + 28
                    b += chip(cx - 105, cy, 210, s["title"], s["detail"], 120)
                    stem = f"M{cx} {cy + 120}V{spine_y - 15}" if above else f"M{cx} {spine_y + 15}V{cy}"
                    b += f'<path d="{stem}" stroke="{p["line"]}" stroke-dasharray="3 5"/>'
                b += txt(500, 420, facts["process"]["result"], 19, "highlight", "middle", 600)
                b += txt(500, 448, "unaffected tasks are untouched: no match, zero bytes out", 14, "muted", "middle")
                save("process", 1000, h, name + " process", facts["process"]["result"], b)

        # ---------------- integrations: orbital, implemented surfaces only ----------------
        for mobile in (False, True):
            kicker = txt(44 if not mobile else 28, 44, "CAPABILITIES / IMPLEMENTED SURFACES", 13, "muted")
            head = para(44 if not mobile else 28, 84, facts["integrations"]["title"], 900 if not mobile else 344, 26 if not mobile else 23, "ink", 2, weight=600)
            if mobile:
                h = 130 + 96 + len(items) * 118 + 30
                b = kicker + head
                b += f'<circle cx="200" cy="176" r="54" fill="{p["panel"]}" stroke="{p["primary"]}"/>'
                b += txt(200, 184, "deadend", 18, "primary", "middle", 700)
                for i, item in enumerate(items):
                    y = 258 + i * 118
                    b += chip(28, y, 344, item["title"], item["detail"], 100)
                save("integrations", 400, h, name + " integration routes", facts["integrations"]["title"], b, mobile=True)
            else:
                h = 600
                b = kicker + head
                cx, cy = 500, 360
                b += f'<ellipse cx="{cx}" cy="{cy}" rx="244" ry="184" fill="none" stroke="{p["line"]}"/>'
                b += f'<ellipse cx="{cx}" cy="{cy}" rx="196" ry="146" fill="none" stroke="{p["line"]}" stroke-dasharray="2 9"/>'
                b += f'<circle cx="{cx}" cy="{cy}" r="72" fill="{p["panel"]}" stroke="{p["primary"]}" stroke-width="1.6"/>'
                b += txt(cx, cy - 2, "deadend", 22, "primary", "middle", 700)
                b += txt(cx, cy + 22, "single binary", 12, "muted", "middle")
                left = items[:3]
                right = items[3:]
                for i, item in enumerate(left):
                    x, y = 40, 150 + i * 140
                    b += chip(x, y, 236, item["title"], item["detail"], 112)
                    b += wire(f"M276 {y + 56}H360V{cy + (i - 1) * 34}H{cx - 72}")
                for i, item in enumerate(right):
                    x, y = 724, 220 + i * 140
                    b += chip(x, y, 236, item["title"], item["detail"], 112)
                    b += wire(f"M724 {y + 56}H640V{cy + (i + 1) * 32}H{cx + 72}")
                save("integrations", 1000, h, name + " integration routes", facts["integrations"]["title"], b)

        # ---------------- static scene fallback ----------------
        b = particles(500, 360, 460, 260, 1400, "scene")
        b += f'<path d="M180 420L360 300L540 350L760 236" fill="none" stroke="{p["primary"]}" stroke-width="1.4"/>'
        for x, y in ((180, 420), (360, 300), (540, 350), (760, 236)):
            b += f'<circle cx="{x}" cy="{y}" r="10" fill="{p["highlight"]}" opacity=".45" filter="url(#glow)"/>'
            b += f'<circle cx="{x}" cy="{y}" r="3" fill="{p["highlight"]}"/>'
        b += f'<rect x="392" y="470" width="216" height="52" rx="8" fill="{p["panel"]}" stroke="{p["line"]}"/>'
        b += txt(500, 502, ".deadend/ledger.jsonl", 15, "ink", "middle", 600)
        save("scene", 1000, 700, name + " particle scene",
             "Illustrative particle scene; recorded output appears in the demo section.", b, static=True)

        # ---------------- demo step cards from the recorded run ----------------
        for i, fact in enumerate(record["steps"]):
            meta = site["demo"]["steps"][i]
            label, body = lang(meta["label"]), lang(meta["body"])
            b = (
                txt(36, 44, f"DEMO / STEP {i + 1:02}", 13, "muted")
                + para(36, 92, label, 688, 26, "ink", 2, weight=600)
                + para(36, 152, body, 688, 16, "muted", 3)
            )
            b += f'<path d="M36 226H724" stroke="{p["line"]}"/>'
            b += para(36, 258, fact["command"], 688, 14, "primary", 2, weight=600)
            b += txt(36, 312, "output: docs/demo-results.json", 12, "muted")
            save(f"demo-{i}", 760, 330, f"{name} demo step {i + 1}", body, b)

    print(f"{name} {palette['id']}: presentation SVGs generated")


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", type=Path, default=Path.cwd())
    render(ap.parse_args().repo.resolve())
