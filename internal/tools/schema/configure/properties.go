// Purpose: Merges core and runtime properties into the complete configure tool property set.
// Why: Provides the top-level assembly point for configure schema properties.
package configure

func toolProperties() map[string]any {
	props := make(map[string]any)
	mergeProps(props, coreProperties())
	mergeProps(props, runtimeProperties())
	mergeProps(props, fixtureProperties())
	return props
}

func mergeProps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
