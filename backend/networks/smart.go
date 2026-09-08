package networks

// Smart-schedule defaults: widely-cited best engagement windows per network
// (B2B + creator consensus, 2024-25). Each entry scores 1-3.
// Analytics boosts these: hours where YOUR past posts earned more
// engagement per post get +2 (see handlers/schedule.go).

import "solomon/backend/models"

type Slot struct {
	Weekday int `json:"weekday"` // 0=Sunday
	Hour    int `json:"hour"`    // 0-23 local
	Score   int `json:"score"`
}

// DefaultSlots returns fallback best-times when there is no analytics yet.
func DefaultSlots(n models.Network) []Slot {
	mk := func(days []int, hours []int, score int) []Slot {
		var out []Slot
		for _, d := range days {
			for _, h := range hours {
				out = append(out, Slot{Weekday: d, Hour: h, Score: score})
			}
		}
		return out
	}
	weekdays := []int{1, 2, 3, 4, 5}
	switch n {
	case models.NetworkLinkedIn:
		// B2B: Tue-Thu mornings win
		return append(mk([]int{2, 3, 4}, []int{9, 10}, 3), mk(weekdays, []int{8, 12}, 2)...)
	case models.NetworkTikTok:
		// midday + evenings, Tue/Thu/Sat peak
		return append(mk([]int{2, 4, 6}, []int{12, 19, 20}, 3), mk(weekdays, []int{15, 21}, 2)...)
	case models.NetworkInstagram:
		// lunch scroll + evening
		return append(mk(weekdays, []int{11, 12, 19}, 3), mk([]int{0, 6}, []int{10, 18}, 2)...)
	case models.NetworkYouTube:
		// afternoons + weekend mornings (watch time)
		return append(mk([]int{0, 6}, []int{9, 10, 15}, 3), mk(weekdays, []int{14, 15, 16}, 2)...)
	case models.NetworkFacebook:
		return append(mk(weekdays, []int{9, 13}, 3), mk([]int{0, 6}, []int{12}, 2)...)
	case models.NetworkPinterest:
		// evenings + weekend planning
		return append(mk([]int{0, 6}, []int{20, 21}, 3), mk(weekdays, []int{20}, 2)...)
	default: // twitter/X: weekday mornings + lunch
		return append(mk(weekdays, []int{9, 12}, 3), mk([]int{0, 6}, []int{11}, 2)...)
	}
}
