package paths

const (
	// DirPerm lets only the owner even list the directory.
	DirPerm = 0o700

	// FilePerm lets only the owner read a file. Everything homa stores is
	// either a secret or a private list of contacts.
	FilePerm = 0o600

	appDir = "homa"
)
