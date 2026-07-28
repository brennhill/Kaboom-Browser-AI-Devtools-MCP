/**
 * Purpose: Pure element-result collection, filtering, limiting, and page metadata shared by commands.
 */
export function selectCommandElements(elements, params) {
    const visible = params.visible_only === true
        ? elements.filter((element) => element.visible !== false)
        : elements;
    const limit = typeof params.limit === 'number' && params.limit > 0 ? params.limit : visible.length;
    return visible.slice(0, limit);
}
export function collectCommandElements(results, limit) {
    const elements = [];
    let firstError;
    for (const item of results) {
        const result = item.result;
        if (result?.success === false) {
            firstError ||= result.error || result.message;
            continue;
        }
        if (result?.elements) {
            elements.push(...result.elements);
            if (elements.length >= limit)
                break;
        }
    }
    return { elements: elements.slice(0, limit), ...(firstError ? { firstError } : {}) };
}
export function commandPageMetadata(tab) {
    return {
        url: tab.url || '',
        title: tab.title || '',
        tab_status: tab.status || '',
        favicon: tab.favIconUrl || '',
        viewport: { width: tab.width, height: tab.height }
    };
}
//# sourceMappingURL=element-results.js.map