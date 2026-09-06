package contacts

// contactsFile is where the address book lives inside the config directory.
const contactsFile = "contacts.json"

// MaxNameLen bounds a contact's nickname. It is only ever shown locally,
// but a menu entry has to fit on a line.
const MaxNameLen = 32
