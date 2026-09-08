package main

import "flag"

// version is stamped at build time. See the ldflags note in the Makefile.
var version = "dev"

// options is what the command line asked for.
type options struct {
	debug       bool
	logFile     string
	showVersion bool
	noColor     bool
}

func parseFlags() options {
	var opts options

	flag.BoolVar(&opts.debug, "debug", false,
		"write diagnostics to stderr (they will interleave with the chat)")
	flag.StringVar(&opts.logFile, "log", "",
		"write diagnostics to this file instead of stderr")
	flag.BoolVar(&opts.showVersion, "version", false,
		"print the version and exit")
	flag.BoolVar(&opts.noColor, "no-color", false,
		"never write color, even to a terminal (setting NO_COLOR does the same)")

	flag.Parse()
	return opts
}
