package gui

type (
	history struct {
		Entries []string `json:"entries"`
		idx     int
	}
)

func (h *history) dedup() {
	set := map[string]int{}
	for i, v := range h.Entries {
		set[v] = i
	}
	var final []string
	for i, v := range h.Entries {
		last := set[v]
		if last == i {
			final = append(final, v)
		}
	}
	h.Entries = final
	h.idx = len(h.Entries)
}

func (h *history) add(code string) {
	h.Entries = append(h.Entries, code)
	h.idx = len(h.Entries)
}

func (h *history) back() string {
	h.idx--
	return h.peek()
}

func (h *history) forward() string {
	h.idx++
	return h.peek()
}

func (h *history) peek() string {
	if len(h.Entries) == 0 {
		return ""
	}
	if h.idx < 0 {
		h.idx = 0
	}
	if h.idx >= len(h.Entries) {
		h.idx = len(h.Entries) - 1
	}
	return h.Entries[h.idx]
}
