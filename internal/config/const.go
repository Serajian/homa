package config

// configFile is where settings live inside the config directory.
const configFile = "config.json"

// MaxNickLen bounds the display name. It is announced to a peer, so it needs
// a limit; long enough for a real name, short enough to sit in a chat line.
const MaxNickLen = 32
