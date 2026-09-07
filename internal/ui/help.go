package ui

// showHelp prints the page a person reaches with h at the menu.
//
// It is one Printf rather than a line each, so a message arriving from a
// conversation cannot land in the middle of it: Printf takes the lock once
// and writes the whole block.
//
// The text is a local literal rather than a package constant. const.go holds
// the values that tune homa's behavior, and a page of prose is not one of
// them; it is content, and it lives with the function that shows it.
//
// It does not list the menu's keys. The menu is drawn underneath it with a
// label on every line, so repeating them would take room this page does not
// have and would rot the first time one changed. What is here is what the
// menu cannot say for itself.
//
// Every line is a claim about what homa does, and a claim that has gone stale
// is worse than no help at all. Whoever changes a command changes this too.
func (a *App) showHelp() {
	const page = `
  homa connects two people directly. There is no account and nothing in the
  middle: an address is all it takes, in either direction.

  to reach somebody, they give you their address and you add it with n. a
  shows yours for them to do the same. Treat it like a password: whoever
  has it can call you. b is the address book: renaming, forgetting, calling.

  q and Ctrl+C quit homa. Inside a conversation /quit leaves only the
  conversation, and /help lists what else you can do in there.

  when somebody calls, homa asks before putting them through, and hangs up
  on them if nobody answers within a minute.

  a name in [brackets] is the one you gave them. A ~ in front means they
  chose it themselves and are not in your contacts.
`

	a.ui.Printf("%s", page)
}
