package httpapi

import "net/http"

// maxRequestBody is the upper bound for JSON request bodies on the
// transport/ownership/focus-state paths added by the live-transport sprint.
// Bounds memory alloc if the loopback binding assumption is ever relaxed.
// 1 MiB is ample for message bodies (tmux paste-buffer throughput limits
// effective usable size far below this).
const maxRequestBody = 1 << 20 // 1 MiB

// limitBody wraps r.Body in http.MaxBytesReader. Call at the top of any
// handler before the first Decode to cap allocator exposure.
func limitBody(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	}
}
