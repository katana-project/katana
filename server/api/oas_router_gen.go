// Code generated by ogen, DO NOT EDIT.

package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/ogen-go/ogen/uri"
)

func (s *Server) cutPrefix(path string) (string, bool) {
	prefix := s.cfg.Prefix
	if prefix == "" {
		return path, true
	}
	if !strings.HasPrefix(path, prefix) {
		// Prefix doesn't match.
		return "", false
	}
	// Cut prefix from the path.
	return strings.TrimPrefix(path, prefix), true
}

// ServeHTTP serves http request as defined by OpenAPI v3 specification,
// calling handler that matches the path or returning not found error.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	elem := r.URL.Path
	elemIsEscaped := false
	if rawPath := r.URL.RawPath; rawPath != "" {
		if normalized, ok := uri.NormalizeEscapedPath(rawPath); ok {
			elem = normalized
			elemIsEscaped = strings.ContainsRune(elem, '%')
		}
	}

	elem, ok := s.cutPrefix(elem)
	if !ok || len(elem) == 0 {
		s.notFound(w, r)
		return
	}
	args := [3]string{}

	// Static code generated router with unwrapped path search.
	switch {
	default:
		if len(elem) == 0 {
			break
		}
		switch elem[0] {
		case '/': // Prefix: "/repos"
			if l := len("/repos"); len(elem) >= l && elem[0:l] == "/repos" {
				elem = elem[l:]
			} else {
				break
			}

			if len(elem) == 0 {
				switch r.Method {
				case "GET":
					s.handleGetReposRequest([0]string{}, elemIsEscaped, w, r)
				default:
					s.notAllowed(w, r, "GET")
				}

				return
			}
			switch elem[0] {
			case '/': // Prefix: "/"
				if l := len("/"); len(elem) >= l && elem[0:l] == "/" {
					elem = elem[l:]
				} else {
					break
				}

				// Param: "id"
				// Match until "/"
				idx := strings.IndexByte(elem, '/')
				if idx < 0 {
					idx = len(elem)
				}
				args[0] = elem[:idx]
				elem = elem[idx:]

				if len(elem) == 0 {
					switch r.Method {
					case "GET":
						s.handleGetRepoByIdRequest([1]string{
							args[0],
						}, elemIsEscaped, w, r)
					default:
						s.notAllowed(w, r, "GET")
					}

					return
				}
				switch elem[0] {
				case '/': // Prefix: "/media"
					if l := len("/media"); len(elem) >= l && elem[0:l] == "/media" {
						elem = elem[l:]
					} else {
						break
					}

					if len(elem) == 0 {
						switch r.Method {
						case "GET":
							s.handleGetRepoMediaRequest([1]string{
								args[0],
							}, elemIsEscaped, w, r)
						default:
							s.notAllowed(w, r, "GET")
						}

						return
					}
					switch elem[0] {
					case '/': // Prefix: "/"
						if l := len("/"); len(elem) >= l && elem[0:l] == "/" {
							elem = elem[l:]
						} else {
							break
						}

						// Param: "mediaId"
						// Match until "/"
						idx := strings.IndexByte(elem, '/')
						if idx < 0 {
							idx = len(elem)
						}
						args[1] = elem[:idx]
						elem = elem[idx:]

						if len(elem) == 0 {
							switch r.Method {
							case "GET":
								s.handleGetRepoMediaByIdRequest([2]string{
									args[0],
									args[1],
								}, elemIsEscaped, w, r)
							default:
								s.notAllowed(w, r, "GET")
							}

							return
						}
						switch elem[0] {
						case '/': // Prefix: "/stream/"
							if l := len("/stream/"); len(elem) >= l && elem[0:l] == "/stream/" {
								elem = elem[l:]
							} else {
								break
							}

							// Param: "format"
							// Leaf parameter
							args[2] = elem
							elem = ""

							if len(elem) == 0 {
								// Leaf node.
								switch r.Method {
								case "GET":
									s.handleGetRepoMediaStreamRequest([3]string{
										args[0],
										args[1],
										args[2],
									}, elemIsEscaped, w, r)
								default:
									s.notAllowed(w, r, "GET")
								}

								return
							}
						}
					}
				}
			}
		}
	}
	s.notFound(w, r)
}

// Route is route object.
type Route struct {
	name        string
	summary     string
	operationID string
	pathPattern string
	count       int
	args        [3]string
}

// Name returns ogen operation name.
//
// It is guaranteed to be unique and not empty.
func (r Route) Name() string {
	return r.name
}

// Summary returns OpenAPI summary.
func (r Route) Summary() string {
	return r.summary
}

