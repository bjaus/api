package api

import "strings"

// paramTags are the struct tags used for binding request parameters.
var paramTags = []string{"path", "query", "header", "cookie"}

// tagOptions splits a struct tag value on comma and returns
// the name and remaining options.
func tagOptions(tag string) (string, string) {
	name, opts, _ := strings.Cut(tag, ",")
	return name, opts
}

// tagContains reports whether a comma-separated list of options
// contains a particular option.
func tagContains(opts string, name string) bool {
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == name {
			return true
		}
	}
	return false
}
