# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## What This Is

**agenKic-orKistrator** — a pixelated AI office orchestrator. A native desktop application where AI agents appear as pixel-art workers at desks. Users see agent activity in real time, route tasks across models (Claude, Gemini, Ollama), and coordinate multi-agent workflows from a retro-aesthetic interface.

## Design Context

### Users

Orchestrator operators — developers and engineers monitoring multi-agent AI workflows. They use the docs to understand architecture, review epic scope, and onboard to the system. Context is a technical deep-dive: they arrive knowing what an orchestrator is and want to see how *this one* works.

### Brand Personality

**Clean, cute, pixelated.**

The project is a pixel-art office world. The documentation should feel like part of that world — not a corporate slide deck bolted onto a retro project. Every page should immediately signal "this is a pixel thing" without sacrificing readability or professionalism.

Emotional goals: clarity, delight, quiet charm. The docs should feel like opening the manual for a well-crafted indie game — inviting, precise, and a little endearing.

### Aesthetic Direction

**Visual tone**: Clean retro with a cute pastel pixel palette. Sharp borders, bitmap typography, pixel grid textures — but rendered in soft, warm colors rather than harsh neon or monochrome green.

**Palette philosophy**: Cute pastel pixel. Soft pinks, lavenders, mints, warm yellows, and peach tones — all rendered with crisp pixel borders on dark or light backgrounds. Think: the color sensibility of a cozy pixel game, not an arcade cabinet.

**Light mode**: Warm cream/ivory background, pastel accents, pixel borders in soft grays.
**Dark mode**: Deep navy/charcoal background, pastel accents glow softly against dark surfaces.

**Typography**:
- Display/headings: `Press Start 2P` (Google Fonts) — the canonical pixel bitmap font
- Body text: `DM Sans` or similar clean sans-serif at 14-15px — pixel fonts at body size hurt readability
- Code/mono: `JetBrains Mono` — sharp, technical, good at small sizes

**Key visual elements**:
- 2px solid pixel borders (no border-radius — sharp corners everywhere)
- Pixel grid background pattern (subtle, 16-20px grid)
- Inset/outset pixel box shadows (2-4px offset, no blur)
- Status badges and tags styled as pixel UI elements
- Mermaid diagrams with pastel node colors matching the palette

**References** (what it should evoke):
- Pixelact UI component style (sharp, systematic pixel borders)
- Cozy indie game manuals (warm, inviting, precise)
- PICO-8 color palette sensibility (constrained but expressive)

**Anti-references** (what it must NOT look like):
- Generic AI SaaS (gradient blobs, glassmorphism, Inter font)
- Corporate editorial (serif headings, earth tones, minimal whitespace) — what the docs currently look like
- Brutalist / unstyled HTML

### Design Principles

1. **Pixel-native**: Every visual element should feel like it belongs in a pixel-art world. Sharp corners, integer-aligned borders, bitmap headings. No smooth gradients, no rounded corners, no soft shadows.

2. **Pastel warmth over neon edge**: The palette is soft and inviting — pinks, lavenders, mints, warm yellows. Avoid harsh neon greens, aggressive reds, or monochrome terminal aesthetics. The project is cute, not gritty.

3. **Readability first**: Pixel fonts for headings only. Body text stays in a clean sans-serif. Diagrams must be legible. The retro aesthetic enhances — never obstructs — information delivery.

4. **Dark and light as equals**: Both modes are first-class. The pastel palette must work on cream backgrounds AND dark navy backgrounds. Design both simultaneously, not one as an afterthought.

5. **Consistent across all pages**: Every explainer, every decision doc, every index page shares the same pixel design system — same palette, same borders, same typography, same spacing scale. No per-page improvisation.

### CSS Design Tokens (Reference Palette)

```css
/* ── Light Mode ── */
--bg: #faf5ef;              /* warm cream */
--surface: #ffffff;
--surface-alt: #f3ede4;
--border: #c4b8a8;          /* warm gray pixel border */
--border-strong: #9e8e7a;
--text: #2d2520;
--text-dim: #7a6e62;

--pink: #e8879b;            /* pastel pink */
--pink-dim: rgba(232,135,155,0.12);
--lavender: #a78bdb;        /* pastel lavender */
--lavender-dim: rgba(167,139,219,0.12);
--mint: #6bc9a6;            /* pastel mint */
--mint-dim: rgba(107,201,166,0.12);
--peach: #f0a868;           /* pastel peach/amber */
--peach-dim: rgba(240,168,104,0.12);
--sky: #7ab8e0;             /* pastel sky blue */
--sky-dim: rgba(122,184,224,0.12);
--lemon: #e8d44d;           /* warm yellow */
--lemon-dim: rgba(232,212,77,0.12);

/* ── Dark Mode ── */
--bg: #151020;              /* deep purple-navy */
--surface: #1e1830;
--surface-alt: #261e3a;
--border: #3a3050;          /* muted purple border */
--border-strong: #5a4a70;
--text: #e8e0f0;
--text-dim: #9a8eb0;

/* Pastel accents brighten slightly in dark mode */
--pink: #f0a0b4;
--lavender: #bea0f0;
--mint: #80e0b8;
--peach: #f0b878;
--sky: #90c8f0;
--lemon: #f0e068;
```

### Pixel Border Conventions

```css
/* Standard card/panel */
border: 2px solid var(--border);
box-shadow: 4px 4px 0 var(--border);

/* Inset panel (recessed) */
border: 2px solid var(--border);
box-shadow: inset 2px 2px 0 var(--border-strong);

/* No border-radius anywhere — sharp pixel corners */
border-radius: 0;
```
