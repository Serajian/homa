// Package contacts is the address book: the local names a person gives to
// peers, and the addresses those names stand for. It exists so nobody has
// to paste a two-hundred-character address twice.
//
// A name is local. Two people may know the same peer by different names,
// and nothing is exchanged over the network about them.
package contacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Serajian/homa/internal/logx"
	"github.com/Serajian/homa/internal/paths"
)

var logger = logx.For("contacts")

// ErrNotFound means no contact goes by that name.
var ErrNotFound = errors.New("contacts: no such contact")

// ErrExists means a contact already goes by that name.
var ErrExists = errors.New("contacts: a contact by that name already exists")

// Contact is one peer a person can reach.
type Contact struct {
	// Name is the local nickname. It never leaves this machine.
	Name string `json:"name"`

	// Addr is the peer's homa address. It is a secret: it carries the
	// pre-shared key that guards their tunnel.
	Addr string `json:"addr"`

	// PubKey identifies the peer cryptographically, so an incoming
	// connection can be matched to a name rather than trusting whatever
	// name the caller announces. It is empty until we have talked to
	// them at least once.
	PubKey string `json:"pubkey,omitempty"`
}

// Book is the whole address book, kept sorted by name so menus are stable
// between runs.
type Book struct {
	list []Contact
}

// Path reports where the book is stored, for messages to the user.
func Path() string { return paths.Display(contactsFile) }

// Load reads the address book. An empty book is a normal state, not an
// error: a fresh install simply knows nobody yet.
func Load() (*Book, error) {
	p, err := paths.File(contactsFile)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		logger.Debug("no address book yet")
		return &Book{}, nil
	case err != nil:
		return nil, fmt.Errorf("contacts: reading %s: %w", p, err)
	}

	var list []Contact
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("contacts: %s is not valid JSON (%w); "+
			"fix it by hand, or delete it to start a fresh address book", p, err)
	}

	book := &Book{list: list}
	if err := book.validate(); err != nil {
		return nil, fmt.Errorf("contacts: %s holds a bad entry: %w", p, err)
	}
	book.sort()

	logger.Info("address book loaded", "count", len(book.list))
	return book, nil
}

// Save writes the address book to disk.
func (b *Book) Save() error {
	p, err := paths.File(contactsFile)
	if err != nil {
		return err
	}

	// Marshal a non-nil slice so an emptied book is written as [] rather
	// than null, which would not survive a round trip as cleanly.
	list := b.list
	if list == nil {
		list = []Contact{}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("contacts: encoding the address book: %w", err)
	}
	if err := paths.WriteAtomic(p, append(data, '\n')); err != nil {
		return err
	}

	logger.Info("address book saved", "count", len(list))
	return nil
}

// All returns the contacts in name order. The slice is a copy: changing it
// does not change the book.
func (b *Book) All() []Contact {
	return slices.Clone(b.list)
}

// Len reports how many contacts are known.
func (b *Book) Len() int { return len(b.list) }

// ByName finds a contact by nickname, ignoring letter case so a person does
// not have to remember how they capitalized it.
func (b *Book) ByName(name string) (Contact, error) {
	i := b.indexByName(name)
	if i < 0 {
		return Contact{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return b.list[i], nil
}

// ByPubKey finds the contact holding a peer's public key, so an incoming
// connection can be shown under the name this machine gave it. The key is
// what actually identifies a peer; the name a caller announces is not.
func (b *Book) ByPubKey(key string) (Contact, bool) {
	if key == "" {
		return Contact{}, false
	}
	for _, c := range b.list {
		if c.PubKey == key {
			return c, true
		}
	}
	return Contact{}, false
}

// Add records a new contact. It fails with ErrExists rather than silently
// replacing one, so a typo cannot overwrite a working address.
func (b *Book) Add(c Contact) error {
	c.Name = strings.TrimSpace(c.Name)
	c.Addr = strings.TrimSpace(c.Addr)

	if err := c.validate(); err != nil {
		return err
	}
	if b.indexByName(c.Name) >= 0 {
		return fmt.Errorf("%w: %q", ErrExists, c.Name)
	}

	b.list = append(b.list, c)
	b.sort()

	logger.Info("contact added", "name", c.Name)
	return nil
}

// Remove deletes a contact by name.
func (b *Book) Remove(name string) error {
	i := b.indexByName(name)
	if i < 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}

	b.list = slices.Delete(b.list, i, i+1)

	logger.Info("contact removed", "name", name)
	return nil
}

// SetPubKey records the public key learned from a conversation, so the next
// connection from that peer can be recognized. It reports whether anything
// changed, which tells the caller whether the book is worth saving.
func (b *Book) SetPubKey(name, key string) (bool, error) {
	i := b.indexByName(name)
	if i < 0 {
		return false, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if b.list[i].PubKey == key {
		return false, nil
	}

	b.list[i].PubKey = key

	logger.Info("contact key recorded", "name", name)
	return true, nil
}

func (b *Book) indexByName(name string) int {
	name = strings.TrimSpace(name)
	return slices.IndexFunc(b.list, func(c Contact) bool {
		return strings.EqualFold(c.Name, name)
	})
}

func (b *Book) sort() {
	slices.SortFunc(b.list, func(x, y Contact) int {
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
}
