package proxy

import "regexp"

type proxyItem struct {
	regexp    *regexp.Regexp
	proxyHost string
}
