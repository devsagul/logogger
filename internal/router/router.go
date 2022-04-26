package router

import (
	"log"
	"net/http"
	"regexp"
)

type route struct {
	path    *regexp.Regexp
	handler func(http.ResponseWriter, *http.Request, []string)
}

type Router struct {
	routes []route
}

func (r *Router) RegisterHandler(rt string, handler func(http.ResponseWriter, *http.Request, []string)) {
	re := regexp.MustCompile(rt)
	r.routes = append(r.routes, route{re, handler})
}

func (r *Router) Handle(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	for _, rt := range r.routes {
		re := rt.path
		match := re.FindStringSubmatch(req.URL.Path)
		if match != nil {
			rt.handler(w, req, match[1:])
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	body := "Status: ERROR\nNot Found"
	log.Printf("URL: %s, response: %s", req.URL.Path, body)
	_, _ = w.Write([]byte(body))
}
