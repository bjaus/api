package api

import "net/http"

// RedirectResp is a declarative response that issues an HTTP redirect.
// It sets the Location header and status in the standard way, and has no body.
//
// Use the Redirect helper for the common case:
//
//	return api.Redirect("/dashboard", http.StatusSeeOther), nil
//
// For redirects that also need to set cookies or extra headers, declare your
// own response type with tagged fields for each concern.
type RedirectResp struct {
	Status   int    `status:""`
	Location string `header:"Location"`
}

// Redirect returns a RedirectResp for the given URL and status code. Pass
// status = 0 to use the default of http.StatusFound (302).
func Redirect(url string, status int) *RedirectResp {
	if status == 0 {
		status = http.StatusFound
	}
	return &RedirectResp{Status: status, Location: url}
}
