# The homa logo

The mark is a terminal prompt, `>_`, and a speech bubble: a chat that lives in
the terminal. Every file here is generated from one geometry; the wordmark and
tagline are JetBrains Mono (SIL OFL 1.1) converted to outlines, so nothing
depends on a font being installed.

| File | Size | What it is |
| --- | --- | --- |
| `homa.svg` / `.png` | 1024×1024 | primary: cream mark and wordmark, green bubble, tagline — for dark backgrounds |
| `homa-horisontal.svg` / `.png` | 1024×256 | mark and wordmark side by side |
| `homa-icon.svg` / `.png` | 512×512 | the mark alone |
| `homa-monochrome.svg` / `.png` | 1024×1024 | primary in one colour (cream) |
| `homa-green.svg` / `.png` | 1024×1024 | primary in one colour (green) — works on light and dark |
| `homa-favicon.svg` / `.png` | 32×32 | the `>_` alone, green |

All PNGs have a transparent background.

## Palette

| | Hex | Used for |
| --- | --- | --- |
| green | `#22E6A7` | the bubble, accents, the green variant |
| cream | `#F1F1E8` | the prompt and wordmark on dark |
| slate | `#2B2F36` | surfaces |
| ink | `#080F12` | the dark background |

## In the terminal

Plain text, for any terminal, logs, and `NO_COLOR`:

```
  \      .-------.
   >     |  ...  |
  / _    '-.-----'

         H O M A
   Peer-to-peer terminal chat
```

One line, for narrow places: `>_ [...] HOMA`

With colour: the same glyphs, `>_` in cream, the bubble and `H O M A` in green,
the tagline muted. Colour only ever reinforces the plain version; it never
replaces it.

With Unicode, `homa-icon.svg` reduces to half-block cells and stays legible
down to about 48 columns; below that the dots go, and below 30 the shape does.
The banner homa draws is that reduction with its horizontal edges moved onto
whole rows, because half cells meeting (`▀` over `▄`) show a seam in most
terminal fonts; the diagonals of the prompt keep theirs.
