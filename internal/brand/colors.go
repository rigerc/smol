package brand

// Stops returns fresh Phoenix palette values sampled from the supplied artwork.
// The dark endpoints retain the requested F83600 and FCDF57 reference values.
func Stops(light bool) []string {
	if light {
		return []string{"#7E0D1D", "#B41B11", "#CC400C", "#C35F09", "#A85F00"}
	}
	return []string{"#F83600", "#FB611C", "#FC8A20", "#F9C523", "#FCDF57"}
}