// OperationID returns OpenAPI operationId.
func (r Route) OperationID() string {
	return r.operationID
}

// PathPattern returns OpenAPI path.
func (r Route) PathPattern() string {
	return r.pathPattern
}

// Args returns parsed arguments.
func (r Route) Args() []string {
	return r.args[:r.count]
}

// FindRoute finds Route for given method and path.
//
// Note: this method does not unescape path or handle reserved characters in path properly. Use FindPath instead.
func (s *Server) FindRoute(method, path string) (Route, bool) {
	return s.FindPath(method, &url.URL{Path: path})
}

// FindPath finds Route for given method and URL.
func (s *Server) FindPath(method string, u *url.URL) (r Route, _ bool) {
	var (
		elem = u.Path
		args = r.args
	)
	if rawPath := u.RawPath; rawPath != "" {
		if normalized, ok := uri.NormalizeEscapedPath(rawPath); ok {
			elem = normalized
		}
		defer func() {
			for i, arg := range r.args[:r.count] {
				if unescaped, err := url.PathUnescape(arg); err == nil {
					r.args[i] = unescaped
				}
			}
		}()
	}

	elem, ok := s.cutPrefix(elem)
	if !ok {
		return r, false
	}

	// Static code generated router with unwrapped path search.
	switch {
	default:
		if len(elem) == 0 {
			break
		}
		switch elem[0] {
		case '/': // Prefix: "/repos"
			if l := len("/repos"); len(elem) >= l && elem[0:l] == "/repos" {
				elem = elem[l:]
			} else {
				break
			}

			if len(elem) == 0 {
				switch method {
				case "GET":
					r.name = "GetRepos"
					r.summary = "Lists repositories."
					r.operationID = "getRepos"
					r.pathPattern = "/repos"
					r.args = args
					r.count = 0
					return r, true
				default:
					return
				}
			}
			switch elem[0] {
			case '/': // Prefix: "/"
				if l := len("/"); len(elem) >= l && elem[0:l] == "/" {
					elem = elem[l:]
				} else {
					break
				}

				// Param: "id"
				// Match until "/"
				idx := strings.IndexByte(elem, '/')
				if idx < 0 {
					idx = len(elem)
				}
				args[0] = elem[:idx]
				elem = elem[idx:]

				if len(elem) == 0 {
					switch method {
					case "GET":
						r.name = "GetRepoById"
						r.summary = "Gets a repository."
						r.operationID = "getRepoById"
						r.pathPattern = "/repos/{id}"
						r.args = args
						r.count = 1
						return r, true
					default:
						return
					}
				}
				switch elem[0] {
				case '/': // Prefix: "/media"
					if l := len("/media"); len(elem) >= l && elem[0:l] == "/media" {
						elem = elem[l:]
					} else {
						break
					}

					if len(elem) == 0 {
						switch method {
						case "GET":
							r.name = "GetRepoMedia"
							r.summary = "Lists a repository's media."
							r.operationID = "getRepoMedia"
							r.pathPattern = "/repos/{id}/media"
							r.args = args
							r.count = 1
							return r, true
						default:
							return
						}
					}
					switch elem[0] {
					case '/': // Prefix: "/"
						if l := len("/"); len(elem) >= l && elem[0:l] == "/" {
							elem = elem[l:]
						} else {
							break
						}

						// Param: "mediaId"
						// Match until "/"
						idx := strings.IndexByte(elem, '/')
						if idx < 0 {
							idx = len(elem)
						}
						args[1] = elem[:idx]
						elem = elem[idx:]

						if len(elem) == 0 {
							switch method {
							case "GET":
								r.name = "GetRepoMediaById"
								r.summary = "Gets a repository's media."
								r.operationID = "getRepoMediaById"
								r.pathPattern = "/repos/{repoId}/media/{mediaId}"
								r.args = args
								r.count = 2
								return r, true
							default:
								return
							}
						}
						switch elem[0] {
						case '/': // Prefix: "/stream/"
							if l := len("/stream/"); len(elem) >= l && elem[0:l] == "/stream/" {
								elem = elem[l:]
							} else {
								break
							}

							// Param: "format"
							// Leaf parameter
							args[2] = elem
							elem = ""

							if len(elem) == 0 {
								switch method {
								case "GET":
									// Leaf: GetRepoMediaStream
									r.name = "GetRepoMediaStream"
									r.summary = "Gets a HTTP media stream."
									r.operationID = "getRepoMediaStream"
									r.pathPattern = "/repos/{repoId}/media/{mediaId}/stream/{format}"
									r.args = args
									r.count = 3
									return r, true
								default:
									return
								}
							}
						}
					}
				}
			}
		}
	}
	return r, false
}
