# The full-screen interface

Version 2, first item. Decided 2026-09-08 after the questions in
[decisions.md](../decisions.md) were answered; this is the design the
implementation plan is written from.

## What it is for

Version 1's interface is a stream of lines the terminal owns until Enter. That
gives it two faults nothing above the line can fix: a message arriving while
you type lands over your half-typed line, and a line not yet sent when the
peer leaves is dropped. It also makes arrow keys, history, tab completion and
scrollback impossible. The full-screen interface owns the terminal instead:
input is a component, the screen is drawn whole, and both faults disappear
as a consequence rather than as a fix.

## Decisions

1. **bubbletea v2, bubbles v2, lipgloss v2** (`github.com/charmbracelet/*/v2`).
   The `golang.org/x/term` route the roadmap measured fixes the input line
   only; it gives no scrolling pane, no resize, no layout, and no place for
   commands offered as they are typed. v2 rather than v1 because both are
   stable and nothing here is written yet.
2. **One conversation at a time**, as today. A contact list beside the chat
   would be decoration: there is nothing to switch to. Several conversations
   at once would change `session` and the accept path — a leak below `ui`,
   and a different version.
3. **Output that is not a terminal is an error.** `homa: needs a terminal`,
   exit 1. A full-screen program cannot draw into a pipe, and nobody chats
   through one. Version 1 stays downloadable as v0.1.0; there is no line-mode
   fallback, because two interfaces means every later feature twice.
4. **No alternate screen.** When homa exits, what was on the screen stays in
   the terminal's scrollback, the way version 1 left it. `vim`-style wiping
   would take the conversation with it.
5. **Only `internal/ui` changes**, plus three lines in `cmd/homa` and the
   modules in `go.mod`. Everything below `ui` is used through the interfaces
   it already exposes (`session.Handler`, `peer.Listener`, `session.Run`, the
   book, the settings). If the work finds it needs something new from below,
   that is a leak and is fixed first, on its own.

What carries over unchanged, because it lives in
[decisions.md](../decisions.md) rather than in code: one meaning per color
(cream/bold is you, green is them, grey is homa, yellow warns), color never the
only signal, network text wrapped in a style and never formatted into one,
menus grouped by space with the people first, every wording and every hint.

## Screens

One frame everywhere: a status line at the top, the body, a key line at the
bottom in grey. Widths adapt on resize; below ~50 columns the frame drops its
decorations and keeps the text.

**Menu.** The body is today's grouped menu with a cursor on the contacts.
Every key still does what it does (`1`, `n`, `q` …); Enter on the highlighted
contact calls them, ↑↓ move.

```
 homa  ·  you are alice  ·  tcpGFwWCDeaV…  ·  listening
                                                          
   ▸ alice                                                
     ~bob                                                 
                                                          
     n  add a contact        s  settings                  
     b  contacts             c  clear                     
     a  my address           h  help                      
                                                          
     r  start over           q  quit                      
                                                          
 ↑↓ choose · Enter call · or press a key                  
```

**Incoming call.** Not a screen: a bar under the body, on whatever screen is
up except a conversation, with the countdown redrawn each second. `y` takes
it, `n` or Enter does not, and running out is what it is today.

```
 ~bob is calling  ·  47s        y take it · n not now     
```

**Conversation.** Three regions: the status line (who, what they call
themselves, where files go), a scrolling pane of messages, and the input line
with the `[me]` label. File events and hints appear in the pane in homa's
grey voice, as today. PgUp/PgDn and the mouse wheel scroll; ↑↓ walk the input
history; a line being typed when the peer leaves stays in the input, and the
pane says they left. The one rule in the interface is the dashed line
between pane and input, kept faint.

```
 talking to alice  ·  they call themselves "alice"  ·  files → ~/homa-files
 [alice] salam! file ro gerefti?                           
 [me] na, befrest                                          
   alice offers notes.md (14 KB)  ·  y accept · n reject   
   receiving notes.md  ·  70%                              
 ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
 [me] man dar▌                                             
 PgUp/PgDn scroll · /help · /quit                          
```

**Calling.** The menu stays; the bar shows `calling alice…` then `waiting for
alice to answer  ·  55s  ·  Enter to give up`, and either becomes the
conversation or a grey line saying why not.

