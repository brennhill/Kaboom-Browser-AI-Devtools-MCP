// Purpose: Merges all interact property groups into the complete interact tool property set.
// Why: Provides the top-level assembly point for interact schema properties.
package interact

func toolProperties() map[string]any {
	props := make(map[string]any)
	mergeProps(props, dispatchProperties())
	mergeProps(props, targetingProperties())
	mergeProps(props, coreActionProperties())
	mergeProps(props, gestureProperties())
	mergeProps(props, findProperties())
	mergeProps(props, environmentPinProperties())
	mergeProps(props, formAndWaitProperties())
	mergeProps(props, outputAndBatchProperties())
	return props
}

func mergeProps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
