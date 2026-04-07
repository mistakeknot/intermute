package httpapi

import "net/http"

// methodHandlers maps HTTP methods to handler functions for use with dispatchByMethod.
type methodHandlers struct {
	get    http.HandlerFunc
	post   http.HandlerFunc
	put    http.HandlerFunc
	patch  http.HandlerFunc
	delete http.HandlerFunc
}

// dispatchByMethod routes a request to the appropriate handler based on HTTP method.
// If no handler is registered for the request method, it responds with 405 Method Not Allowed.
func dispatchByMethod(w http.ResponseWriter, r *http.Request, h methodHandlers) {
	switch r.Method {
	case http.MethodGet:
		if h.get != nil {
			h.get(w, r)
			return
		}
	case http.MethodPost:
		if h.post != nil {
			h.post(w, r)
			return
		}
	case http.MethodPut:
		if h.put != nil {
			h.put(w, r)
			return
		}
	case http.MethodPatch:
		if h.patch != nil {
			h.patch(w, r)
			return
		}
	case http.MethodDelete:
		if h.delete != nil {
			h.delete(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}