**Contacts, one contact, add a contact, settings, first run, start over,
help.** Today's content in the same frame: lists with a cursor, forms as
`textinput` fields walked with Enter, yes/no questions as `[y/N]`, start over
still demanding the typed word `reset`. The help page is the same text in
the body.

## Model

One `tea.Model` with a `screen` value — `menu`, `conversation`, `contacts`,
`contact`, `addContact`, `settings`, `setup`, `reset`, `help` — and a
sub-model per screen holding only what that screen needs. Cross-cutting state
sits on the root: the incoming call (with its deadline), the outgoing call,
the width and height, the style.

Every `Update` runs on one goroutine. **No mutex remains in `ui`**; the
locks version 1 needed around the prompt and the shared handler state go
with the prompt.

### Messages

Everything from below arrives as a `tea.Msg` through `program.Send`, from the
same places that print today:

| Message | Sent by |
| --- | --- |
| `callArrived{call}` | the accept loop, now a `tea.Cmd` that runs for the life of the program |
| `callAnswered`, `callRefused{reason}`, `callFailed{err}` | the dial path |
| `peerSaid{text}` | `Handler.OnText` |
| `peerLeft{err}` | the goroutine that runs the session |
| `fileOffered{name, size, reply chan bool}` | `Handler.OnFileOffer` |
| `fileProgress{name, pct}`, `fileDone{name, path}`, `fileFailed{name, err}` | the handler |
| `tick` | `tea.Tick`, once a second while a countdown is showing |

The handler is an adapter that only calls `Send`. `OnFileOffer` keeps
blocking the session's read goroutine on a channel, exactly as today; `Update`
fills the channel when `y` or `n` is pressed. Nothing below `ui` learns that
the interface changed.

### Layout and style

`View` composes the frame with lipgloss: status line, body sized to the
window, key line. A `styles` value holds every color and border in one place
— the theme is data, and changing the look is changing that value, not the
logic. lipgloss detects the color depth and honors `NO_COLOR`; `-no-color`
forces it off, as today. Unicode borders and the banner need a UTF-8 locale;
otherwise ASCII, the rule version 1 already has.

## What goes, what stays

Goes: `ui.go` (the pump, `Prompt`/`ErasePrompt`/`EndPrompt`, `clearLine`),
`prompt.go`, `countdown.go`, `style.go`, `theme.go`, the drawing half of
`welcome.go`. These are the machinery of owning a line, and the line is gone.

Stays, moved into the new shape: the behavior in `menu.go`, `contacts.go`,
`chat.go`, `files.go`, `handler.go`, `setup.go` — dialing, greeting and
parking a call, `describe`, `rememberKey`, `resolveSend`, the listing, the
settings walk, the reset — and every wording. The banner's rows stay; only
what draws them changes.

`cmd/homa`: `ui.New`, `ui.NewApp`, `ui.Setup` become one `ui.Run(ctx, deps)`
that builds the model and runs the program; the terminal check happens
there. `go.mod`: the three Charm modules and their dependencies.

## Testing

- `Update` and `View` are pure: every screen is rendered from a hand-built
  model and compared as text. The colored output stripped of escapes equals
  the plain output, as today's test holds.
- Flows through `teatest`: a call arrives → `y` → conversation → a message
  each way → an offer → `y` → `/quit` → the menu.
- `test/live` keeps driving two real processes through a relay, but reads a
  *screen* rather than lines: a small VT interpreter in the test applies
  cursor movement and erasing to a grid, and the `await`s look at the grid.
  The 37 expectations are rewritten against it.
- The README's transcripts are recaptured from the finished interface.

## Order of work

Each step is one change with its own report, and homa runs at the end of
every step.

1. The skeleton: model, frame, menu, quit; `cmd/homa` switched over; the
   terminal check. Old screens unreachable but still compiled.
2. Calls and the conversation: accept loop as a command, incoming bar, dial,
   the pane and the input. The two faults are gone here.
3. Files: offers, progress, `/files`, `/send`.
4. The remaining screens: contacts, contact, add, settings, setup, reset, help.
5. Delete what is dead in `ui`; `test/live` on the screen grid.
6. README recaptured; `status.md` loses the two warts; `architecture.md`.

## Out of scope, on purpose

Commands offered as they are typed (version 2, next item — the input
component is where it will plug in), the bell, `/store`, several
conversations at once, a hand-picked light or dark theme, mouse beyond
scrolling.
